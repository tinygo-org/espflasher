//go:build js && wasm

package webusb

import (
	"fmt"
	"syscall/js"

	"go.bug.st/serial"
	"tinygo.org/x/espflasher/pkg/espflasher"
)

// Filters returns a JS array of filters for navigator.usb.requestDevice
// that matches all supported devices.
func Filters() js.Value {
	arr := js.Global().Get("Array").New()
	for _, id := range filterIDs() {
		f := map[string]any{"vendorId": int(id[0])}
		if id[1] != 0 {
			f["productId"] = int(id[1])
		}
		arr.Call("push", f)
	}
	return arr
}

// PortName returns the name used for dev in espflasher.
func PortName(dev js.Value) string {
	serialNumber := ""
	if s := dev.Get("serialNumber"); s.Type() == js.TypeString {
		serialNumber = s.String()
	}
	return portName(uint16(dev.Get("vendorId").Int()), uint16(dev.Get("productId").Int()), serialNumber)
}

func devices() ([]js.Value, error) {
	u, err := usb()
	if err != nil {
		return nil, err
	}
	list, err := await(u.Call("getDevices"))
	if err != nil {
		return nil, err
	}
	out := make([]js.Value, list.Length())
	for i := range out {
		out[i] = list.Index(i)
	}
	return out, nil
}

// OpenPort opens the already permitted device with the given PortName.
// Use it as espflasher.FlasherOptions.SerialOpener so the flasher can
// reopen the device after it re-enumerates.
func OpenPort(name string, mode *serial.Mode) (serial.Port, error) {
	devs, err := devices()
	if err != nil {
		return nil, err
	}
	for _, d := range devs {
		if PortName(d) == name {
			return Open(d, mode)
		}
	}
	return nil, fmt.Errorf("webusb: device %s not found", name)
}

// ListUSBPorts lists permitted devices. Use it as
// espflasher.FlasherOptions.ListUSBPorts.
func ListUSBPorts() ([]espflasher.USBPort, error) {
	devs, err := devices()
	if err != nil {
		return nil, err
	}
	ports := make([]espflasher.USBPort, len(devs))
	for i, d := range devs {
		ports[i] = espflasher.USBPort{
			Name: PortName(d),
			VID:  fmt.Sprintf("%04X", d.Get("vendorId").Int()),
			PID:  fmt.Sprintf("%04X", d.Get("productId").Int()),
		}
	}
	return ports, nil
}

// Options returns espflasher.DefaultOptions wired for WebUSB.
func Options() *espflasher.FlasherOptions {
	opts := espflasher.DefaultOptions()
	opts.SerialOpener = OpenPort
	opts.ListUSBPorts = ListUSBPorts
	return opts
}
