//go:build !darwin || cgo

package espflasher

import (
	"strings"

	"go.bug.st/serial/enumerator"
)

// portVIDPID looks up a port's USB VID/PID by name. A variable so tests can
// stub enumeration. Excluded on macOS without cgo, where enumerator needs
// IOKit; see usbjtag_nocgo.go.
var portVIDPID = enumeratePortVIDPID

// enumeratePortVIDPID returns the upper-case VID/PID of the named port. ok
// is false if it isn't found, isn't USB, or reports no VID/PID.
func enumeratePortVIDPID(name string) (vid, pid string, ok bool) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", "", false
	}
	for _, p := range ports {
		if !p.IsUSB || !samePortName(p.Name, name) {
			continue
		}
		if p.VID == "" || p.PID == "" {
			return "", "", false
		}
		return strings.ToUpper(p.VID), strings.ToUpper(p.PID), true
	}
	return "", "", false
}

// findUSBJTAGPort returns the USB-Serial/JTAG port, preferring the given
// name. ok is false if there is no such port or more than one to choose from.
var findUSBJTAGPort = func(prefer string) (name string, ok bool) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", false
	}
	var found []string
	for _, p := range ports {
		if !p.IsUSB || strings.ToUpper(p.VID) != espressifUSBVID || strings.ToUpper(p.PID) != usbSerialJTAGPID {
			continue
		}
		if samePortName(p.Name, prefer) {
			return p.Name, true
		}
		found = append(found, p.Name)
	}
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}
