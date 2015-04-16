// Copyright 2015 Alexander Bulimov. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package process

import (
	"testing"
	"time"
)

func TestHistoryPush(t *testing.T) {
	var history HistoryDB
	timeStamp := time.Now()
	procs, err := GetProcesses("./testroot")
	if err != nil {
		t.Fatal("process reading fail", err)
	}
	entry := make(HistoryEntry)
	entry["test"] = Snapshot{timeStamp, procs}
	// save some reference number
	p := procs.FindProc(uint64(6930))
	expectedCPUUserTime := p.Stat.Cutime

	// push crafted entry to history
	history.Push(entry)

	// get processes from history
	historyProcs := history[len(history)-1]["test"].Processes

	// get reference number
	historyP := historyProcs.FindProc(uint64(6930))
	historyCPUUserTime := historyP.Stat.Cutime
	if expectedCPUUserTime != historyCPUUserTime {
		t.Errorf("%s not equal to expected %s", historyCPUUserTime, expectedCPUUserTime)
	}
}

func TestHistoryLastData(t *testing.T) {
	var history HistoryDB
	timeStamp1 := time.Now()
	procs1, err := GetProcesses("./testroot")
	if err != nil {
		t.Fatal("process reading fail", err)
	}
	// create first entry
	entry1 := make(HistoryEntry)
	entry1["test"] = Snapshot{timeStamp1, procs1}
	timeStamp2 := time.Now()

	// create procs for second entry
	procs2 := make(List, 0)
	for _, p := range procs1 {
		// we s
		if p.Stat.Pid != 6930 {
			procs2 = append(procs2, p)
		}
	}

	// update Utime for one particular process
	someP := procs1.FindProc(uint64(6930))
	if someP != nil {
		someP.Stat.Utime += 500
	} else {
		t.Fatal("failed to get proc")
	}
	procs2 = append(procs2, *someP)

	// create second entry
	entry2 := make(HistoryEntry)
	entry2["test"] = Snapshot{timeStamp2, procs2}

	// push both entries to history
	history.Push(entry1)
	history.Push(entry2)
	snap, err := history.GetLastData("test", 1, 1)
	if err != nil {
		t.Fatal("getting history last data failed", err)
	}

	// check caculated RelativeCPUUsage for one particular process
	gotP := snap.Processes.FindProc(uint64(6930))
	expectedCPUUsage := float64(100)
	gotCPUUsage := gotP.RelativeCPUUsage
	if expectedCPUUsage != gotCPUUsage {
		t.Errorf("%f not equal to expected %f", gotCPUUsage, expectedCPUUsage)
	}
}
