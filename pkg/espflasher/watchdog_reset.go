package espflasher

import (
	"fmt"
	"time"
)

// RTC watchdog reset values, from esptool watchdog_reset() in
// esptool/targets/esp32s2.py, esp32s3.py and esp32c3.py.
const (
	rtcWDTWKey              uint32 = 0x50D83AA1
	rtcWDTConfig0ResetValue uint32 = (1 << 31) | (5 << 28) | (1 << 8) | 2
	rtcWDTConfig1ResetTicks uint32 = 2000

	// GPIO_STRAP bit 3 set means the boot pin was high (SPI boot).
	// Reference: esp-idf components/soc/<chip>/include/soc/boot_mode.h
	gpioStrapSPIBootMask uint32 = 1 << 3
)

// rtcWDTReset describes the registers for a strap-gated RTC watchdog reset.
// The DTR/RTS reset over USB only resets the digital core, so a boot pin
// strapped low at power-on still selects download mode. The RTC watchdog
// reset latches the strapping pins again.
type rtcWDTReset struct {
	strapReg          uint32
	option1Reg        uint32
	forceDownloadMask uint32
	wdtWProtect       uint32
	wdtConfig0        uint32
	wdtConfig1        uint32
}

// hardReset does the watchdog reset when the chip is on USB and download
// mode came from the boot pin strap. It returns false to fall back to the
// DTR/RTS reset. Mirrors esptool ESP32S3ROM.hard_reset().
func (r rtcWDTReset) hardReset(f *Flasher) bool {
	if !f.usesUSB {
		return false
	}

	strap, err := f.ReadRegister(r.strapReg)
	if err != nil {
		return false
	}
	option1, err := f.ReadRegister(r.option1Reg)
	if err != nil {
		return false
	}
	if strap&gpioStrapSPIBootMask != 0 || option1&r.forceDownloadMask != 0 {
		return false
	}

	if err := r.watchdogReset(f); err != nil {
		f.logf("watchdog reset failed, falling back to DTR/RTS reset: %v", err)
		return false
	}
	return true
}

func (r rtcWDTReset) watchdogReset(f *Flasher) error {
	if err := f.WriteRegister(r.wdtWProtect, rtcWDTWKey); err != nil {
		return fmt.Errorf("unlock RTC WDT: %w", err)
	}
	if err := f.WriteRegister(r.wdtConfig1, rtcWDTConfig1ResetTicks); err != nil {
		return fmt.Errorf("set RTC WDT timeout: %w", err)
	}
	if err := f.WriteRegister(r.wdtConfig0, rtcWDTConfig0ResetValue); err != nil {
		return fmt.Errorf("enable RTC WDT: %w", err)
	}
	if err := f.WriteRegister(r.wdtWProtect, 0); err != nil {
		return fmt.Errorf("lock RTC WDT: %w", err)
	}

	f.logf("Hard resetting with a watchdog...")
	time.Sleep(500 * time.Millisecond)
	return nil
}
