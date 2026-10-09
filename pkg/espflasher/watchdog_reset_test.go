package espflasher

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var watchdogResetChips = []struct {
	name string
	chip *chipDef
	r    rtcWDTReset
}{
	{"esp32s2", defESP32S2, esp32s2WatchdogReset},
	{"esp32s3", defESP32S3, esp32s3WatchdogReset},
	{"esp32c3", defESP32C3, esp32c3WatchdogReset},
}

type regWrite struct {
	addr, value uint32
}

func TestWatchdogResetSequence(t *testing.T) {
	for _, tt := range watchdogResetChips {
		t.Run(tt.name, func(t *testing.T) {
			var writes []regWrite
			mc := &mockConnection{
				writeRegFunc: func(addr, value, mask, delayUS uint32) error {
					writes = append(writes, regWrite{addr, value})
					return nil
				},
			}
			f := &Flasher{conn: mc, opts: &FlasherOptions{}, usesUSB: true}

			require.True(t, tt.r.hardReset(f))
			assert.Equal(t, []regWrite{
				{tt.r.wdtWProtect, rtcWDTWKey},
				{tt.r.wdtConfig1, rtcWDTConfig1ResetTicks},
				{tt.r.wdtConfig0, rtcWDTConfig0ResetValue},
				{tt.r.wdtWProtect, 0},
			}, writes)
		})
	}
}

func TestWatchdogResetFallsBack(t *testing.T) {
	readErr := errors.New("register not readable")
	for _, tt := range watchdogResetChips {
		for _, c := range []struct {
			name    string
			usesUSB bool
			read    func(r rtcWDTReset, addr uint32) (uint32, error)
			write   func(r rtcWDTReset, addr uint32) error
		}{
			{name: "not USB", usesUSB: false},
			{name: "boot pin high", usesUSB: true, read: func(r rtcWDTReset, addr uint32) (uint32, error) {
				if addr == r.strapReg {
					return gpioStrapSPIBootMask, nil
				}
				return 0, nil
			}},
			{name: "force download set", usesUSB: true, read: func(r rtcWDTReset, addr uint32) (uint32, error) {
				if addr == r.option1Reg {
					return r.forceDownloadMask, nil
				}
				return 0, nil
			}},
			{name: "strap unreadable", usesUSB: true, read: func(r rtcWDTReset, addr uint32) (uint32, error) {
				if addr == r.strapReg {
					return 0, readErr
				}
				return 0, nil
			}},
			{name: "option1 unreadable", usesUSB: true, read: func(r rtcWDTReset, addr uint32) (uint32, error) {
				if addr == r.option1Reg {
					return 0, readErr
				}
				return 0, nil
			}},
			{name: "write fails", usesUSB: true, write: func(r rtcWDTReset, addr uint32) error {
				if addr == r.wdtConfig0 {
					return errors.New("write failed")
				}
				return nil
			}},
		} {
			t.Run(tt.name+"/"+c.name, func(t *testing.T) {
				mc := &mockConnection{
					readRegFunc: func(addr uint32) (uint32, error) {
						if c.read == nil {
							return 0, nil
						}
						return c.read(tt.r, addr)
					},
					writeRegFunc: func(addr, value, mask, delayUS uint32) error {
						if c.write == nil {
							t.Fatalf("unexpected write to 0x%08X", addr)
						}
						return c.write(tt.r, addr)
					},
				}
				f := &Flasher{conn: mc, opts: &FlasherOptions{}, usesUSB: c.usesUSB}

				assert.False(t, tt.r.hardReset(f))
			})
		}
	}
}

func TestFlasherResetWatchdog(t *testing.T) {
	for _, tt := range watchdogResetChips {
		t.Run(tt.name, func(t *testing.T) {
			port := &recordingPort{}
			f := &Flasher{
				conn:    &mockConnection{},
				port:    port,
				opts:    &FlasherOptions{},
				chip:    tt.chip,
				usesUSB: true,
			}

			f.Reset()

			assert.Empty(t, port.calls, "watchdog reset must not toggle DTR/RTS")
		})
	}
}

func TestFlasherResetWatchdogFallsBackToDTRRTS(t *testing.T) {
	for _, tt := range watchdogResetChips {
		t.Run(tt.name, func(t *testing.T) {
			port := &recordingPort{}
			mc := &mockConnection{
				readRegFunc: func(addr uint32) (uint32, error) {
					if addr == tt.r.strapReg {
						return gpioStrapSPIBootMask, nil
					}
					return 0, nil
				},
			}
			f := &Flasher{
				conn:    mc,
				port:    port,
				opts:    &FlasherOptions{},
				chip:    tt.chip,
				usesUSB: true,
			}

			f.Reset()

			assert.NotEmpty(t, port.calls, "boot pin high must use the DTR/RTS reset")
		})
	}
}
