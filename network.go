// Copyright © 2016 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package sysinfo

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"
	"syscall"
	"unsafe"
)

// NetworkDevice information.
type NetworkDevice struct {
	Name        string   `json:"name,omitempty"`
	Driver      string   `json:"driver,omitempty"`
	IpAddresses []string `json:"ip_address,omitempty"`
	MACAddress  string   `json:"mac_address,omitempty"`
	Port        string   `json:"port,omitempty"`
	Speed       uint     `json:"speed,omitempty"` // device max supported speed in Mbps
}

func getPortType(supp uint32) (port string) {
	for i, p := range [...]string{"tp", "aui", "mii", "fibre", "bnc"} {
		if supp&(1<<uint(i+7)) > 0 {
			port += p + "/"
		}
	}

	port = strings.TrimRight(port, "/")
	return
}

func getPortTypeForGLinkSetting(supp uint8) (port string) {
	if supp == 0x00 {
		port = "twisted pair"
	} else if supp == 0x01 {
		port = "AUI"
	} else if supp == 0x02 {
		port = "media-independent"
	} else if supp == 0x03 {
		port = "fibre"
	} else if supp == 0x04 {
		port = "BNC"
	} else if supp == 0x05 {
		port = "direct attach"
	} else if supp == 0xef {
		port = "none"
	} else if supp == 0xff {
		port = "other"
	}
	return
}

func getMaxSpeed(supp uint32) (speed uint) {
	// Fancy, right?
	switch {
	case supp&0x78000000 > 0:
		speed = 56000
	case supp&0x07800000 > 0:
		speed = 40000
	case supp&0x00600000 > 0:
		speed = 20000
	case supp&0x001c1000 > 0:
		speed = 10000
	case supp&0x00008000 > 0:
		speed = 2500
	case supp&0x00020030 > 0:
		speed = 1000
	case supp&0x0000000c > 0:
		speed = 100
	case supp&0x00000003 > 0:
		speed = 10
	}

	return
}

func getSupported(name string) uint32 {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_IP)
	if err != nil {
		return 0
	}
	defer syscall.Close(fd)

	// struct ethtool_cmd from /usr/include/linux/ethtool.h
	var ethtool struct {
		Cmd           uint32
		Supported     uint32
		Advertising   uint32
		Speed         uint16
		Duplex        uint8
		Port          uint8
		PhyAddress    uint8
		Transceiver   uint8
		Autoneg       uint8
		MdioSupport   uint8
		Maxtxpkt      uint32
		Maxrxpkt      uint32
		SpeedHi       uint16
		EthTpMdix     uint8
		Reserved2     uint8
		LpAdvertising uint32
		Reserved      [2]uint32
	}

	// ETHTOOL_GSET from /usr/include/linux/ethtool.h
	const GSET = 0x1

	ethtool.Cmd = GSET

	// struct ifreq from /usr/include/linux/if.h
	var ifr struct {
		Name [16]byte
		Data uintptr
	}

	copy(ifr.Name[:], name+"\000")
	ifr.Data = uintptr(unsafe.Pointer(&ethtool))

	// SIOCETHTOOL from /usr/include/linux/sockios.h
	const SIOCETHTOOL = 0x8946

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(SIOCETHTOOL), uintptr(unsafe.Pointer(&ifr)))
	if errno == 0 {
		return ethtool.Supported
	}

	return 0
}

// struct ethtool_cmd from include/uapi/linux/ethtool.h
type ethtoolLinkSettingType struct {
	Cmd                 uint32
	Speed               uint32
	Duplex              uint8
	Port                uint8
	PhyAddress          uint8
	Autoneg             uint8
	MdioSupport         uint8
	EthTpMdix           uint8
	EthTpMdixCtrl       uint8
	LinkModeMasksNwords int8
	Transceiver         uint8
	Reserved1           [3]uint32
	Reserved            [7]uint32
	LinkModeMasks       [0]uint32
}

func getSupportedWithEthtoolGLinkSetting(name string) (*ethtoolLinkSettingType, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_IP)
	if err != nil {
		return nil, fmt.Errorf("error: syscall socket err, %s", err)
	}
	defer syscall.Close(fd)

	// ETHTOOL_GLINKSETTINGS from include/uapi/linux/ethtool.h
	const GLINKSETTING = 0x0000004c

	var ethtoolLinkSetting ethtoolLinkSettingType
	ethtoolLinkSetting.Cmd = GLINKSETTING

	// struct ifreq from include/uapi/linux/if.h
	var ifr struct {
		Name [16]byte
		Data uintptr
	}

	copy(ifr.Name[:], name+"\000")
	ifr.Data = uintptr(unsafe.Pointer(&ethtoolLinkSetting))

	// SIOCETHTOOL from /usr/include/linux/sockios.h
	const SIOCETHTOOL = 0x8946

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(SIOCETHTOOL), uintptr(unsafe.Pointer(&ifr)))
	if errno == 0 {
		if ethtoolLinkSetting.LinkModeMasksNwords >= 0 || ethtoolLinkSetting.Cmd != 0x0000004c {
			return nil, fmt.Errorf("error: link mode mask nwords check, %d", ethtoolLinkSetting.LinkModeMasksNwords)
		}
		ethtoolLinkSetting.LinkModeMasksNwords = -ethtoolLinkSetting.LinkModeMasksNwords

		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(SIOCETHTOOL), uintptr(unsafe.Pointer(&ifr)))
		if errno == 0 {
			return &ethtoolLinkSetting, nil
		} else {
			return nil, fmt.Errorf("error: GLinkSetting err with link mode mask, %#v", errno)
		}

	} else {
		return nil, fmt.Errorf("error: GLinkSetting err, %#v", errno)
	}
	/*
	   return nil, fmt.Errorf("error: should not reach here, errno, %d", errno)
	*/
}

func (si *SysInfo) getNetworkInfo() {
	sysClassNet := "/sys/class/net"
	devices, err := ioutil.ReadDir(sysClassNet)
	if err != nil {
		return
	}

	si.Network = make([]NetworkDevice, 0)
	for _, link := range devices {
		fullpath := path.Join(sysClassNet, link.Name())
		_, err := os.Readlink(fullpath)
		if err != nil {
			continue
		}

		/* Use virtual intefaces as well
		if strings.HasPrefix(dev, "../../devices/virtual/") {
			continue
		}
		*/

		gLinkSetting, err := getSupportedWithEthtoolGLinkSetting(link.Name())
		var portType string
		var maxSpeed uint

		if err != nil {
			// fmt.Printf("err, gLinkSetting, fallback to GSET, err: %s", err)

			supp := getSupported(link.Name())
			portType = getPortType(supp)
			maxSpeed = getMaxSpeed(supp)
		} else {
			portType = getPortTypeForGLinkSetting(gLinkSetting.Port)
			maxSpeed = uint(gLinkSetting.Speed)
		}

		deviceAddresses := []string{}
		byNameInterface, _ := net.InterfaceByName(link.Name())
		if err == nil {
			addresses, _ := byNameInterface.Addrs()
			for _, v := range addresses {
				deviceAddresses = append(deviceAddresses, v.String())
			}
		}

		device := NetworkDevice{
			Name:        link.Name(),
			MACAddress:  slurpFile(path.Join(fullpath, "address")),
			Port:        portType,
			Speed:       maxSpeed,
			IpAddresses: deviceAddresses,
		}

		if driver, err := os.Readlink(path.Join(fullpath, "device", "driver")); err == nil {
			device.Driver = path.Base(driver)
		}

		si.Network = append(si.Network, device)
	}
}
