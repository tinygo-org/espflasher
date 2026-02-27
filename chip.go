package espflash

import "fmt"

// ChipType identifies the ESP chip family.
type ChipType int

const (
	ChipESP8266 ChipType = iota
	ChipESP32
	ChipESP32S2
	ChipESP32S3
	ChipESP32C2
	ChipESP32C3
	ChipESP32C6
	ChipESP32H2
	ChipAuto // Auto-detect chip type
)

// String returns the human-readable chip name.
func (c ChipType) String() string {
	switch c {
	case ChipESP8266:
		return "ESP8266"
	case ChipESP32:
		return "ESP32"
	case ChipESP32S2:
		return "ESP32-S2"
	case ChipESP32S3:
		return "ESP32-S3"
	case ChipESP32C2:
		return "ESP32-C2"
	case ChipESP32C3:
		return "ESP32-C3"
	case ChipESP32C6:
		return "ESP32-C6"
	case ChipESP32H2:
		return "ESP32-H2"
	case ChipAuto:
		return "Auto"
	default:
		return fmt.Sprintf("Unknown(%d)", int(c))
	}
}

// chipDef holds chip-specific constants and register addresses.
// These values are derived from the ESP-IDF and esptool source code.
type chipDef struct {
	// ChipType identifies this chip family.
	ChipType ChipType

	// Name is the display name (e.g. "ESP32-S3").
	Name string

	// MagicValue is read from CHIP_DETECT_MAGIC_REG_ADDR on older chips.
	// Only used for ESP8266 and ESP32 which don't support get_chip_id.
	MagicValue uint32

	// ImageChipID is returned by the GET_SECURITY_INFO command on newer chips.
	// ESP8266 and ESP32 use magic value detection instead (ImageChipID = 0).
	ImageChipID uint32

	// UsesMagicValue indicates this chip uses the magic register for detection
	// rather than the chip ID command.
	UsesMagicValue bool

	// SPI register base and offsets for flash operations.
	SPIRegBase  uint32
	SPIUSROffs  uint32
	SPIUSR1Offs uint32
	SPIUSR2Offs uint32
	SPIMOSIOffs uint32
	SPIMISOOffs uint32
	SPIW0Offs   uint32

	// SPIAddrRegMSB: if true, SPI peripheral sends from MSB of 32-bit register.
	SPIAddrRegMSB bool

	// UARTDateReg is used for clock divider reading.
	UARTDateReg uint32
	UARTClkDiv  uint32
	XTALClkDiv  uint32

	// BootloaderFlashOffset is where the bootloader image lives in flash.
	BootloaderFlashOffset uint32

	// FlashFrequency maps frequency strings to register values.
	FlashFrequency map[string]byte

	// FlashSizes maps size strings to header byte values.
	FlashSizes map[string]byte
}

// chipDetectMagicRegAddr is the register address that has a different
// value on each chip model. Used for chip auto-detection.
const chipDetectMagicRegAddr uint32 = 0x40001000

// All known chip definitions.
var chipDefs = map[ChipType]*chipDef{
	ChipESP8266: defESP8266,
	ChipESP32:   defESP32,
	ChipESP32S2: defESP32S2,
	ChipESP32S3: defESP32S3,
	ChipESP32C2: defESP32C2,
	ChipESP32C3: defESP32C3,
	ChipESP32C6: defESP32C6,
	ChipESP32H2: defESP32H2,
}

// detectChip reads the chip magic register or chip ID to identify the
// connected ESP device.
func detectChip(c *conn) (*chipDef, error) {
	// First try reading the magic register (works for all chips)
	magic, err := c.readReg(chipDetectMagicRegAddr)
	if err != nil {
		return nil, fmt.Errorf("read chip detect register: %w", err)
	}

	// Check magic value against known chips
	for _, def := range chipDefs {
		if def.UsesMagicValue && def.MagicValue == magic {
			return def, nil
		}
	}

	// For newer chips (ESP32-S3, ESP32-C3, etc.), the magic register value
	// may not match. These chips use chip ID from the security info instead.
	// However, reading chip ID requires the GET_SECURITY_INFO command which
	// may not work before we know the chip type.
	//
	// As a fallback, we can also detect by the UART date register value,
	// or by trying known magic values for newer chips.
	// Newer chips have different magic values at this register.

	// Try matching by non-magic-value chips
	for _, def := range chipDefs {
		if !def.UsesMagicValue {
			// For these chips, try to read a chip-specific register to verify
			// We can't easily do chip_id check without knowing the chip first,
			// so try to match by reading registers at known addresses.
			continue
		}
	}

	return nil, &ChipDetectError{MagicValue: magic}
}

// detectChipByMagic returns the chip definition matching the given magic value.
func detectChipByMagic(magic uint32) *chipDef {
	for _, def := range chipDefs {
		if def.UsesMagicValue && def.MagicValue == magic {
			return def
		}
	}
	return nil
}

// defaultFlashSizes returns the standard flash size map used by most chips.
func defaultFlashSizes() map[string]byte {
	return map[string]byte{
		"256KB": 0x12,
		"512KB": 0x13,
		"1MB":   0x14,
		"2MB":   0x15,
		"4MB":   0x16,
		"8MB":   0x17,
		"16MB":  0x18,
		"32MB":  0x19,
		"64MB":  0x1A,
		"128MB": 0x21,
	}
}
