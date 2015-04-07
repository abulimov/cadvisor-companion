package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	linuxproc "github.com/c9s/goprocinfo/linux"
)

type customProcs struct {
	linuxproc.Process
	CPUUsage float64
}

type byCPU []customProcs
type byRSS []customProcs

func (p byRSS) Len() int {
	return len(p)
}
func (p byRSS) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p byRSS) Less(i, j int) bool {
	return p[i].Status.VmRSS < p[j].Status.VmRSS
}

func (p byCPU) Len() int {
	return len(p)
}
func (p byCPU) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p byCPU) Less(i, j int) bool {
	return p[i].CPUUsage < p[j].CPUUsage
}

type dockerSnapshot struct {
	processes []linuxproc.Process
	cpuStat   uint64
}
type historyEntry map[string]dockerSnapshot

var channel chan historyEntry

var history [60]historyEntry

func getProcesses(dockerID string) ([]linuxproc.Process, error) {
	tasksPath := fmt.Sprintf("/sys/fs/cgroup/cpu/docker/%s/tasks", dockerID)
	dataBytes, err := ioutil.ReadFile(tasksPath)
	if err != nil {
		return nil, err
	}

	tasksString := string(dataBytes)
	tasks := strings.Split(tasksString, "\n")
	var procs []linuxproc.Process
	for _, t := range tasks {
		pid, err := strconv.ParseUint(t, 10, 64)
		if err == nil {
			p, err := linuxproc.ReadProcess(pid, "/proc/")
			if err != nil {
				continue
			}
			if p.Cmdline != "" && p.Status.VmRSS > 0 {
				procs = append(procs, *p)
			}
		}
	}
	return procs, nil
}

func getCPUTotalUsage(dockerID string) (uint64, error) {
	cpuAcctPath := fmt.Sprintf("/sys/fs/cgroup/cpuacct/docker/%s/cpuacct.stat", dockerID)
	dataBytes, err := ioutil.ReadFile(cpuAcctPath)
	if err != nil {
		return 0, nil
	}
	cpuAcctString := string(dataBytes)
	cpuAcct := strings.Split(cpuAcctString, "\n")
	var total uint64
	total = 0
	for _, s := range cpuAcct {
		splitted := strings.Split(s, " ")
		if len(splitted) > 1 {
			usage, _ := strconv.ParseUint(splitted[1], 10, 64)
			total += usage
		}
	}

	return total, nil
}

func findProc(pid uint64, procs []linuxproc.Process) *linuxproc.Process {
	for _, p := range procs {
		if p.Status.Pid == pid {
			return &p
		}
	}
	return nil
}

func getLastData(dockerID string) ([]customProcs, error) {
	last := len(history) - 1
	first := last
	for i, e := range history {
		if len(e[dockerID].processes) > 0 {
			first = i
			break
		}
	}
	entry1 := history[first][dockerID]
	entry2 := history[last][dockerID]
	var procs []customProcs
	for _, p2 := range entry2.processes {
		p1 := findProc(p2.Status.Pid, entry1.processes)
		if p1 != nil {
			user := int64(p2.Stat.Utime-p1.Stat.Utime) + (p2.Stat.Cutime - p1.Stat.Cutime)
			system := int64(p2.Stat.Stime-p1.Stat.Stime) + (p2.Stat.Cstime - p1.Stat.Cstime)
			percent := (float64(user+system) / float64(entry2.cpuStat-entry1.cpuStat)) * 100
			procs = append(procs, customProcs{p2, percent})
		}

	}
	return procs, nil
}

func getTopCPU(dockerID string, limit int) ([]customProcs, error) {
	procs, err := getLastData(dockerID)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(byCPU(procs)))
	var result []customProcs
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%f%% CPU %s\n", p.CPUUsage, p.Cmdline)
	}
	return result, nil
}

func getTopMem(dockerID string, limit int) ([]customProcs, error) {
	procs, err := getLastData(dockerID)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(byRSS(procs)))
	var result []customProcs
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%dKb %s\n", p.Status.VmRSS, p.Cmdline)
	}
	return result, nil
}

func getDockerIDs() ([]string, error) {
	d, err := os.Open("/sys/fs/cgroup/cpu/docker")
	if err != nil {
		return nil, err
	}
	defer d.Close()

	results := make([]string, 0, 50)
	for {
		fis, err := d.Readdir(10)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		for _, fi := range fis {
			// We only care about directories, since all pids are dirs
			if !fi.IsDir() {
				continue
			}

			// We only care if the name starts with a numeric
			name := fi.Name()
			if len(name) > '8' {
				results = append(results, name)
			}
		}
	}

	return results, nil
}

func memHandler(res http.ResponseWriter, req *http.Request) {
	dockerID := "f33b34a760f631a7176f10d9babab89c20dd0ebde744ed83b1ea27f21ce0bb75"
	result, err := getTopMem(dockerID, 5)
	if err != nil {
		fmt.Println(err)
	}
	jsonResult, _ := json.Marshal(result)
	res.Header().Set(
		"Content-Type",
		"text/json",
	)
	io.WriteString(res, string(jsonResult))
}

func cpuHandler(res http.ResponseWriter, req *http.Request) {
	dockerID := "f33b34a760f631a7176f10d9babab89c20dd0ebde744ed83b1ea27f21ce0bb75"
	result, err := getTopCPU(dockerID, 5)
	if err != nil {
		fmt.Println(err)
	}
	jsonResult, _ := json.Marshal(result)
	res.Header().Set(
		"Content-Type",
		"text/json",
	)
	io.WriteString(res, string(jsonResult))
}

func collectData() {
	dockerIDs, err := getDockerIDs()
	if err != nil {
		fmt.Println(err)
	}
	for {
		entry := make(historyEntry)
		for _, id := range dockerIDs {
			res, _ := getProcesses(id)
			cpu, _ := getCPUTotalUsage(id)
			entry[id] = dockerSnapshot{res, cpu}
		}
		channel <- entry
		time.Sleep(time.Second)
	}
}
func getData() {
	for {
		entry := <-channel
		for i, v := range history[1:] {
			history[i] = v
		}
		history[59] = entry
	}
}

func main() {
	channel = make(chan historyEntry)
	go collectData()
	go getData()
	http.HandleFunc("/mem", memHandler)
	http.HandleFunc("/cpu", cpuHandler)
	http.ListenAndServe(":8801", nil)
}
