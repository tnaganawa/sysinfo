package sysinfo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCPUInfo(t *testing.T) {
	// change cpuinfo file to testdata
	cpuInfoPrev := cpuInfo
	defer func() { cpuInfo = cpuInfoPrev }()
	cpuInfo = "testdata/cpuinfo"

	si := &SysInfo{}

	si.getCPUInfo()

	fmt.Printf("%#v\n", si.CPU)

	assert.Equal(t, "Intel(R) Xeon(R) Platinum 8175M CPU @ 2.50GHz", si.CPU.Model, "Model don't match")
	assert.Equal(t, "GenuineIntel", si.CPU.Vendor, "Vendor don't match")
	assert.Equal(t, uint(2500), si.CPU.Speed, "Speed don't match")
	assert.Equal(t, uint(2), si.CPU.Cpus, "Cpus don't match")
}
