// ifaddr.go - Simple utility to enumerate all interfaces
//
// Author: Sudhi Herle (sw@herle.net)
// License: GPLv2

package main

import (
	"fmt"
	flag "github.com/opencoff/pflag"
	"net"
	"os"
	"strings"
)

var V6, HW, Sh, All bool

func main() {

	flag.BoolVarP(&V6, "ipv6", "6", false, "Show IPv6 address")
	flag.BoolVarP(&HW, "mac", "m", false, "Show MAC address")
	flag.BoolVarP(&Sh, "shell", "s", false, "Export shell vars (sh/ksh/bash)")
	flag.BoolVarP(&All, "all", "a", false, "Also show loopback interface")

	usage := fmt.Sprintf("%s [options] [interface..]", os.Args[0])
	flag.Usage = func() {
		fmt.Printf("%s - Show one or more interface's addresses\nUsage: %s\n", os.Args[0], usage)
		flag.PrintDefaults()
	}

	var ifs []string

	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		for _, nm := range args {
			ii, err := net.InterfaceByName(nm)
			if err != nil {
				die("can't find interface %s", nm)
			}

			// If loopback is explicitly asked, we print it.
			if printIf(ii) {
				ifs = append(ifs, ii.Name)
			}
		}
	} else {
		iv, err := net.Interfaces()
		if err != nil {
			die("can't get interface address: %s", err)
		}

		for i := range iv {
			ii := &iv[i]
			if printIf(ii) {
				ifs = append(ifs, ii.Name)
			}
		}
	}

	if Sh {
		fmt.Printf("IFACES='%s'\n", strings.Join(ifs, " "))
	}
}

// Return true if we actually printed something, false otherwise
func printIf(ii *net.Interface) bool {
	av, err := ii.Addrs()
	if err != nil {
		die("can't get address for %s: %s", ii.Name, err)
	}

	var addrs []string
	var v6v []string
	for _, a := range av {
		ifa, ok := a.(*net.IPNet)
		if !ok {
			continue
		}

		if ifa.IP.IsLoopback() && !All {
			return false
		}

		ip := ifa.IP
		if ip.IsMulticast() {
			continue
		}

		if ip.To4() == nil {
			v6v = append(v6v, fmt.Sprintf("%s", ifa))
		} else {
			addrs = append(addrs, fmt.Sprintf("%s", ifa))
		}
	}

	if V6 {
		addrs = append(addrs, v6v...)
	}

	if len(addrs) == 0 && !All {
		return false
	}

	if Sh {
		s := strings.Join(addrs, " ")
		nm := ii.Name
		fmt.Printf("IPADDR_%s='%s'\n", nm, s)
		if HW && len(ii.HardwareAddr) > 0 {
			fmt.Printf("MACADDR_%s='%s'\n", nm, ii.HardwareAddr)
		}
		return true
	}

	fmt.Printf("%s: %s", ii.Name, strings.Join(addrs, ", "))
	if HW {
		fmt.Printf(" [%s]", ii.HardwareAddr)
	}
	fmt.Printf("\n")
	return true
}

// die with error
func die(f string, v ...interface{}) {
	warn(f, v...)
	os.Exit(1)
}

func warn(f string, v ...interface{}) {
	z := fmt.Sprintf("%s: %s", os.Args[0], f)
	s := fmt.Sprintf(z, v...)
	if n := len(s); s[n-1] != '\n' {
		s += "\n"
	}

	os.Stderr.WriteString(s)
	os.Stderr.Sync()
}
