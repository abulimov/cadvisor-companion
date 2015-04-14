package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"time"

	proc "github.com/abulimov/cadvisor-companion/process"
)

var version = "0.0.5"

// set up cli vars
var argIP = flag.String("listen_ip", "", "IP to listen on, defaults to all IPs")
var argPort = flag.Int("port", 8801, "port to listen")
var versionFlag = flag.Bool("version", false, "print cAdvisor-companion version and exit")

// ProcessSnapshot represents slice of processes in time
type ProcessSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Processes proc.List `json:"processes"`
}

// HistoryEntry containes all ProcessSnapshots for some moment in time
type HistoryEntry map[string]ProcessSnapshot

// history holds RRD-like array of last 60 history entries
var history [60]HistoryEntry

// getLastData returns data from history with added relative CPU usage
// offset (in seconds) lets us get data from the past.
// interval (in seconds) is used to calculate CPU usage
func getLastData(containerID string, interval, offset int) (*ProcessSnapshot, error) {
	last := len(history) - offset
	first := last - interval
	entry1 := history[first][containerID]
	entry2 := history[last][containerID]
	var procs proc.List
	cpu1 := entry1.Processes.GetCPUTotalUsage()
	cpu2 := entry2.Processes.GetCPUTotalUsage()
	for _, p2 := range entry2.Processes {
		p1 := entry1.Processes.FindProc(p2.Status.Pid)
		if p1 != nil {
			user := int64(p2.Stat.Utime - p1.Stat.Utime)
			system := int64(p2.Stat.Stime - p1.Stat.Stime)
			percent := (float64(user+system) / float64(cpu2-cpu1)) * 100
			p2.RelativeCPUUsage = percent
			procs = append(procs, p2)
		}

	}
	return &ProcessSnapshot{entry2.Timestamp, procs}, nil
}

// getTopCPU returns `limit` entries with top CPU usage
func getTopCPU(containerID string, limit, interval, offset int) (*ProcessSnapshot, error) {
	entry, err := getLastData(containerID, interval, offset)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(proc.ByCPU(entry.Processes)))
	if limit > len(entry.Processes) {
		limit = len(entry.Processes)
	}
	var result ProcessSnapshot
	result.Timestamp = entry.Timestamp
	for _, p := range entry.Processes[:limit] {
		result.Processes = append(result.Processes, p)
	}
	return &result, nil
}

// getTopMem returns `limit` entries with top VmRSS usage
func getTopMem(containerID string, limit, interval, offset int) (*ProcessSnapshot, error) {
	entry, err := getLastData(containerID, interval, offset)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(proc.ByRSS(entry.Processes)))
	if limit > len(entry.Processes) {
		limit = len(entry.Processes)
	}
	var result ProcessSnapshot
	result.Timestamp = entry.Timestamp
	for _, p := range entry.Processes[:limit] {
		result.Processes = append(result.Processes, p)
	}
	return &result, nil
}

// apiHandler handles http requests
func apiHandler(res http.ResponseWriter, req *http.Request) {
	var validPath = regexp.MustCompile("^/api/v1.0/([a-zA-Z0-9/]+)/processes$")

	// validate requested URL
	m := validPath.FindStringSubmatch(req.URL.Path)
	if m == nil {
		http.NotFound(res, req)
		return
	}

	// process get parameters

	// sortStr is used to get top sorted procs
	sortStr := req.URL.Query().Get("sort")

	// limit is used to limit top sorted procs
	limitStr := req.URL.Query().Get("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 5
	}

	// interval is the interval we use to calculate CPU usage
	// and to iterate back to the past
	intervalStr := req.URL.Query().Get("interval")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 1 {
		interval = 1
	}

	// count is the count of resulting points in time
	countStr := req.URL.Query().Get("count")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 {
		count = 1
	}

	// containerID is the name of our container/cgroup
	containerID := "/" + m[1]

	// set response headers
	res.Header().Set(
		"Content-Type",
		"text/json",
	)
	var result []ProcessSnapshot
	var ps *ProcessSnapshot
	// case our possible sort parameters
	switch sortStr {
	case "cpu":
		for i := count - 1; i >= 0; i-- {
			ps, err = getTopCPU(containerID, limit, interval, i*interval+1)
			result = append(result, *ps)
		}
	case "mem":
		for i := count - 1; i >= 0; i-- {
			ps, err = getTopMem(containerID, limit, interval, i*interval+1)
			result = append(result, *ps)
		}
	case "":
		for i := count - 1; i >= 0; i-- {
			ps, err = getLastData(containerID, interval, i*interval+1)
			result = append(result, *ps)
		}
	}
	if err != nil {
		fmt.Println(err)
		res.WriteHeader(500) // HTTP 500
		io.WriteString(res, err.Error())
	}
	jsonResult, _ := json.Marshal(result)
	io.WriteString(res, string(jsonResult))
}

// collectData scrapes procs data for all containers every second
// and keeps it in global history var
func collectData(rootPath string) {
	for {
		timeStamp := time.Now()
		// get all processes without cgroup grouping
		allProcs, _ := proc.GetProcesses(rootPath)
		// group all processes by their cgroups
		cgroupsProcs := allProcs.GetCgroupsMap()

		entry := make(HistoryEntry)
		for e, p := range cgroupsProcs {
			entry[e] = ProcessSnapshot{timeStamp, p}
		}

		// rotate our history
		for i, v := range history[1:] {
			history[i] = v
		}
		history[len(history)-1] = entry
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
	// When we are inside the container, host's /proc should be mounted
	// at /rootfs/proc, so rootPath is /rootfs
	rootPath := getRootPath()

	// start collecting data
	go collectData(rootPath)

	addr := fmt.Sprintf("%s:%d", *argIP, *argPort)
	fmt.Printf("Starting cAdvisor-companion version: %q on port %d\n", version, *argPort)
	http.HandleFunc("/api/", apiHandler)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
