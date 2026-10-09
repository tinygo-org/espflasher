package espflasher

import (
	"fmt"
	"net"
)

// ESP32-C2 register addresses used for MAC/revision/feature decoding.
// Reference: esptool/targets/esp32c2.py
//
// Unlike C3/C5/C6/H2, the C2's MAC/revision eFuse words live in BLOCK2
// (offset +0x040 from EFUSE_BASE), not BLOCK1 (+0x044).
const (
	esp32c2EfuseBlock2Word0 uint32 = 0x60008840 // MAC_EFUSE_REG (num_word 0)
	esp32c2EfuseBlock2Word1 uint32 = 0x60008844 // num_word 1
	esp32c2EfuseBlock2Word7 uint32 = 0x6000885C // num_word 7
)

// ESP32-C2 (ESP8684) target definition.
// Reference: https://github.com/espressif/esptool/blob/master/esptool/targets/esp32c2.py

var defESP32C2 = &chipDef{
	ChipType:            ChipESP32C2,
	Name:                "ESP32-C2",
	ImageChipID:         12,
	UsesMagicValue:      false, // Uses chip ID
	FallbackMagicValues: []uint32{0x6F51306F, 0x7C41A06F, 0x0C21E06F},

	SPIRegBase:  0x60002000,
	SPIUSROffs:  0x18,
	SPIUSR1Offs: 0x1C,
	SPIUSR2Offs: 0x20,
	SPIMOSIOffs: 0x24,
	SPIMISOOffs: 0x98,
	SPIW0Offs:   0x58,

	SPIMISODLenOffs: 0x28,
	SPIMOSIDLenOffs: 0x24,

	SPIAddrRegMSB: true,

	UARTDateReg: 0x60000078,
	UARTClkDiv:  0x60000014,
	XTALClkDiv:  1,

	BootloaderFlashOffset: 0x0,

	SupportsEncryptedFlash: true,
	ROMHasCompressedFlash:  true,
	ROMHasChangeBaud:       true,

	FlashFrequency: map[string]byte{
		"60m": 0xF,
		"30m": 0x0,
		"20m": 0x1,
		"15m": 0x2,
	},

	FlashSizes: defaultFlashSizes(),

	ReadMAC:          esp32c2ReadMAC,
	ReadChipRevision: esp32c2ReadChipRevision,
	ReadChipFeatures: esp32c2ReadChipFeatures,
}

// esp32c2ReadMAC reads the factory-programmed base MAC from eFuse.
// Reference: esptool/targets/esp32c2.py read_mac().
func esp32c2ReadMAC(f *Flasher) (net.HardwareAddr, error) {
	word0, err := f.ReadRegister(esp32c2EfuseBlock2Word0)
	if err != nil {
		return nil, err
	}
	word1, err := f.ReadRegister(esp32c2EfuseBlock2Word0 + 4)
	if err != nil {
		return nil, err
	}
	return decodeEfuseMAC(word0, word1), nil
}

// esp32c2ReadChipRevision reads the eFuse-encoded silicon revision.
// Reference: esptool/targets/esp32c2.py get_major_chip_version()/
// get_minor_chip_version().
func esp32c2ReadChipRevision(f *Flasher) (ChipRevision, error) {
	word1, err := f.ReadRegister(esp32c2EfuseBlock2Word1)
	if err != nil {
		return ChipRevision{}, err
	}
	major := (word1 >> 20) & 0x3
	minor := (word1 >> 16) & 0xF
	return ChipRevision{Major: int(major), Minor: int(minor)}, nil
}

// esp32c2ReadChipFeatures returns the chip feature list.
// Reference: esptool/targets/esp32c2.py get_chip_features().
func esp32c2ReadChipFeatures(f *Flasher) ([]string, error) {
	word7, err := f.ReadRegister(esp32c2EfuseBlock2Word7)
	if err != nil {
		return nil, err
	}

	features := []string{"Wi-Fi", "BT 5 (LE)", "Single Core", "120MHz"}

	flashCap := (word7 >> 29) & 0x7
	flash, ok := map[uint32]string{
		1: "Embedded Flash 4MB",
		2: "Embedded Flash 2MB",
		3: "Embedded Flash 1MB",
	}[flashCap]
	if !ok && flashCap != 0 {
		flash = "Unknown Embedded Flash"
	}
	if flash != "" {
		vendorID := (word7 >> 24) & 0x7
		vendor := map[uint32]string{
			1: "XMC",
			2: "GD",
			3: "FM",
			4: "TT",
			5: "ZBIT",
		}[vendorID]
		features = append(features, fmt.Sprintf("%s (%s)", flash, vendor))
	}

	return features, nil
}
