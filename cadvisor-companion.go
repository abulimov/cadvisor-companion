// Copyright 2015 Alexander Bulimov. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	proc "github.com/abulimov/cadvisor-companion/process"
)

var version = "0.1.1"

// set up cli vars
var argIP = flag.String("listen_ip", "", "IP to listen on, defaults to all IPs")
var argPort = flag.Int("port", 8801, "port to listen")
var versionFlag = flag.Bool("version", false, "print cAdvisor-companion version and exit")

var history proc.HistoryDB

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
	if err != nil || limit < 0 {
		limit = 0
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
	var result []proc.Snapshot
	var ps *proc.Snapshot
	fail := func(err error) {
		message := map[string]string{"error": err.Error()}
		messageJSON, _ := json.Marshal(message)
		fmt.Printf("Error: %s\n", err.Error())
		res.WriteHeader(500) // HTTP 500
		io.WriteString(res, string(messageJSON))
		return
	}

	// get data for all `count` HistoryEntries
	for i := count - 1; i >= 0; i-- {
		// case our possible sort parameters
		switch sortStr {
		case "cpu":
			ps, err = history.GetTopCPU(containerID, limit, interval, i*interval+1)
		case "mem":
			ps, err = history.GetTopMem(containerID, limit, interval, i*interval+1)
		case "":
			ps, err = history.GetLastData(containerID, interval, i*interval+1)
		}
		if err != nil {
			fail(err)
			return
		}
		result = append(result, *ps)
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

		entry := make(proc.HistoryEntry)
		for e, p := range cgroupsProcs {
			entry[e] = proc.Snapshot{Timestamp: timeStamp, Processes: p}
		}
		history.Push(entry)

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
