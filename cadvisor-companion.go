package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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

// Process is our extended linuxproc.Process type
// with custom cgroup and cpuUsage attributes
type Process struct {
	Status   linuxproc.ProcessStatus `json:"status"`
	Stat     linuxproc.ProcessStat   `json:"stat"`
	Cmdline  string                  `json:"cmdline"`
	CPUUsage float64
	Cgroup   string
}

// byCPU helps us sort array of Process by CPUUsage
type byCPU []Process

// byRSS helps us sort array of Process by Status.VmRSS
type byRSS []Process

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
	processes []Process
	cpuStat   uint64
}

// historyEntry containes all dockerSnapshots for some moment in time
type historyEntry map[string]dockerSnapshot

// history holds RRD-like array of last 60 history entries
var history [60]historyEntry

// readCgroup reads and parses /proc/{pid}/cgroup file
func readCgroup(pid, procPath string) (string, error) {
	cgroupPath := fmt.Sprintf("%s/%s/cgroup", procPath, pid)
	dataBytes, err := ioutil.ReadFile(cgroupPath)
	if err != nil {
		return "/", err
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
	return "/", nil
}

// getProcesses returns list of processes that are in any cgroup
func getProcesses(rootPath string) ([]Process, error) {
	path := filepath.Join(rootPath, "/proc/")
	d, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	procs := make([]Process, 0, 50)
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
			if name[0] < '0' || name[0] > '9' {
				continue
			}

			// From this point forward, any errors we just ignore, because
			// it might simply be that the process doesn't exist anymore.
			_, err := strconv.ParseUint(name, 10, 64)
			if err == nil {
				pid := name
				cgroup, err := readCgroup(pid, path)
				if err != nil {
					continue
				}
				var stat *linuxproc.ProcessStat
				var status *linuxproc.ProcessStatus
				var cmdline string
				if cgroup != "/" {
					p := Process{}
					if stat, err = linuxproc.ReadProcessStat(filepath.Join(path, pid, "stat")); err != nil {
						continue
					}
					if status, err = linuxproc.ReadProcessStatus(filepath.Join(path, pid, "status")); err != nil {
						continue
					}
					if cmdline, err = linuxproc.ReadProcessCmdline(filepath.Join(path, pid, "cmdline")); err != nil {
						continue
					}
					p.Cmdline = cmdline
					p.Cgroup = cgroup
					p.Stat = *stat
					p.Status = *status
					if p.Cmdline != "" && p.Status.VmRSS > 0 {
						procs = append(procs, p)
					}
				}
			}
		}
	}
	return procs, nil
}

// getCPUTotalUsage returns amount of CPU time, used by given docker container
func getCPUTotalUsage(dockerID string) (uint64, error) {
	cpuAcctPath := fmt.Sprintf("/sys/fs/cgroup/cpuacct/%s/cpuacct.stat", dockerID)
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
func findProc(pid uint64, procs []Process) *Process {
	for _, p := range procs {
		if p.Status.Pid == pid {
			return &p
		}
	}
	return nil
}

// getCgroups collects all possible cgroups found in []Process
func getCgroupsProcs(procs []Process) map[string][]Process {
	cgroupsMap := make(map[string][]Process, 0)
	for _, p := range procs {
		cgroupsMap[p.Cgroup] = append(cgroupsMap[p.Cgroup], p)
	}
	return cgroupsMap
}

// getLastData returns latest data from history with added calculated CPU usage
func getLastData(dockerID string) ([]Process, error) {
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
	var procs []Process
	for _, p2 := range entry2.processes {
		p1 := findProc(p2.Status.Pid, entry1.processes)
		if p1 != nil {
			user := int64(p2.Stat.Utime-p1.Stat.Utime) + (p2.Stat.Cutime - p1.Stat.Cutime)
			system := int64(p2.Stat.Stime-p1.Stat.Stime) + (p2.Stat.Cstime - p1.Stat.Cstime)
			percent := (float64(user+system) / float64(entry2.cpuStat-entry1.cpuStat)) * 100
			p2.CPUUsage = percent
			procs = append(procs, p2)
		}

	}
	return procs, nil
}

// getTopCPU returns `limit` entries with top CPU usage
func getTopCPU(dockerID string, limit int) ([]Process, error) {
	procs, err := getLastData(dockerID)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(byCPU(procs)))
	if limit > len(procs) {
		limit = len(procs)
	}
	var result []Process
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%f%% CPU %s\n", p.CPUUsage, p.Cmdline)
	}
	return result, nil
}

// getTopMem returns `limit` entries with top VmRSS usage
func getTopMem(dockerID string, limit int) ([]Process, error) {
	procs, err := getLastData(dockerID)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(byRSS(procs)))
	if limit > len(procs) {
		limit = len(procs)
	}
	var result []Process
	for _, p := range procs[:limit] {
		result = append(result, p)
		//fmt.Printf("%dKb %s\n", p.Status.VmRSS, p.Cmdline)
	}
	return result, nil
}

// httpHandler handles http requests
func httpHandler(res http.ResponseWriter, req *http.Request) {
	var validPath = regexp.MustCompile("^/api/v1.0/([a-zA-Z0-9/]+)/(mem|cpu|all)$")
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
	dockerID := "/" + m[1]
	var result []Process
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

// collectData scrapes procs data for all docker containers every second
// and keeps it in global history var
func collectData(rootPath string) {
	for {
		entry := make(historyEntry)
		res, _ := getProcesses(rootPath)
		cgroups := getCgroupsProcs(res)
		for cg, procs := range cgroups {
			cpu, _ := getCPUTotalUsage(cg)
			entry[cg] = dockerSnapshot{procs, cpu}
		}
		for i, v := range history[1:] {
			history[i] = v
		}
		history[59] = entry
		time.Sleep(time.Second)
	}
}

func getRootPath() string {
	dir, err := os.Stat("/rootfs")
	if err != nil {
		return "/"
	}
	// check if the /rootfs is indeed a directory or not
	if !dir.IsDir() {
		return "/"
	}
	return "/rootfs"
}

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("cAdvisor-companion version %s\n", version)
		os.Exit(0)
	}

	// rootPath is where our /proc is mounted.
	// When we are in docker, host's /proc should be mounted
	// at /rootfs/proc, so rootPath is /rootfs
	rootPath := getRootPath()

	// start collecting data
	go collectData(rootPath)

	addr := fmt.Sprintf("%s:%d", *argIP, *argPort)
	fmt.Printf("Starting cAdvisor-companion version: %q on port %d\n", version, *argPort)
	http.HandleFunc("/", httpHandler)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
