//go:build !darwin || cgo

package espflasher

import (
	"go.bug.st/serial/enumerator"
)

// portVIDPID looks up a port's USB VID/PID by name. A variable so tests can
// stub enumeration. Excluded on macOS without cgo, where enumerator needs
// IOKit; see usbjtag_nocgo.go.
var portVIDPID = func(name string) (vid, pid string, ok bool) {
	ports, err := enumerateUSBPorts()
	if err != nil {
		return "", "", false
	}
	return lookupVIDPID(ports, name)
}

// findUSBJTAGPort returns the USB-Serial/JTAG port, preferring the given
// name. ok is false if there is no such port or more than one to choose from.
var findUSBJTAGPort = func(prefer string) (name string, ok bool) {
	ports, err := enumerateUSBPorts()
	if err != nil {
		return "", false
	}
	return lookupUSBJTAGPort(ports, prefer)
}

func enumerateUSBPorts() ([]USBPort, error) {
	list, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	var ports []USBPort
	for _, p := range list {
		if p.IsUSB {
			ports = append(ports, USBPort{Name: p.Name, VID: p.VID, PID: p.PID})
		}
	}
	return ports, nil
}
