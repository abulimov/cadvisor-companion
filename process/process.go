package process

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	linuxproc "github.com/c9s/goprocinfo/linux"
)

// Process is our extended linuxproc.Process type
// with custom cgroup and cpuUsage attributes
type Process struct {
	Status  linuxproc.ProcessStatus `json:"status"`
	Stat    linuxproc.ProcessStat   `json:"stat"`
	Cmdline string                  `json:"cmdline"`
	// RelativeCPUUsage is the percent of total CPU time used by container
	// that was used by this particular process in some time interval.
	// This value IS NOT traditional %CPU usage, which is the amount
	// of total available CPU time, used by particular process.
	// So, if all processes in given container used 10% of available host
	// resources, RelativeCPUUsage of 90% would mean that this process used
	// 9% of available host CPU resources.
	RelativeCPUUsage float64 `json:"relativecpuusage"`
	Cgroup           string  `json:"cgroup"`
}

// List of Processes
type List []Process

// ByCPU helps us sort array of Process by RelativeCPUUsage
type ByCPU List

// ByRSS helps us sort array of Process by Status.VmRSS
type ByRSS List

func (p ByRSS) Len() int {
	return len(p)
}
func (p ByRSS) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p ByRSS) Less(i, j int) bool {
	return p[i].Status.VmRSS < p[j].Status.VmRSS
}

func (p ByCPU) Len() int {
	return len(p)
}
func (p ByCPU) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p ByCPU) Less(i, j int) bool {
	return p[i].RelativeCPUUsage < p[j].RelativeCPUUsage
}

// readProcessCgroup reads and parses /proc/{pid}/cgroup file
func readProcessCgroup(pid, procPath string) (string, error) {
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

// GetProcesses returns list of processes that are in any cgroup
func GetProcesses(rootPath string) (List, error) {
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
				cgroup, err := readProcessCgroup(pid, path)
				if err != nil {
					continue
				}
				var stat *linuxproc.ProcessStat
				var status *linuxproc.ProcessStatus
				var cmdline string
				// we collect only processes from containers
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

// FindProc searches for given pid in list of processes and returns
// its process if found
func (procs List) FindProc(pid uint64) *Process {
	for _, p := range procs {
		if p.Status.Pid == pid {
			return &p
		}
	}
	return nil
}

// GetCPUTotalUsage returns total amount of CPU time used by list of given procs
func (procs List) GetCPUTotalUsage() int64 {
	totalUsage := int64(0)
	for _, p := range procs {
		user := int64(p.Stat.Utime)
		system := int64(p.Stat.Stime)
		totalUsage += user + system
	}
	return totalUsage
}

// GetCgroupsMap collects all possible cgroups found in []Process
func (procs List) GetCgroupsMap() map[string]List {
	cgroupsMap := make(map[string]List, 0)
	for _, p := range procs {
		cgroupsMap[p.Cgroup] = append(cgroupsMap[p.Cgroup], p)
	}
	return cgroupsMap
}
