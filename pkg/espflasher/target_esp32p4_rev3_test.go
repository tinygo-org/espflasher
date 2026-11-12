package espflasher

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestESP32P4Rev3RegisterAddress pins the rev3-specific UARTDEV_BUF_NO and
// confirms it differs from the rev1 address (the only register that moved in
// production silicon).
func TestESP32P4Rev3RegisterAddress(t *testing.T) {
	assert.Equal(t, uint32(0x4FFBFEC8), esp32p4Rev3UARTDevBufNo,
		"rev3 UARTDEV_BUF_NO should be 0x4FFBFEC8")
	assert.NotEqual(t, esp32p4Rev3UARTDevBufNo, esp32p4Rev1UARTDevBufNo,
		"rev3 UARTDEV_BUF_NO must differ from rev1")
}

// TestESP32P4Rev3SkipStub documents that P4 must flash in ROM mode: the RAM
// stub loads but stops answering, so the target opts out of the stub.
func TestESP32P4Rev3SkipStub(t *testing.T) {
	assert.True(t, defESP32P4Rev3.SkipStub, "P4 rev3 must set SkipStub (ROM-only flash)")
}

func TestESP32P4Rev3PostConnectUSBJTAG(t *testing.T) {
	writeCount := 0
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == esp32p4Rev3UARTDevBufNo {
				return esp32p4UARTDevBufNoUSBJTAGSerial, nil
			}
			return 0, nil // SWD conf read
		},
		writeRegFunc: func(addr, value, mask, delayUS uint32) error {
			writeCount++
			return nil
		},
	}
	f := &Flasher{conn: mc, opts: &FlasherOptions{}}

	err := esp32p4Rev3PostConnect(f)
	require.NoError(t, err)
	assert.True(t, f.usesUSB, "usesUSB should be true for USB-JTAG/Serial")
	assert.Greater(t, writeCount, 0, "should have written registers to disable watchdog")
}

func TestESP32P4Rev3PostConnectUART(t *testing.T) {
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			return 0, nil // not the USB-JTAG sentinel
		},
	}
	f := &Flasher{conn: mc, opts: &FlasherOptions{}}

	err := esp32p4Rev3PostConnect(f)
	require.NoError(t, err)
	assert.False(t, f.usesUSB, "usesUSB should be false for a UART bridge")
}

// TestESP32P4Rev3PostConnectSecureMode checks that an unreadable register
// (e.g. Secure Download Mode) degrades gracefully to non-USB instead of
// returning an error.
func TestESP32P4Rev3PostConnectSecureMode(t *testing.T) {
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			return 0, errors.New("register not readable")
		},
	}
	f := &Flasher{conn: mc, opts: &FlasherOptions{}}

	err := esp32p4Rev3PostConnect(f)
	require.NoError(t, err)
	assert.False(t, f.usesUSB, "should fall back to non-USB on read error")
}

func TestESP32P4Rev3ReadMAC(t *testing.T) {
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			switch addr {
			case esp32p4EfuseBlock1Word0:
				return 0x60559ff7, nil
			case esp32p4EfuseBlock1Word0 + 4:
				return 0x00002ca2, nil
			}
			return 0, nil
		},
	}
	f := &Flasher{conn: mc, chip: defESP32P4Rev3}

	mac, err := f.MAC()
	require.NoError(t, err)
	assert.Equal(t, "2c:a2:60:55:9f:f7", mac.String())
}

func TestESP32P4Rev3ChipRevision(t *testing.T) {
	tests := []struct {
		name  string
		word2 uint32
		want  ChipRevision
	}{
		{"v0.0", 0x00000000, ChipRevision{0, 0}},
		{"v1.1", (1 << 4) | 1, ChipRevision{1, 1}},
		{"v5.15 (major bit2 set)", (1 << 23) | 1<<4 | 0xF, ChipRevision{5, 15}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &mockConnection{
				readRegFunc: func(addr uint32) (uint32, error) {
					if addr == esp32p4EfuseBlock1Word2 {
						return tt.word2, nil
					}
					return 0, nil
				},
			}
			f := &Flasher{conn: mc, chip: defESP32P4Rev3}
			got, err := f.ChipRevision()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestESP32P4Rev3ChipFeatures(t *testing.T) {
	mc := &mockConnection{}
	f := &Flasher{conn: mc, chip: defESP32P4Rev3}
	got, err := f.ChipFeatures()
	require.NoError(t, err)
	assert.Equal(t, []string{"Dual Core + LP Core", "400MHz"}, got)
}

// TestESP32P4Rev3ReadErrorsPropagate ensures a read failure while decoding MAC
// or revision surfaces as an error rather than being swallowed.
func TestESP32P4Rev3ReadErrorsPropagate(t *testing.T) {
	newF := func(readReg func(addr uint32) (uint32, error)) *Flasher {
		return &Flasher{conn: &mockConnection{readRegFunc: readReg}, chip: defESP32P4Rev3}
	}
	assertRegisterErrorsPropagate(t, newF, []uint32{esp32p4EfuseBlock1Word0, esp32p4EfuseBlock1Word0 + 4},
		func(f *Flasher) error { _, err := f.MAC(); return err })
	assertRegisterErrorsPropagate(t, newF, []uint32{esp32p4EfuseBlock1Word2},
		func(f *Flasher) error { _, err := f.ChipRevision(); return err })
}
