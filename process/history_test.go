// Copyright 2015 Alexander Bulimov. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package process

import (
	"testing"
	"time"
)

func prepareHistory() (*HistoryDB, error) {
	var history HistoryDB
	timeStamp1 := time.Now()
	procs1, err := GetProcesses("./testroot")
	if err != nil {
		return nil, err
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
		return nil, err
	}
	procs2 = append(procs2, *someP)

	// create second entry
	entry2 := make(HistoryEntry)
	entry2["test"] = Snapshot{timeStamp2, procs2}

	// push both entries to history
	history.Push(entry1)
	history.Push(entry2)
	return &history, nil
}

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
	history, err := prepareHistory()
	if err != nil {
		t.Fatal("preparing history failed", err)
	}
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

func TestHistoryLastDataWrongConstraints(t *testing.T) {
	history, err := prepareHistory()
	if err != nil {
		t.Fatal("preparing history failed", err)
	}
	_, err = history.GetLastData("test", 1, 100)
	if err == nil {
		t.Error("GetLastData with wrong offset didn't failed when expected to fail")
	}

	_, err = history.GetLastData("test", 100, 1)
	if err == nil {
		t.Error("GetLastData with wrong interval didn't failed when expected to fail")
	}
}

func TestHistoryLastDataContainerName(t *testing.T) {
	history, err := prepareHistory()
	if err != nil {
		t.Fatal("preparing history failed", err)
	}
	_, err = history.GetLastData("missing", 1, 1)
	if err == nil {
		t.Error("GetLastData with wrong container name didn't failed when expected to fail")
	}
}

func TestHistoryTopCPU(t *testing.T) {
	history, err := prepareHistory()
	if err != nil {
		t.Fatal("preparing history failed", err)
	}
	snap, err := history.GetTopCPU("test", 1, 1, 1)
	if err != nil {
		t.Fatal("getting history TopCPU failed", err)
	}

	gotP := snap.Processes[0]
	expectedCPUUsage := float64(100)
	gotCPUUsage := gotP.RelativeCPUUsage
	if expectedCPUUsage != gotCPUUsage {
		t.Errorf("%f not equal to expected %f", gotCPUUsage, expectedCPUUsage)
	}
}

func TestHistoryTopCPUWrongConstraints(t *testing.T) {
	history, err := prepareHistory()
	if err != nil {
		t.Fatal("preparing history failed", err)
	}
	_, err = history.GetTopCPU("test", 1, 1, 100)
	if err == nil {
		t.Error("GetTopCPU with wrong offset didn't failed when expected to fail")
	}

	_, err = history.GetTopCPU("test", 1, 100, 1)
	if err == nil {
		t.Error("GetTopCPU with wrong interval didn't failed when expected to fail")
	}

	_, err = history.GetTopCPU("test", 100, 1, 1)
	if err != nil {
		t.Error("GetTopCPU with wrong limit failed when should not")
	}
}

func TestHistoryTopMem(t *testing.T) {
	history, err := prepareHistory()
	if err != nil {
		t.Fatal("preparing history failed", err)
	}
	snap, err := history.GetTopMem("test", 1, 1, 1)
	if err != nil {
		t.Fatal("getting history TopMem failed", err)
	}

	gotP := snap.Processes[0]
	expectedVMRSS := uint64(53984)
	gotVMRSS := gotP.Status.VmRSS
	if expectedVMRSS != gotVMRSS {
		t.Errorf("%d not equal to expected %d", gotVMRSS, expectedVMRSS)
	}
}

func TestHistoryTopMemWrongConstraints(t *testing.T) {
	history, err := prepareHistory()
	if err != nil {
		t.Fatal("preparing history failed", err)
	}
	_, err = history.GetTopMem("test", 1, 1, 100)
	if err == nil {
		t.Error("GetTopMem with wrong offset didn't failed when expected to fail")
	}

	_, err = history.GetTopMem("test", 1, 100, 1)
	if err == nil {
		t.Error("GetTopMem with wrong interval didn't failed when expected to fail")
	}

	_, err = history.GetTopMem("test", 100, 1, 1)
	if err != nil {
		t.Error("GetTopMem with wrong limit failed when should not")
	}
}
