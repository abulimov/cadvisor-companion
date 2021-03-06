// Copyright 2015 Alexander Bulimov. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package process

import (
	"testing"
)

func TestReadProcessCgroupRoot(t *testing.T) {
	cgroup, err := ReadProcessCgroup("./testroot/proc/2309/cgroup")

	if err != nil {
		t.Fatal("process cgroup read fail", err)
	}

	expected := "/"
	if cgroup != expected {
		t.Errorf("%s not equal to expected %s", cgroup, expected)
	}
}

func TestReadProcessCgroupDocker(t *testing.T) {
	cgroup, err := ReadProcessCgroup("./testroot/proc/8743/cgroup")

	if err != nil {
		t.Fatal("process cgroup read fail", err)
	}

	expected := "/docker/325898765f2a47af0ea45addd6632d8ea555b9615f83a3bd38857b6c4cabb53a"
	if cgroup != expected {
		t.Errorf("%s not equal to expected %s", cgroup, expected)
	}
}

func TestReadProcessCgroupDockerSystemd(t *testing.T) {
	cgroup, err := ReadProcessCgroup("./testroot/proc/22291/cgroup")

	if err != nil {
		t.Fatal("process cgroup read fail", err)
	}

	expected := "/system.slice/docker-7dc0bf65ec30b7d45868963cf1186a18a42fcf30d5a2df2002678bd0a1b31cad.scope"
	if cgroup != expected {
		t.Errorf("%s not equal to expected %s", cgroup, expected)
	}
}

func TestGetProcesses(t *testing.T) {
	procs, err := GetProcesses("./testroot")
	if err != nil {
		t.Fatal("process reading fail", err)
	}
	expected := 7
	if len(procs) != expected {
		t.Errorf("%d not equal to expected %d", len(procs), expected)
	}
}

func TestCPUTotalUsageEmpty(t *testing.T) {
	procs := make(List, 0)
	usage := procs.GetCPUTotalUsage()
	expected := int64(0)
	if usage != expected {
		t.Errorf("%d not equal to expected %d", usage, expected)
	}
}

func TestCPUTotalUsageNormal(t *testing.T) {
	procs, err := GetProcesses("./testroot")
	if err != nil {
		t.Fatal("process reading fail", err)
	}
	usage := procs.GetCPUTotalUsage()
	expected := int64(110175)
	if usage != expected {
		t.Errorf("%d not equal to expected %d", usage, expected)
	}
}

func TestFindProcEmpty(t *testing.T) {
	procs := make(List, 0)
	p := procs.FindProc(2309)
	if p != nil {
		t.Errorf("%d not equal to expected nil", p)
	}
}
func TestFindProcNormal(t *testing.T) {
	procs, err := GetProcesses("./testroot")
	if err != nil {
		t.Fatal("process reading fail", err)
	}
	p := procs.FindProc(6930)
	expected := "/usr/bin/cadvisor-companion"
	if p.Cmdline != expected {
		t.Errorf("%s not equal to expected %s", p.Cmdline, expected)
	}
}

func TestGetCgroupsMapEmpty(t *testing.T) {
	procs := make(List, 0)
	cgroupsMap := procs.GetCgroupsMap()
	expected := 0
	if len(cgroupsMap) != expected {
		t.Errorf("%d not equal to expected %d", len(cgroupsMap), expected)
	}
}

func TestGetCgroupsMapNormal(t *testing.T) {
	procs, err := GetProcesses("./testroot")
	if err != nil {
		t.Fatal("process reading fail", err)
	}
	cgroupsMap := procs.GetCgroupsMap()
	expected := "/docker/325898765f2a47af0ea45addd6632d8ea555b9615f83a3bd38857b6c4cabb53a"
	procs, ok := cgroupsMap[expected]
	if !ok {
		t.Errorf("%s not found in cgroupsMap", expected)
	}
	expectedLen := 3
	if len(procs) != expectedLen {
		t.Errorf("%d not equal to expected %d", len(procs), expectedLen)
	}
}
