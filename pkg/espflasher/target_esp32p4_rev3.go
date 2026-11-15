package espflasher

import "time"

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
		if err := disableWatchdogsLP(f, esp32p4LPWDTConfig0, esp32p4LPWDTWProtect, esp32p4LPWDTSWDConf, esp32p4LPWDTSWDWProtect); err != nil {
			return err
		}
	}

	if f.chip != nil && f.chip.SecureDownloadMode {
		return nil
	}
	return esp32p4PowerOnFlash(f)
}

// Flash power registers. Reference: esptool/targets/esp32p4.py.
const (
	esp32p4EfuseRdRepeatData1    uint32 = 0x5012D034
	esp32p4EfuseDownloadXPDOn    uint32 = 1 << 16
	esp32p4LPSysAnaXPDPadGroup   uint32 = 0x5011010C
	esp32p4PMUExtLDOP00P1A       uint32 = 0x501151B8
	esp32p4PMUExtLDOP00P1AAna    uint32 = 0x501151BC
	esp32p4PMUDate               uint32 = 0x501153FC
	esp32p4PMU0P1AForceTieHSel   uint32 = 1 << 7
	esp32p4PMU0P1ATarget0        uint32 = 0xFF << 23
	esp32p4PMUAna0P1AEnCurLim    uint32 = 1 << 27
	esp32p4PMUDateFlashForceOn   uint32 = 0x3
	esp32p4PMUExtLDOP00P1AUpdate uint32 = 0x80
)

// esp32p4PowerOnFlash powers on the flash before SPI attach. Revision 3.1 and
// later boot with the flash off. Reference: esptool/targets/esp32p4.py power_on_flash().
func esp32p4PowerOnFlash(f *Flasher) error {
	rev, err := esp32p4Rev1ReadChipRevision(f)
	if err != nil {
		return err
	}
	revision := rev.Major*100 + rev.Minor
	if revision < 301 {
		return nil
	}

	if revision == 302 {
		efuse, err := f.ReadRegister(esp32p4EfuseRdRepeatData1)
		if err != nil {
			return err
		}
		if efuse&esp32p4EfuseDownloadXPDOn != 0 {
			date, err := f.ReadRegister(esp32p4PMUDate)
			if err != nil {
				return err
			}
			if date&esp32p4PMUDateFlashForceOn == esp32p4PMUDateFlashForceOn {
				return f.WriteRegister(esp32p4PMUDate, date&^esp32p4PMUDateFlashForceOn)
			}
			return nil
		}
	}

	f.logf("Powering on flash (ESP32-P4 rev %s)", rev)
	if err := f.WriteRegister(esp32p4LPSysAnaXPDPadGroup, 1); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)

	steps := []struct {
		addr      uint32
		set, clr  uint32
		sleepTime time.Duration
	}{
		{esp32p4PMUExtLDOP00P1AAna, esp32p4PMUAna0P1AEnCurLim, 0, 0},
		{esp32p4PMUExtLDOP00P1A, esp32p4PMU0P1AForceTieHSel, 0, 0},
		{esp32p4PMUDate, esp32p4PMUDateFlashForceOn, 0, 50 * time.Microsecond},
		{esp32p4PMUExtLDOP00P1AAna, 0, esp32p4PMUAna0P1AEnCurLim, 0},
		{esp32p4PMUExtLDOP00P1A, 0, esp32p4PMU0P1ATarget0, 0},
		{esp32p4PMUExtLDOP00P1A, esp32p4PMUExtLDOP00P1AUpdate, 0, 0},
		{esp32p4PMUExtLDOP00P1A, 0, esp32p4PMU0P1AForceTieHSel, 1800 * time.Microsecond},
	}
	for _, s := range steps {
		v, err := f.ReadRegister(s.addr)
		if err != nil {
			return err
		}
		if err := f.WriteRegister(s.addr, v&^s.clr|s.set); err != nil {
			return err
		}
		time.Sleep(s.sleepTime)
	}
	return nil
}

// esp32p4DefByRevision selects the ESP32-P4 definition from the eFuse revision,
// because rev1 and rev3 share chip ID 18.
// Reference: esptool/targets/esp32p4.py ESP32P4StubLoader.stub_json_name().
func (f *Flasher) esp32p4DefByRevision() *chipDef {
	rev, err := esp32p4Rev1ReadChipRevision(f)
	if err != nil {
		f.logf("unable to read ESP32-P4 revision, assuming rev3: %s", err)
		return defESP32P4Rev3
	}
	if rev.Major < 3 {
		return defESP32P4Rev1
	}
	return defESP32P4Rev3
}
