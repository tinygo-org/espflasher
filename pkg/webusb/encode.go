package webusb

import (
	"encoding/binary"
	"fmt"
)

type driverKind int

const (
	driverCDC driverKind = iota
	driverCP210x
	driverCH34x
)

// driverFor picks a driver by VID/PID, falling back to CDC-ACM.
func driverFor(vid, pid uint16) driverKind {
	switch vid {
	case 0x10C4:
		switch pid {
		case 0xEA60, 0xEA70, 0xEA71:
			return driverCP210x
		}
	case 0x1A86:
		switch pid {
		case 0x7523, 0x5523:
			return driverCH34x
		}
	}
	return driverCDC
}

// filterIDs lists the VID/PID pairs offered by Filters. A zero PID matches any.
func filterIDs() [][2]uint16 {
	return [][2]uint16{
		{0x303A, 0}, // Espressif native USB
		{0x10C4, 0xEA60},
		{0x10C4, 0xEA70},
		{0x10C4, 0xEA71},
		{0x1A86, 0x7523},
		{0x1A86, 0x5523},
		{0x1A86, 0x55D4},
	}
}

// cdcLineCoding encodes 8N1 at baud for SET_LINE_CODING.
// Ref: USB CDC PSTN 1.2, section 6.3.11.
func cdcLineCoding(baud int) []byte {
	b := make([]byte, 7)
	binary.LittleEndian.PutUint32(b, uint32(baud))
	b[6] = 8
	return b
}

// cdcLineState is the SET_CONTROL_LINE_STATE value.
// Ref: USB CDC PSTN 1.2, section 6.3.12.
func cdcLineState(dtr, rts bool) uint16 {
	var v uint16
	if dtr {
		v |= 1
	}
	if rts {
		v |= 2
	}
	return v
}

// cp210xMHS is the SET_MHS value writing both DTR and RTS.
// Ref: Silicon Labs AN571, section 5.7.
func cp210xMHS(dtr, rts bool) uint16 {
	return 0x0300 | cdcLineState(dtr, rts)
}

// ch34xModemCtrl is the inverted MODEM_CTRL value.
// Ref: Linux drivers/usb/serial/ch341.c, ch341_set_handshake.
func ch34xModemCtrl(dtr, rts bool) uint16 {
	var v uint16
	if dtr {
		v |= 1 << 5
	}
	if rts {
		v |= 1 << 6
	}
	return ^v
}

// ch34xDivisor returns the prescaler/divisor register value for baud.
// Ref: Linux drivers/usb/serial/ch341.c, ch341_get_divisor.
func ch34xDivisor(baud int) (uint16, error) {
	const clk = 48000000
	clkDiv := func(ps, fact int) int { return 1 << (12 - 3*ps - fact) }
	minRate := func(ps int) int { return clk / (clkDiv(ps, 1) * 512) }
	speed := min(max(baud, minRate(0)), clk/(clkDiv(3, 0)*2))

	ps := 3
	for ; ps >= 0; ps-- {
		if speed > minRate(ps) {
			break
		}
	}
	if ps < 0 {
		return 0, fmt.Errorf("ch34x: unsupported baud rate %d", baud)
	}
	fact := 1
	cd := clkDiv(ps, fact)
	div := clk / (cd * speed)
	if div < 9 || div > 255 {
		div /= 2
		cd *= 2
		fact = 0
	}
	if div < 2 {
		return 0, fmt.Errorf("ch34x: unsupported baud rate %d", baud)
	}
	if 16*clk/(cd*div)-16*speed >= 16*speed-16*clk/(cd*(div+1)) {
		div++
	}
	if fact == 1 && div%2 == 0 {
		div /= 2
		fact = 0
	}
	return uint16((0x100-div)<<8 | fact<<2 | ps), nil
}

func portName(vid, pid uint16, serial string) string {
	return fmt.Sprintf("webusb:%04x:%04x:%s", vid, pid, serial)
}
