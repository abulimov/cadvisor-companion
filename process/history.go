package process

import (
	"errors"
	"sort"
	"time"
)

// HistoryDB holds RRD-like array of last 60 history entries
type HistoryDB [60]HistoryEntry

// Snapshot represents slice of processes in time
type Snapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Processes List      `json:"processes"`
}

// HistoryEntry containes all Snapshots for some moment in time
type HistoryEntry map[string]Snapshot

// Push adds new entry to our History, and rotates old data
func (history *HistoryDB) Push(entry HistoryEntry) {
	// rotate our history
	for i, v := range history[1:] {
		history[i] = v
	}
	history[len(history)-1] = entry
}

// GetLastData returns data from history with added relative CPU usage
// offset (in seconds) lets us get data from the past.
// interval (in seconds) is used to calculate CPU usage
func (history *HistoryDB) GetLastData(containerID string, interval, offset int) (*Snapshot, error) {
	if len(history) < offset+interval || offset < 1 || interval < 1 {
		return nil, errors.New("Wrong offset and interval combination")
	}
	last := len(history) - offset
	first := last - interval
	entry1 := history[first][containerID]
	entry2 := history[last][containerID]
	var procs List
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
	return &Snapshot{entry2.Timestamp, procs}, nil
}

// GetTopCPU returns `limit` entries with top CPU usage
func (history *HistoryDB) GetTopCPU(containerID string, limit, interval, offset int) (*Snapshot, error) {
	entry, err := history.GetLastData(containerID, interval, offset)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(ByCPU(entry.Processes)))
	if limit > len(entry.Processes) {
		limit = len(entry.Processes)
	}
	var result Snapshot
	result.Timestamp = entry.Timestamp
	for _, p := range entry.Processes[:limit] {
		result.Processes = append(result.Processes, p)
	}
	return &result, nil
}

// GetTopMem returns `limit` entries with top VmRSS usage
func (history *HistoryDB) GetTopMem(containerID string, limit, interval, offset int) (*Snapshot, error) {
	entry, err := history.GetLastData(containerID, interval, offset)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(ByRSS(entry.Processes)))
	if limit > len(entry.Processes) {
		limit = len(entry.Processes)
	}
	var result Snapshot
	result.Timestamp = entry.Timestamp
	for _, p := range entry.Processes[:limit] {
		result.Processes = append(result.Processes, p)
	}
	return &result, nil
}
