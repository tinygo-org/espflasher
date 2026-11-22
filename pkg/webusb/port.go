//go:build js && wasm

package webusb

import (
	"errors"
	"sync"
	"syscall/js"
	"time"

	"go.bug.st/serial"
)

const readChunk = 4096

var errClosed = errors.New("webusb: port closed")

// Port is a serial.Port over a WebUSB device.
type Port struct {
	dev     js.Value
	drv     driver
	inEP    int
	outEP   int
	claimed []int

	dtr, rts bool

	mu          sync.Mutex
	buf         []byte
	readErr     error
	notify      chan struct{}
	readTimeout time.Duration
	closed      bool
}

var _ serial.Port = (*Port)(nil)

// Open opens and configures a USBDevice obtained from navigator.usb.
// It blocks, so call it from a goroutine and not from a JS callback.
func Open(dev js.Value, mode *serial.Mode) (*Port, error) {
	p := &Port{
		dev:         dev,
		drv:         newDriver(driverFor(uint16(dev.Get("vendorId").Int()), uint16(dev.Get("productId").Int()))),
		notify:      make(chan struct{}, 1),
		readTimeout: serial.NoTimeout,
	}
	if !dev.Get("opened").Bool() {
		if _, err := await(dev.Call("open")); err != nil {
			return nil, err
		}
	}
	if dev.Get("configuration").IsNull() {
		if _, err := await(dev.Call("selectConfiguration", 1)); err != nil {
			p.closeDevice()
			return nil, err
		}
	}
	if err := p.drv.setup(p); err != nil {
		p.closeDevice()
		return nil, err
	}
	if mode != nil {
		if err := p.SetMode(mode); err != nil {
			p.closeDevice()
			return nil, err
		}
	}
	go p.readLoop()
	return p, nil
}

func (p *Port) readLoop() {
	for {
		res, err := await(p.dev.Call("transferIn", p.inEP, readChunk))
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		if err != nil {
			p.readErr = err
			p.mu.Unlock()
			p.signal()
			return
		}
		status := res.Get("status").String()
		if status == "ok" {
			p.buf = append(p.buf, dataViewBytes(res.Get("data"))...)
		}
		p.mu.Unlock()
		if status == "stall" {
			await(p.dev.Call("clearHalt", "in", p.inEP)) //nolint:errcheck
		}
		p.signal()
	}
}

func (p *Port) signal() {
	select {
	case p.notify <- struct{}{}:
	default:
	}
}

func dataViewBytes(dv js.Value) []byte {
	if dv.IsUndefined() || dv.IsNull() {
		return nil
	}
	n := dv.Get("byteLength").Int()
	if n == 0 {
		return nil
	}
	u8 := js.Global().Get("Uint8Array").New(dv.Get("buffer"), dv.Get("byteOffset"), n)
	b := make([]byte, n)
	js.CopyBytesToGo(b, u8)
	return b
}

// Read returns buffered bytes, waiting up to the read timeout. It returns
// 0 and no error on timeout, like go.bug.st/serial.
func (p *Port) Read(b []byte) (int, error) {
	p.mu.Lock()
	timeout := p.readTimeout
	p.mu.Unlock()
	var deadline <-chan time.Time
	if timeout >= 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		deadline = t.C
	}
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return 0, errClosed
		}
		if len(p.buf) > 0 {
			n := copy(b, p.buf)
			p.buf = p.buf[n:]
			p.mu.Unlock()
			return n, nil
		}
		if p.readErr != nil {
			err := p.readErr
			p.mu.Unlock()
			return 0, err
		}
		p.mu.Unlock()
		select {
		case <-p.notify:
		case <-deadline:
			return 0, nil
		}
	}
}

// Write sends b on the bulk OUT endpoint and waits for completion.
func (p *Port) Write(b []byte) (int, error) {
	buf := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(buf, b)
	res, err := await(p.dev.Call("transferOut", p.outEP, buf))
	if err != nil {
		return 0, err
	}
	if s := res.Get("status").String(); s != "ok" {
		return 0, errors.New("webusb: transferOut: " + s)
	}
	return res.Get("bytesWritten").Int(), nil
}

// Drain is a no-op since Write waits for the transfer to complete.
func (p *Port) Drain() error { return nil }

func (p *Port) ResetInputBuffer() error {
	p.mu.Lock()
	p.buf = nil
	p.mu.Unlock()
	return nil
}

func (p *Port) ResetOutputBuffer() error { return nil }

// SetMode sets the baud rate. Only 8N1 framing is supported.
func (p *Port) SetMode(mode *serial.Mode) error {
	if mode.DataBits != 0 && mode.DataBits != 8 || mode.Parity != serial.NoParity || mode.StopBits != serial.OneStopBit {
		return errors.New("webusb: only 8N1 is supported")
	}
	return p.drv.setBaud(p, mode.BaudRate)
}

func (p *Port) SetDTR(dtr bool) error { return p.SetDTRAndRTS(dtr, p.rts) }

func (p *Port) SetRTS(rts bool) error { return p.SetDTRAndRTS(p.dtr, rts) }

// SetDTRAndRTS sets both modem lines in one control request.
func (p *Port) SetDTRAndRTS(dtr, rts bool) error {
	p.dtr, p.rts = dtr, rts
	return p.drv.setLines(p, dtr, rts)
}

func (p *Port) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}

func (p *Port) SetReadTimeout(t time.Duration) error {
	p.mu.Lock()
	p.readTimeout = t
	p.mu.Unlock()
	return nil
}

func (p *Port) Break(time.Duration) error {
	return errors.New("webusb: break is not supported")
}

// Close releases the interfaces and closes the device.
func (p *Port) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()
	p.signal()
	return p.closeDevice()
}

func (p *Port) closeDevice() error {
	for _, n := range p.claimed {
		await(p.dev.Call("releaseInterface", n)) //nolint:errcheck
	}
	p.claimed = nil
	if !p.dev.Get("opened").Bool() {
		return nil
	}
	_, err := await(p.dev.Call("close"))
	return err
}
