package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	linuxproc "github.com/c9s/goprocinfo/linux"
)

var version = "0.0.1"

// set up cli vars
var argIP = flag.String("listen_ip", "", "IP to listen on, defaults to all IPs")
var argPort = flag.Int("port", 8801, "port to listen")
var versionFlag = flag.Bool("version", false, "print cAdvisor-companion version and exit")

// customProcs is our extended linuxproc.Process type
// with custom cpuUsage attribute
type customProcs struct {
	linuxproc.Process
	CPUUsage float64
}

// byCPU helps us sort array of customProcs by CPUUsage
type byCPU []customProcs

// byRSS helps us sort array of customProcs by Status.VmRSS
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

// dockerSnapshot containes process list and cpu usage for some docker container
type dockerSnapshot struct {
	processes []linuxproc.Process
	cpuStat   uint64
}

// historyEntry containes all dockerSnapshots for some moment in time
type historyEntry map[string]dockerSnapshot

// history holds RRD-like array of last 60 history entries
var history [60]historyEntry

// readCgroup reads and parses /proc/{pid}/cgroup file
func readCgroup(pid uint64, procPath string) (string, error) {
	cgroupPath := fmt.Sprintf("%s/%d/cgroup", procPath, pid)
	dataBytes, err := ioutil.ReadFile(cgroupPath)
	if err != nil {
		return "", err
	}
	cgroupString := string(dataBytes)
	lines := strings.Split(cgroupString, "\n")
	var validLine = regexp.MustCompile("^[0-9]+:([a-z,]+):([a-z0-9/]+)$")
	for _, l := range lines {
		m := validLine.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		// we care only about cpu cgroup
		if strings.Contains(m[1], "cpu") {
			return m[2], nil
		}
	}
	return "", nil
}

// getProcesses returns list of processes for given dockerID
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
			cgroup, err := readCgroup(pid, "/proc/")
			fmt.Println(cgroup)
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

// getCPUTotalUsage returns amount of CPU time, used by given docker container
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

// findProc searches for given pid in list of processes and returns
// its process if found
func findProc(pid uint64, procs []linuxproc.Process) *linuxproc.Process {
	for _, p := range procs {
		if p.Status.Pid == pid {
			return &p
		}
	}
	return nil
}

// getLastData returns latest data from history with added calculated CPU usage
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

// getTopCPU returns `limit` entries with top CPU usage
func getTopCPU(dockerID string, limit int) ([]customProcs, error) {
	procs, err := getLastData(dockerID)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(byCPU(procs)))
	if limit > len(procs) {
		limit = len(procs)
	}
	var result []customProcs
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%f%% CPU %s\n", p.CPUUsage, p.Cmdline)
	}
	return result, nil
}

// getTopMem returns `limit` entries with top VmRSS usage
func getTopMem(dockerID string, limit int) ([]customProcs, error) {
	procs, err := getLastData(dockerID)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(byRSS(procs)))
	if limit > len(procs) {
		limit = len(procs)
	}
	var result []customProcs
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%dKb %s\n", p.Status.VmRSS, p.Cmdline)
	}
	return result, nil
}

// getDockerIDs collects all docker ids from cgroups pseudo-filesystem
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
			name := fi.Name()
			if len(name) > '8' {
				results = append(results, name)
			}
		}
	}

	return results, nil
}

// httpHandler handles http requests
func httpHandler(res http.ResponseWriter, req *http.Request) {
	var validPath = regexp.MustCompile("^/([a-zA-Z0-9]+)/(mem|cpu|all)$")
	m := validPath.FindStringSubmatch(req.URL.Path)
	if m == nil {
		http.NotFound(res, req)
		return
	}
	limitStr := req.URL.Query().Get("limit")
	limit, err := strconv.ParseInt(limitStr, 10, 0)
	if err != nil || limit < 1 {
		limit = 5
	}
	dockerID := m[1]
	var result []customProcs
	switch m[2] {
	case "cpu":
		result, err = getTopCPU(dockerID, int(limit))
	case "mem":
		result, err = getTopMem(dockerID, int(limit))
	case "all":
		result, err = getLastData(dockerID)
	}
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

// collectData scrapes procs data for all docker containers
// every second, and sends through channel to getData()
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
		for i, v := range history[1:] {
			history[i] = v
		}
		history[59] = entry
		time.Sleep(time.Second)
	}
}

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("cAdvisor-companion version %s\n", version)
		os.Exit(0)
	}

	go collectData()
	//go processData()
	addr := fmt.Sprintf("%s:%d", *argIP, *argPort)
	fmt.Printf("Starting cAdvisor-companion version: %q on port %d\n", version, *argPort)
	http.HandleFunc("/", httpHandler)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
