package espflasher

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestESP32C3ChipRevisionRev0(t *testing.T) {
	// Efuse value with major=0, minor=0
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == esp32c3EfuseRdMacSpiSys1 {
				return 0x00000000, nil
			}
			return 0, nil
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	rev, err := esp32c3ChipRevision(f)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), rev, "revision should be 0 for major=0, minor=0")
}

func TestESP32C3ChipRevisionRev101(t *testing.T) {
	// Efuse value with major=1, minor=1
	// major=1: bits 24:22 = (1 << 22) = 0x00400000
	// minor=1: bits 21:20 = (1 << 20) = 0x00100000
	// Combined: 0x00500000
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == esp32c3EfuseRdMacSpiSys1 {
				// major=1: (1 << 22) = 0x00400000
				// minor=1: (1 << 20) = 0x00100000
				// combined: 0x00500000
				return 0x00500000, nil
			}
			return 0, nil
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	rev, err := esp32c3ChipRevision(f)
	require.NoError(t, err)
	assert.Equal(t, uint32(101), rev, "revision should be 101 for major=1, minor=1")
}

func TestESP32C3UARTDevAddrRev0(t *testing.T) {
	// Return major=0, minor=0 from efuse
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			return 0x00000000, nil
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	addr := esp32c3UARTDevAddr(f)
	assert.Equal(t, esp32c3UARTDevBufNoRev0, addr, "should use Rev0 address")
}

func TestESP32C3UARTDevAddrRev101(t *testing.T) {
	// Return major=1, minor=1 from efuse (revision 101)
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == esp32c3EfuseRdMacSpiSys1 {
				// major=1: (1 << 22) = 0x00400000
				// minor=1: (1 << 20) = 0x00100000
				// combined: 0x00500000
				return 0x00500000, nil
			}
			return 0, nil
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	addr := esp32c3UARTDevAddr(f)
	assert.Equal(t, esp32c3UARTDevBufNoRev101, addr, "should use Rev101 address")
}

func TestESP32C3UARTDevAddrReadError(t *testing.T) {
	// Efuse read fails, should default to Rev0
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			return 0, errors.New("efuse read error")
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	addr := esp32c3UARTDevAddr(f)
	assert.Equal(t, esp32c3UARTDevBufNoRev0, addr, "should default to Rev0 on read error")
}

func TestESP32C3PostConnectUSBJTAGRev0(t *testing.T) {
	writeCount := 0
	readCount := 0

	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			readCount++
			if addr == esp32c3EfuseRdMacSpiSys1 {
				return 0x00000000, nil // Major=0, Minor=0 -> Rev0
			}
			if addr == esp32c3UARTDevBufNoRev0 {
				return esp32c3UARTDevBufNoUSBJTAGSerial, nil
			}
			// Return 0 for SWD conf read
			return 0, nil
		},
		writeRegFunc: func(addr, value, mask, delayUS uint32) error {
			writeCount++
			return nil
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	err := esp32c3PostConnect(f)
	require.NoError(t, err)
	assert.True(t, f.usesUSB, "usesUSB should be true for USB-JTAG/Serial")
	assert.Greater(t, writeCount, 0, "should have written registers to disable watchdog")
}

func TestESP32C3PostConnectUSBJTAGRev101(t *testing.T) {
	writeCount := 0

	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == esp32c3EfuseRdMacSpiSys1 {
				// major=1, minor=1: (1 << 22) | (1 << 20) = 0x00500000
				return 0x00500000, nil // Major=1, Minor=1 -> Rev101
			}
			if addr == esp32c3UARTDevBufNoRev101 {
				return esp32c3UARTDevBufNoUSBJTAGSerial, nil
			}
			// Return 0 for SWD conf read
			return 0, nil
		},
		writeRegFunc: func(addr, value, mask, delayUS uint32) error {
			writeCount++
			return nil
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	err := esp32c3PostConnect(f)
	require.NoError(t, err)
	assert.True(t, f.usesUSB, "usesUSB should be true for USB-JTAG/Serial")
	assert.Greater(t, writeCount, 0, "should have written registers to disable watchdog")
}

func TestESP32C3PostConnectUART(t *testing.T) {
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == esp32c3EfuseRdMacSpiSys1 {
				return 0x00000000, nil
			}
			return 0, nil // Not USB, return UART value
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	err := esp32c3PostConnect(f)
	require.NoError(t, err)
	assert.False(t, f.usesUSB, "usesUSB should be false for UART")
}

func TestESP32C3PostConnectSecureMode(t *testing.T) {
	// Simulate read error (secure download mode)
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			return 0, errors.New("register not readable") // Simulate unreadable register
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	err := esp32c3PostConnect(f)
	require.NoError(t, err, "should gracefully handle read error")
	assert.False(t, f.usesUSB, "should default to non-USB on read error")
}
