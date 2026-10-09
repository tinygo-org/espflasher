package espflasher

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestESP32S2PostConnectUSBOTG(t *testing.T) {
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == esp32s2UARTDevBufNo {
				return esp32s2UARTDevBufNoUSBOTG, nil
			}
			return 0, nil
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	err := esp32s2PostConnect(f)
	require.NoError(t, err)
	assert.True(t, f.usesUSB, "usesUSB should be true for USB-OTG")
}

func TestESP32S2PostConnectUART(t *testing.T) {
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			return 0, nil // Not USB, return UART value
		},
	}
	f := &Flasher{
		conn: mc,
		opts: &FlasherOptions{},
	}

	err := esp32s2PostConnect(f)
	require.NoError(t, err)
	assert.False(t, f.usesUSB, "usesUSB should be false for UART")
}

func TestESP32S2PostConnectSecureMode(t *testing.T) {
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

	err := esp32s2PostConnect(f)
	require.NoError(t, err, "should gracefully handle read error")
	assert.False(t, f.usesUSB, "should default to non-USB on read error")
}

func TestESP32S2MAC(t *testing.T) {
	mc := &mockConnection{
		readRegFunc: func(addr uint32) (uint32, error) {
			switch addr {
			case esp32s2EfuseBlock1Word0:
				return 0x60559ff7, nil
			case esp32s2EfuseBlock1Word0 + 4:
				return 0x00002ca2, nil
			}
			return 0, nil
		},
	}
	f := &Flasher{conn: mc, chip: defESP32S2}

	mac, err := f.MAC()
	require.NoError(t, err)
	assert.Equal(t, "2c:a2:60:55:9f:f7", mac.String())
}

func TestESP32S2ChipRevision(t *testing.T) {
	tests := []struct {
		name  string
		word3 uint32
		word4 uint32
		want  ChipRevision
	}{
		{"v0.0", 0x00000000, 0x00000000, ChipRevision{0, 0}},
		{"v1.1", 1 << 18, 1 << 4, ChipRevision{1, 1}},
		{"max", (3 << 18) | (1 << 20), 0x7 << 4, ChipRevision{3, 15}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &mockConnection{
				readRegFunc: func(addr uint32) (uint32, error) {
					switch addr {
					case esp32s2EfuseBlock1Word3:
						return tt.word3, nil
					case esp32s2EfuseBlock1Word4:
						return tt.word4, nil
					}
					return 0, nil
				},
			}
			f := &Flasher{conn: mc, chip: defESP32S2}
			got, err := f.ChipRevision()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestESP32S2ChipFeatures(t *testing.T) {
	tests := []struct {
		name        string
		word3       uint32
		block2word4 uint32
		want        []string
	}{
		{
			"defaults",
			0, 0,
			[]string{"Wi-Fi", "Single Core", "240MHz", "No Embedded Flash", "No Embedded PSRAM", "No calibration in BLK2 of efuse"},
		},
		{
			"4MB flash, 4MB psram, V2 calib",
			(2 << 21) | (2 << 28), 2 << 4,
			[]string{"Wi-Fi", "Single Core", "240MHz", "Embedded Flash 4MB", "Embedded PSRAM 4MB", "ADC and temperature sensor calibration in BLK2 of eFuse V2"},
		},
		{
			"unknown values",
			(0xF << 21) | (0xF << 28), 0x7 << 4,
			[]string{"Wi-Fi", "Single Core", "240MHz", "Unknown Embedded Flash", "Unknown Embedded PSRAM", "Unknown calibration in BLK2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &mockConnection{
				readRegFunc: func(addr uint32) (uint32, error) {
					switch addr {
					case esp32s2EfuseBlock1Word3:
						return tt.word3, nil
					case esp32s2EfuseBlock2Word4:
						return tt.block2word4, nil
					}
					return 0, nil
				},
			}
			f := &Flasher{conn: mc, chip: defESP32S2}
			got, err := f.ChipFeatures()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestESP32S2ReadErrorsPropagate(t *testing.T) {
	newF := func(readReg func(addr uint32) (uint32, error)) *Flasher {
		return &Flasher{conn: &mockConnection{readRegFunc: readReg}, chip: defESP32S2}
	}
	assertRegisterErrorsPropagate(t, newF, []uint32{esp32s2EfuseBlock1Word0, esp32s2EfuseBlock1Word0 + 4},
		func(f *Flasher) error { _, err := f.MAC(); return err })
	assertRegisterErrorsPropagate(t, newF, []uint32{esp32s2EfuseBlock1Word3, esp32s2EfuseBlock1Word4},
		func(f *Flasher) error { _, err := f.ChipRevision(); return err })
	assertRegisterErrorsPropagate(t, newF, []uint32{esp32s2EfuseBlock1Word3, esp32s2EfuseBlock2Word4},
		func(f *Flasher) error { _, err := f.ChipFeatures(); return err })
}
