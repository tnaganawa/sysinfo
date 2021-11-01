// Copyright © 2016 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// CPU information.
type CPU struct {
	Vendor  string `json:"vendor,omitempty"`
	Model   string `json:"model,omitempty"`
	Speed   uint   `json:"speed,omitempty"`   // CPU clock rate in MHz
	Cache   uint   `json:"cache,omitempty"`   // CPU cache size in KB
	Cpus    uint   `json:"cpus,omitempty"`    // number of physical CPUs
	Cores   uint   `json:"cores,omitempty"`   // number of physical CPU cores
	Threads uint   `json:"threads,omitempty"` // number of logical (HT) CPU cores
	Flags   string  `json:"flags,omitempty"`  // CPU flags

}

var (
	reTwoColumns = regexp.MustCompile("\t+: ")
	reExtraSpace = regexp.MustCompile(" +")
	reCacheSize  = regexp.MustCompile(`^(\d+) KB$`)
	cpuInfo      = "/proc/cpuinfo"
)

func (si *SysInfo) getCPUInfo() {
	si.CPU.Threads = uint(runtime.NumCPU())

	f, err := os.Open(cpuInfo)
	if err != nil {
		return
	}
	defer f.Close()

	cpu := make(map[string]bool)
	core := make(map[string]bool)

	// for virtualized environment
	cpuCount := 0
	coreCount := 0

	var cpuID string

	s := bufio.NewScanner(f)
	for s.Scan() {
		//log.Printf("Line: %s", s.Text())
		if sl := reTwoColumns.Split(s.Text(), 2); sl != nil {
			switch sl[0] {
			case "processor":
				cpuID = sl[1]
				cpu[cpuID] = true

				cpuCount += 1
			case "core id":
				coreID := fmt.Sprintf("%s/%s", cpuID, sl[1])
				core[coreID] = true
			case "cpu cores":
				c, err := strconv.ParseInt(sl[1], 10, 8)
				if err == nil {
					coreCount += int(c)
				}
				coreCount += 1
			case "vendor_id":
				if si.CPU.Vendor == "" {
					si.CPU.Vendor = sl[1]
				}
			case "flags":
				if si.CPU.Flags == "" {
					si.CPU.Flags = sl[1]
				}
			case "model name":
				if si.CPU.Model == "" {
					// CPU model, as reported by /proc/cpuinfo, can be a bit ugly. Clean up...
					model := reExtraSpace.ReplaceAllLiteralString(sl[1], " ")
					si.CPU.Model = strings.Replace(model, "- ", "-", 1)
				}
			case "cpu MHz":
				if si.CPU.Speed == uint(0) {
					i, err := strconv.ParseFloat(sl[1], 32)
					if err == nil {
						si.CPU.Speed = uint(i)
					}
				}
			case "cache size":
				if si.CPU.Cache == 0 {
					if m := reCacheSize.FindStringSubmatch(sl[1]); m != nil {
						if cache, err := strconv.ParseUint(m[1], 10, 64); err == nil {
							si.CPU.Cache = uint(cache)
						}
					}
				}
			}
		}
	}
	if s.Err() != nil {
		return
	}

	// getNodeInfo() must have run first, to detect if we're dealing with a virtualized CPU! Detecting number of
	// physical processors and/or cores is totally unreliable in virtualized environments, so let's not do it.
	if si.Node.Hostname == "" || si.Node.Hypervisor != "" {
		// fallback to counts when virtualized
		si.CPU.Cpus = uint(cpuCount)
		si.CPU.Cores = uint(coreCount)
	}

	si.CPU.Cpus = uint(len(cpu))
	si.CPU.Cores = uint(len(core))
}
