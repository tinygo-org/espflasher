//go:build js && wasm

package webusb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"syscall/js"
)

// driver implements the chip specific control requests.
type driver interface {
	setup(p *Port) error
	setBaud(p *Port, baud int) error
	setLines(p *Port, dtr, rts bool) error
}

func newDriver(kind driverKind) driver {
	switch kind {
	case driverCP210x:
		return &cp210x{}
	case driverCH34x:
		return &ch34x{}
	default:
		return &cdcACM{}
	}
}

func (p *Port) controlOut(reqType, recipient string, request, value, index uint16, data []byte) error {
	setup := map[string]any{
		"requestType": reqType,
		"recipient":   recipient,
		"request":     int(request),
		"value":       int(value),
		"index":       int(index),
	}
	var res js.Value
	var err error
	if data != nil {
		buf := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(buf, data)
		res, err = await(p.dev.Call("controlTransferOut", setup, buf))
	} else {
		res, err = await(p.dev.Call("controlTransferOut", setup))
	}
	if err != nil {
		return err
	}
	if s := res.Get("status").String(); s != "ok" {
		return fmt.Errorf("webusb: control request 0x%02x: %s", request, s)
	}
	return nil
}

func (p *Port) controlIn(reqType, recipient string, request, value, index uint16, length int) ([]byte, error) {
	setup := map[string]any{
		"requestType": reqType,
		"recipient":   recipient,
		"request":     int(request),
		"value":       int(value),
		"index":       int(index),
	}
	res, err := await(p.dev.Call("controlTransferIn", setup, length))
	if err != nil {
		return nil, err
	}
	if s := res.Get("status").String(); s != "ok" {
		return nil, fmt.Errorf("webusb: control request 0x%02x: %s", request, s)
	}
	return dataViewBytes(res.Get("data")), nil
}

// claimBulk claims iface and records its bulk endpoints.
func (p *Port) claimBulk(iface js.Value) error {
	num := iface.Get("interfaceNumber").Int()
	if _, err := await(p.dev.Call("claimInterface", num)); err != nil {
		return fmt.Errorf("webusb: claim interface %d: %w", num, err)
	}
	p.claimed = append(p.claimed, num)
	eps := iface.Get("alternate").Get("endpoints")
	for i := 0; i < eps.Length(); i++ {
		ep := eps.Index(i)
		if ep.Get("type").String() != "bulk" {
			continue
		}
		if ep.Get("direction").String() == "in" {
			p.inEP = ep.Get("endpointNumber").Int()
		} else {
			p.outEP = ep.Get("endpointNumber").Int()
		}
	}
	if p.inEP == 0 || p.outEP == 0 {
		return fmt.Errorf("webusb: interface %d has no bulk endpoints", num)
	}
	return nil
}

func interfaces(dev js.Value) []js.Value {
	list := dev.Get("configuration").Get("interfaces")
	out := make([]js.Value, list.Length())
	for i := range out {
		out[i] = list.Index(i)
	}
	return out
}

type cdcACM struct {
	comm int
}

func (d *cdcACM) setup(p *Port) error {
	var data js.Value
	d.comm = -1
	for _, iface := range interfaces(p.dev) {
		switch iface.Get("alternate").Get("interfaceClass").Int() {
		case 0x02:
			if d.comm < 0 {
				d.comm = iface.Get("interfaceNumber").Int()
			}
		case 0x0A:
			if data.IsUndefined() {
				data = iface
			}
		}
	}
	if d.comm < 0 || data.IsUndefined() {
		return errors.New("webusb: no CDC-ACM interfaces found")
	}
	if _, err := await(p.dev.Call("claimInterface", d.comm)); err != nil {
		return fmt.Errorf("webusb: claim interface %d: %w", d.comm, err)
	}
	p.claimed = append(p.claimed, d.comm)
	return p.claimBulk(data)
}

func (d *cdcACM) setBaud(p *Port, baud int) error {
	return p.controlOut("class", "interface", 0x20, 0, uint16(d.comm), cdcLineCoding(baud))
}

func (d *cdcACM) setLines(p *Port, dtr, rts bool) error {
	return p.controlOut("class", "interface", 0x22, cdcLineState(dtr, rts), uint16(d.comm), nil)
}

// cp210x request codes. Ref: Silicon Labs AN571, table 3.
const (
	cp210xIfcEnable   = 0x00
	cp210xSetLineCtl  = 0x03
	cp210xSetMHS      = 0x07
	cp210xSetBaudRate = 0x1E
)

type cp210x struct{}

func (d *cp210x) setup(p *Port) error {
	ifaces := interfaces(p.dev)
	if len(ifaces) == 0 {
		return errors.New("webusb: cp210x has no interfaces")
	}
	if err := p.claimBulk(ifaces[0]); err != nil {
		return err
	}
	if err := p.controlOut("vendor", "interface", cp210xIfcEnable, 1, 0, nil); err != nil {
		return err
	}
	return p.controlOut("vendor", "interface", cp210xSetLineCtl, 0x0800, 0, nil)
}

func (d *cp210x) setBaud(p *Port, baud int) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(baud))
	return p.controlOut("vendor", "interface", cp210xSetBaudRate, 0, 0, b)
}

func (d *cp210x) setLines(p *Port, dtr, rts bool) error {
	return p.controlOut("vendor", "interface", cp210xSetMHS, cp210xMHS(dtr, rts), 0, nil)
}

// ch34x request codes and registers. Ref: Linux drivers/usb/serial/ch341.c.
const (
	ch34xReqReadVersion = 0x5F
	ch34xReqWriteReg    = 0x9A
	ch34xReqSerialInit  = 0xA1
	ch34xReqModemCtrl   = 0xA4
	ch34xRegDivPre      = 0x1312
	ch34xRegLCR         = 0x2518
	ch34xLCR8N1         = 0xC3
)

type ch34x struct {
	version byte
}

func (d *ch34x) setup(p *Port) error {
	ifaces := interfaces(p.dev)
	if len(ifaces) == 0 {
		return errors.New("webusb: ch34x has no interfaces")
	}
	if err := p.claimBulk(ifaces[0]); err != nil {
		return err
	}
	v, err := p.controlIn("vendor", "device", ch34xReqReadVersion, 0, 0, 2)
	if err != nil {
		return err
	}
	if len(v) > 0 {
		d.version = v[0]
	}
	return p.controlOut("vendor", "device", ch34xReqSerialInit, 0, 0, nil)
}

func (d *ch34x) setBaud(p *Port, baud int) error {
	val, err := ch34xDivisor(baud)
	if err != nil {
		return err
	}
	// Disables 32 byte RX buffering on newer chips. Ref: ch341.c.
	if d.version > 0x27 {
		val |= 1 << 7
	}
	if err := p.controlOut("vendor", "device", ch34xReqWriteReg, ch34xRegDivPre, val, nil); err != nil {
		return err
	}
	if d.version < 0x30 {
		return nil
	}
	return p.controlOut("vendor", "device", ch34xReqWriteReg, ch34xRegLCR, ch34xLCR8N1, nil)
}

func (d *ch34x) setLines(p *Port, dtr, rts bool) error {
	return p.controlOut("vendor", "device", ch34xReqModemCtrl, ch34xModemCtrl(dtr, rts), 0, nil)
}
