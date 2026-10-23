package espflasher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.bug.st/serial"
)

type touchPort struct {
	mockPort
	baud   int
	dtrOff bool
}

func (p *touchPort) SetMode(mode *serial.Mode) error {
	p.baud = mode.BaudRate
	return nil
}

func (p *touchPort) SetDTR(dtr bool) error {
	p.dtrOff = !dtr
	return nil
}

func stubFindUSBJTAGPort(t *testing.T, name string, ok bool) {
	t.Helper()
	prev := findUSBJTAGPort
	findUSBJTAGPort = func(string) (string, bool) { return name, ok }
	t.Cleanup(func() { findUSBJTAGPort = prev })
}

func TestTouchUSBOTGSkipsOtherPorts(t *testing.T) {
	tests := []struct {
		name     string
		vid, pid string
	}{
		{"usb-serial-jtag", "303A", "1001"},
		{"rom usb-otg s3", "303A", "0009"},
		{"usb-uart bridge", "10C4", "EA60"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubPortVIDPID(t, tt.vid, tt.pid, true)
			p := &touchPort{}
			f := &Flasher{port: p, opts: &FlasherOptions{}, portStr: "/dev/ttyACM0"}
			assert.False(t, f.touchUSBOTG())
			assert.Equal(t, 0, p.baud)
			assert.Equal(t, 0, p.closeCalls)
		})
	}
}

func TestTouchUSBOTGSwitchesToUSBJTAGPort(t *testing.T) {
	stubPortVIDPID(t, "303A", "4001", true)
	stubFindUSBJTAGPort(t, "/dev/ttyACM1", true)
	app := &touchPort{}
	rom := &mockPort{}
	var opened string
	opts := &FlasherOptions{
		Logger: &StdoutLogger{W: nopWriter{}},
		SerialOpener: func(name string, mode *serial.Mode) (serial.Port, error) {
			opened = name
			return rom, nil
		},
	}
	f := &Flasher{port: app, conn: newConn(app), opts: opts, portStr: "/dev/ttyACM0"}

	assert.True(t, f.touchUSBOTG())
	assert.Equal(t, 1200, app.baud)
	assert.True(t, app.dtrOff)
	assert.Equal(t, "/dev/ttyACM1", f.portStr)
	assert.Equal(t, "/dev/ttyACM1", opened)
	assert.Same(t, serial.Port(rom), f.port)
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
