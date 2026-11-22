package espflasher

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListUSBPortsOption(t *testing.T) {
	stubPortVIDPID(t, "", "", false)
	stubFindUSBJTAGPort(t, "", false)
	ports := []USBPort{
		{Name: "webusb:10c4:ea60:1", VID: "10c4", PID: "ea60"},
		{Name: "webusb:303a:1001:2", VID: "303a", PID: "1001"},
	}
	f := &Flasher{
		chip:    defESP32C3,
		portStr: "webusb:303a:1001:2",
		opts:    &FlasherOptions{ListUSBPorts: func() ([]USBPort, error) { return ports, nil }},
	}
	assert.Equal(t, usbInterfaceSerialJTAG, f.usbInterfaceFromPort())

	name, ok := f.findUSBJTAGPort("other")
	assert.True(t, ok)
	assert.Equal(t, "webusb:303a:1001:2", name)

	f.portStr = "webusb:10c4:ea60:1"
	assert.Equal(t, usbInterfaceUnknown, f.usbInterfaceFromPort())
}

func TestListUSBPortsOptionError(t *testing.T) {
	f := &Flasher{
		portStr: "x",
		opts: &FlasherOptions{ListUSBPorts: func() ([]USBPort, error) {
			return nil, errors.New("no usb")
		}},
	}
	_, _, ok := f.portVIDPID("x")
	assert.False(t, ok)
	_, ok = f.findUSBJTAGPort("x")
	assert.False(t, ok)
}

func TestLookupUSBJTAGPortAmbiguous(t *testing.T) {
	ports := []USBPort{
		{Name: "a", VID: "303A", PID: "1001"},
		{Name: "b", VID: "303A", PID: "1001"},
	}
	_, ok := lookupUSBJTAGPort(ports, "c")
	assert.False(t, ok)
	name, ok := lookupUSBJTAGPort(ports, "b")
	assert.True(t, ok)
	assert.Equal(t, "b", name)
}

type atomicLinesPort struct {
	mockPort
	calls [][2]bool
}

func (p *atomicLinesPort) SetDTRAndRTS(dtr, rts bool) error {
	p.calls = append(p.calls, [2]bool{dtr, rts})
	return nil
}

func TestSetDTRandRTSUsesAtomicSetter(t *testing.T) {
	p := &atomicLinesPort{}
	assert.NoError(t, setDTRandRTS(p, true, false))
	assert.Equal(t, [][2]bool{{true, false}}, p.calls)
}
