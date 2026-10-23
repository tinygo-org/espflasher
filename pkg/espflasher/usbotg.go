package espflasher

import (
	"strconv"
	"time"

	"go.bug.st/serial"
)

// touchUSBOTG sends a 1200 baud touch to a program that runs USB-OTG CDC
// with the Espressif VID, so it restarts into download mode on USB-Serial/JTAG.
// It reports whether the flasher now uses that USB-Serial/JTAG port.
func (f *Flasher) touchUSBOTG() bool {
	vid, pid, ok := portVIDPID(f.portStr)
	if !ok || vid != espressifUSBVID || pid == usbSerialJTAGPID {
		return false
	}
	// The ROM's own USB-OTG port uses the image chip ID as PID, as matched
	// by esptool uses_usb_otg(). It is already in download mode.
	if n, err := strconv.ParseUint(pid, 16, 16); err != nil || n < 0x100 {
		return false
	}
	f.logf("USB-OTG port detected, sending 1200 baud touch")

	// Ioctls on a cdc_acm fd can block once the device disconnects.
	port := f.port
	done := make(chan struct{})
	go func() {
		port.SetMode(&serial.Mode{BaudRate: 1200}) //nolint:errcheck
		port.SetDTR(false)                         //nolint:errcheck
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}
	port.Close() //nolint:errcheck

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)
		if name, ok := findUSBJTAGPort(f.portStr); ok {
			f.portStr = name
			return f.reopenPort() == nil
		}
	}
	f.logf("No USB-JTAG/Serial port after 1200 baud touch")
	f.reopenPort() //nolint:errcheck
	return false
}
