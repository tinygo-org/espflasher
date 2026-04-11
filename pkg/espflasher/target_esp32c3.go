package espflasher

import "fmt"

// ESP32-C3 register addresses for USB interface detection and watchdog control.
// Reference: esptool/targets/esp32c3.py
const (
	// UARTDEV_BUF_NO address depends on chip revision (read from efuse).
	// Revision < 1 (ECO0-ECO3): base 0x3FCDF064
	// Revision >= 1 (ECO4+): base 0x3FCDF060
	// The actual register is 24 bytes (+0x18) from the base address.
	esp32c3UARTDevBufNoRev0         uint32 = 0x3FCDF07C // 0x3FCDF064 + 24
	esp32c3UARTDevBufNoRev101       uint32 = 0x3FCDF078 // 0x3FCDF060 + 24
	esp32c3UARTDevBufNoUSBJTAGSerial uint32 = 3          // USB-JTAG/Serial active

	// Efuse register for chip version detection.
	// Major chip version is in bits 24:22; minor in bits 21:20.
	esp32c3EfuseRdMacSpiSys1 uint32 = 0x60008844

	// RTC_CNTL watchdog registers (different offsets from S3).
	esp32c3RTCCntlWDTConfig0  uint32 = 0x60008090
	esp32c3RTCCntlWDTWProtect uint32 = 0x600080A8
	esp32c3RTCCntlWDTWKey     uint32 = 0x50D83AA1

	// Super Watchdog (SWD) registers.
	esp32c3RTCCntlSWDConf       uint32 = 0x600080AC
	esp32c3RTCCntlSWDAutoFeedEn uint32 = 1 << 31
	esp32c3RTCCntlSWDWProtect   uint32 = 0x600080B0
	esp32c3RTCCntlSWDWKey       uint32 = 0x8F1D312A
)

// ESP32-C3 target definition.
// Reference: https://github.com/espressif/esptool/blob/master/esptool/targets/esp32c3.py

var defESP32C3 = &chipDef{
	ChipType:       ChipESP32C3,
	Name:           "ESP32-C3",
	ImageChipID:    5,
	UsesMagicValue: false, // Uses chip ID

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
		"80m": 0xF,
		"40m": 0x0,
		"20m": 0x2,
	},

	FlashSizes: defaultFlashSizes(),

	PostConnect: esp32c3PostConnect,
}

// esp32c3ChipRevision reads the chip revision from efuse.
// Returns major*100 + minor (e.g., 101 = major 1, minor 0, sub 1).
// Chip revision is encoded in EFUSE_RD_MAC_SPI_SYS_1_REG:
//   - Major version: bits 24:22
//   - Minor version: bits 21:20
func esp32c3ChipRevision(f *Flasher) (uint32, error) {
	val, err := f.ReadRegister(esp32c3EfuseRdMacSpiSys1)
	if err != nil {
		return 0, err
	}
	major := (val >> 22) & 0x7
	minor := (val >> 20) & 0x3
	return major*100 + minor, nil
}

// esp32c3UARTDevAddr returns the correct UARTDEV_BUF_NO address based on chip revision.
func esp32c3UARTDevAddr(f *Flasher) uint32 {
	rev, err := esp32c3ChipRevision(f)
	if err != nil {
		// Default to older revision if efuse read fails
		return esp32c3UARTDevBufNoRev0
	}
	if rev >= 101 {
		return esp32c3UARTDevBufNoRev101
	}
	return esp32c3UARTDevBufNoRev0
}

// esp32c3PostConnect detects the USB interface type and disables watchdogs
// when connected via USB-JTAG/Serial. Without this, the RTC WDT fires
// during flash and resets the chip mid-operation.
// Reference: esptool/targets/esp32c3.py _post_connect()
func esp32c3PostConnect(f *Flasher) error {
	addr := esp32c3UARTDevAddr(f)
	val, err := f.ReadRegister(addr)
	if err != nil {
		// In secure download mode, the register may be unreadable.
		// Default to non-USB behavior (safe fallback).
		return nil
	}

	if val == esp32c3UARTDevBufNoUSBJTAGSerial {
		f.usesUSB = true
		f.logf("USB-JTAG/Serial interface detected, disabling watchdogs")
		if err := disableWatchdogsESP32C3(f); err != nil {
			return err
		}
	}

	return nil
}

// disableWatchdogsESP32C3 disables the RTC WDT and enables SWD auto-feed.
// This prevents the watchdog from resetting the chip during flash operations
// when connected via USB-JTAG/Serial.
func disableWatchdogsESP32C3(f *Flasher) error {
	// Unlock and disable RTC WDT
	if err := f.WriteRegister(esp32c3RTCCntlWDTWProtect, esp32c3RTCCntlWDTWKey); err != nil {
		return fmt.Errorf("unlock RTC WDT: %w", err)
	}
	if err := f.WriteRegister(esp32c3RTCCntlWDTConfig0, 0); err != nil {
		return fmt.Errorf("disable RTC WDT: %w", err)
	}
	if err := f.WriteRegister(esp32c3RTCCntlWDTWProtect, 0); err != nil {
		return fmt.Errorf("lock RTC WDT: %w", err)
	}

	// Unlock SWD and enable auto-feed
	if err := f.WriteRegister(esp32c3RTCCntlSWDWProtect, esp32c3RTCCntlSWDWKey); err != nil {
		return fmt.Errorf("unlock SWD: %w", err)
	}

	swdConf, err := f.ReadRegister(esp32c3RTCCntlSWDConf)
	if err != nil {
		return fmt.Errorf("read SWD conf: %w", err)
	}
	swdConf |= esp32c3RTCCntlSWDAutoFeedEn
	if err := f.WriteRegister(esp32c3RTCCntlSWDConf, swdConf); err != nil {
		return fmt.Errorf("enable SWD auto-feed: %w", err)
	}
	if err := f.WriteRegister(esp32c3RTCCntlSWDWProtect, 0); err != nil {
		return fmt.Errorf("lock SWD: %w", err)
	}

	return nil
}
