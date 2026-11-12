package espflasher

// ESP32-P4 Rev3 (production silicon, chip revision >= 3.0, ECO6) register
// addresses for USB interface detection and watchdog control.
//
// These addresses are P4-wide and shared with the rev1 (ECO2) target except
// for UARTDEV_BUF_NO, which moved in production. Reference: esptool/targets/
// esp32p4.py and esptool/targets/esp32.py (ESP32ROM base-class SPI defaults P4
// inherits).
//
// UARTDEV_BUF_NO is revision-dependent in esptool:
//
//	rev <  3.0 (ECO2, ChipESP32P4Rev1):  0x4FF3FEB0 + 24 = 0x4FF3FEC8
//	rev >= 3.0 (production, this target): 0x4FFBFEB0 + 24 = 0x4FFBFEC8
//
// The USB-JTAG/Serial sentinel (6) and the LP-WDT / eFuse addresses are
// identical to the rev1 target and are referenced from there rather than
// redeclared.
const (
	// esp32p4Rev3UARTDevBufNo is the rev3-specific UARTDEV_BUF_NO address.
	// The USB-JTAG/Serial sentinel value (6) and the LP-WDT / eFuse addresses
	// are P4-wide and referenced from target_esp32p4_rev1.go.
	esp32p4Rev3UARTDevBufNo uint32 = 0x4FFBFEC8
)

// ESP32-P4 Rev3 target definition.
//
// Rev3 is production silicon; it differs from ChipESP32P4Rev1 only in the
// UARTDEV_BUF_NO address used for USB-JTAG detection and in the stub it loads
// (esp32p4.json here vs esp32p4-rev1.json for rev1). The eFuse-based MAC,
// revision, and feature readers are P4-wide and reused from the rev1 target.
//
// Reference: https://github.com/espressif/esptool/blob/master/esptool/targets/esp32p4.py
var defESP32P4Rev3 = &chipDef{
	ChipType:       ChipESP32P4Rev3,
	Name:           "ESP32-P4-Rev3",
	ImageChipID:    18,
	UsesMagicValue: false,

	SPIRegBase:  0x5008D000,
	SPIUSROffs:  0x18,
	SPIUSR1Offs: 0x1C,
	SPIUSR2Offs: 0x20,
	SPIMOSIOffs: 0x24,
	SPIMISOOffs: 0x98,
	SPIW0Offs:   0x58,

	SPIMISODLenOffs: 0x28,
	SPIMOSIDLenOffs: 0x24,

	SPIAddrRegMSB: false,

	UARTDateReg: 0x500CA08C,
	UARTClkDiv:  0x500CA014,
	XTALClkDiv:  1,

	// Bootloader flashes at 0x2000 on the P4 (not 0x0).
	BootloaderFlashOffset: 0x2000,

	SupportsEncryptedFlash: true,
	ROMHasCompressedFlash:  true,
	ROMHasChangeBaud:       true,

	// The P4 RAM stub loads (prints "OHAI") but then stops answering flash
	// commands, so P4 must flash in ROM mode. SkipStub also makes connect()
	// skip the stub even when the user did not set FlasherOptions.SkipStub.
	SkipStub: true,

	FlashFrequency: map[string]byte{
		"80m": 0xF,
		"40m": 0x0,
		"20m": 0x2,
	},

	FlashSizes: defaultFlashSizes(),

	PostConnect: esp32p4Rev3PostConnect,

	ReadMAC:          esp32p4Rev1ReadMAC,
	ReadChipRevision: esp32p4Rev1ReadChipRevision,
	ReadChipFeatures: esp32p4Rev1ReadChipFeatures,
}

// esp32p4Rev3PostConnect detects the USB interface type and disables watchdogs
// when connected via USB-JTAG/Serial. Without this, the LP watchdog fires
// during flash and resets the chip mid-operation.
// Reference: esptool/targets/esp32p4.py _post_connect().
func esp32p4Rev3PostConnect(f *Flasher) error {
	// Prefer VID/PID (as esptool does); fall back to the ROM variable when
	// the host reports none.
	usbJTAG := f.usbInterfaceFromPort() == usbInterfaceSerialJTAG
	if !usbJTAG {
		uartDev, err := f.ReadRegister(esp32p4Rev3UARTDevBufNo)
		if err != nil {
			return nil // unreadable (e.g. SDM): default to non-USB, safe fallback
		}
		usbJTAG = uartDev == esp32p4UARTDevBufNoUSBJTAGSerial
	}

	if usbJTAG {
		f.usesUSB = true
		f.logf("USB-JTAG/Serial interface detected (ESP32-P4 rev3), disabling watchdogs")
		return disableWatchdogsLP(f, esp32p4LPWDTConfig0, esp32p4LPWDTWProtect, esp32p4LPWDTSWDConf, esp32p4LPWDTSWDWProtect)
	}

	return nil
}
