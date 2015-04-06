package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	linuxproc "github.com/c9s/goprocinfo/linux"
)

type byRSS []linuxproc.Process

func (p byRSS) Len() int {
	return len(p)
}
func (p byRSS) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p byRSS) Less(i, j int) bool {
	return p[i].Status.VmRSS < p[j].Status.VmRSS
}

type simpleProcs struct {
	Cmdline  string
	CPUUsage float64
}

type byCPU []simpleProcs

func (p byCPU) Len() int {
	return len(p)
}
func (p byCPU) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p byCPU) Less(i, j int) bool {
	return p[i].CPUUsage < p[j].CPUUsage
}

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

func getCPUTotal(dockerID string) uint64 {
	cpuAcctPath := fmt.Sprintf("/sys/fs/cgroup/cpuacct/docker/%s/cpuacct.stat", dockerID)
	dataBytes, _ := ioutil.ReadFile(cpuAcctPath)
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

	return total
}

func getProc(pid uint64, procs []linuxproc.Process) *linuxproc.Process {
	for _, p := range procs {
		if p.Status.Pid == pid {
			return &p
		}
	}
	return nil
}

func getTopCPU(dockerID string, limit int) ([]simpleProcs, error) {
	procs1, err := getProcesses(dockerID)
	if err != nil {
		return nil, err
	}
	cpu1 := getCPUTotal(dockerID)
	time.Sleep(time.Second)
	procs2, err := getProcesses(dockerID)
	if err != nil {
		return nil, err
	}
	cpu2 := getCPUTotal(dockerID)
	var procs []simpleProcs
	for _, p2 := range procs2 {
		p1 := getProc(p2.Status.Pid, procs1)
		if p1 != nil {
			user := int64(p2.Stat.Utime-p1.Stat.Utime) + (p2.Stat.Cutime - p1.Stat.Cutime)
			system := int64(p2.Stat.Stime-p1.Stat.Stime) + (p2.Stat.Cstime - p1.Stat.Cstime)
			percent := (float64(user+system) / float64(cpu2-cpu1)) * 100
			procs = append(procs, simpleProcs{p2.Cmdline, percent})
		}

	}
	sort.Sort(sort.Reverse(byCPU(procs)))
	var result []simpleProcs
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%f%% CPU %s\n", p.CPUUsage, p.Cmdline)
	}
	return result, nil
}

func getTopMem(dockerID string, limit int) ([]linuxproc.Process, error) {
	procs, err := getProcesses(dockerID)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(byRSS(procs)))
	var result []linuxproc.Process
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%dKb %s\n", p.Status.VmRSS, p.Cmdline)
	}
	return result, nil
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

func main() {
	http.HandleFunc("/mem", memHandler)
	http.HandleFunc("/cpu", cpuHandler)
	http.ListenAndServe(":9000", nil)
}
