//go:build js && wasm

package webusb

import (
	"syscall/js"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/serial"
)

// mockDevice builds a fake USBDevice that records control and bulk transfers.
const mockDevice = `(function(vid, pid, serial, ifaces) {
  const d = {
    vendorId: vid, productId: pid, serialNumber: serial,
    opened: false, configuration: null,
    controls: [], out: [], claimed: [], pending: [], queued: [],
    open() { this.opened = true; return Promise.resolve(); },
    close() { this.opened = false; this.pending.forEach(p => p.reject(new Error("closed"))); return Promise.resolve(); },
    selectConfiguration() { this.configuration = { interfaces: ifaces }; return Promise.resolve(); },
    claimInterface(n) { this.claimed.push(n); return Promise.resolve(); },
    releaseInterface() { return Promise.resolve(); },
    controlTransferOut(s, data) {
      this.controls.push({ dir: "out", ...s, data: data ? Array.from(data) : null });
      return Promise.resolve({ status: "ok" });
    },
    controlTransferIn(s, len) {
      this.controls.push({ dir: "in", ...s });
      return Promise.resolve({ status: "ok", data: new DataView(new Uint8Array([0x31, 0]).buffer) });
    },
    transferOut(ep, data) {
      this.out.push(Array.from(data));
      return Promise.resolve({ status: "ok", bytesWritten: data.length });
    },
    transferIn(ep, len) {
      if (this.queued.length) {
        const b = this.queued.shift();
        return Promise.resolve({ status: "ok", data: new DataView(b.buffer) });
      }
      return new Promise((resolve, reject) => this.pending.push({ resolve, reject }));
    },
    push(bytes) {
      const b = new Uint8Array(bytes);
      const p = this.pending.shift();
      if (p) p.resolve({ status: "ok", data: new DataView(b.buffer) }); else this.queued.push(b);
    },
  };
  return d;
})`

func bulkIface(num, class int) map[string]any {
	return map[string]any{
		"interfaceNumber": num,
		"alternate": map[string]any{
			"interfaceClass": class,
			"endpoints": []any{
				map[string]any{"endpointNumber": 1, "direction": "out", "type": "bulk"},
				map[string]any{"endpointNumber": 2, "direction": "in", "type": "bulk"},
			},
		},
	}
}

func newMock(vid, pid int, serial string, ifaces ...any) js.Value {
	return js.Global().Call("eval", mockDevice).Invoke(vid, pid, serial, ifaces)
}

func cdcMock() js.Value {
	comm := map[string]any{
		"interfaceNumber": 0,
		"alternate":       map[string]any{"interfaceClass": 2, "endpoints": []any{}},
	}
	return newMock(0x303A, 0x1001, "AA", comm, bulkIface(1, 0x0A))
}

type control struct {
	request, value, index int
	data                  []byte
}

func controls(dev js.Value) []control {
	list := dev.Get("controls")
	out := make([]control, list.Length())
	for i := range out {
		c := list.Index(i)
		out[i] = control{request: c.Get("request").Int(), value: c.Get("value").Int(), index: c.Get("index").Int()}
		if d := c.Get("data"); d.Type() == js.TypeObject && !d.IsNull() {
			for j := 0; j < d.Length(); j++ {
				out[i].data = append(out[i].data, byte(d.Index(j).Int()))
			}
		}
	}
	return out
}

var mode8N1 = &serial.Mode{BaudRate: 115200, DataBits: 8}

func TestCDCOpen(t *testing.T) {
	dev := cdcMock()
	p, err := Open(dev, mode8N1)
	require.NoError(t, err)
	defer p.Close()

	assert.Equal(t, 2, dev.Get("claimed").Length())
	assert.Equal(t, []control{{0x20, 0, 0, cdcLineCoding(115200)}}, controls(dev))

	require.NoError(t, p.SetDTRAndRTS(true, false))
	require.NoError(t, p.SetRTS(true))
	c := controls(dev)
	assert.Equal(t, control{request: 0x22, value: 1}, c[1])
	assert.Equal(t, control{request: 0x22, value: 3}, c[2])
}

func TestReadWrite(t *testing.T) {
	dev := cdcMock()
	p, err := Open(dev, mode8N1)
	require.NoError(t, err)
	defer p.Close()

	require.NoError(t, p.SetReadTimeout(20*time.Millisecond))
	buf := make([]byte, 8)
	n, err := p.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 0, n)

	dev.Call("push", []any{1, 2, 3})
	require.NoError(t, p.SetReadTimeout(time.Second))
	n, err = p.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, buf[:n])

	n, err = p.Write([]byte{0xC0, 0x00, 0xC0})
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, 1, dev.Get("out").Length())

	dev.Call("push", []any{9})
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, p.ResetInputBuffer())
	require.NoError(t, p.SetReadTimeout(10*time.Millisecond))
	n, _ = p.Read(buf)
	assert.Equal(t, 0, n)
}

func TestReadAfterClose(t *testing.T) {
	p, err := Open(cdcMock(), mode8N1)
	require.NoError(t, err)
	require.NoError(t, p.Close())
	_, err = p.Read(make([]byte, 1))
	assert.Error(t, err)
}

func TestCP210x(t *testing.T) {
	dev := newMock(0x10C4, 0xEA60, "", bulkIface(0, 0xFF))
	p, err := Open(dev, mode8N1)
	require.NoError(t, err)
	defer p.Close()
	require.NoError(t, p.SetDTRAndRTS(false, true))
	assert.Equal(t, []control{
		{cp210xIfcEnable, 1, 0, nil},
		{cp210xSetLineCtl, 0x0800, 0, nil},
		{cp210xSetBaudRate, 0, 0, []byte{0x00, 0xC2, 0x01, 0x00}},
		{cp210xSetMHS, 0x0302, 0, nil},
	}, controls(dev))
}

func TestCH34x(t *testing.T) {
	dev := newMock(0x1A86, 0x7523, "", bulkIface(0, 0xFF))
	p, err := Open(dev, mode8N1)
	require.NoError(t, err)
	defer p.Close()
	require.NoError(t, p.SetDTRAndRTS(true, false))
	assert.Equal(t, []control{
		{ch34xReqReadVersion, 0, 0, nil},
		{ch34xReqSerialInit, 0, 0, nil},
		{ch34xReqWriteReg, ch34xRegDivPre, 0xCC83, nil},
		{ch34xReqWriteReg, ch34xRegLCR, ch34xLCR8N1, nil},
		{ch34xReqModemCtrl, 0xFFDF, 0, nil},
	}, controls(dev))
}

func TestOpenPortAndList(t *testing.T) {
	dev := cdcMock()
	nav := js.Global().Get("navigator")
	if nav.IsUndefined() {
		nav = js.Global().Get("Object").New()
		js.Global().Set("navigator", nav)
	}
	usbObj := js.Global().Get("Object").New()
	usbObj.Set("getDevices", js.FuncOf(func(js.Value, []js.Value) any {
		return js.Global().Get("Promise").Call("resolve", []any{dev})
	}))
	js.Global().Get("Object").Call("defineProperty", nav, "usb", map[string]any{"value": usbObj, "configurable": true})

	ports, err := ListUSBPorts()
	require.NoError(t, err)
	require.Len(t, ports, 1)
	assert.Equal(t, "webusb:303a:1001:AA", ports[0].Name)
	assert.Equal(t, "303A", ports[0].VID)
	assert.Equal(t, "1001", ports[0].PID)

	p, err := OpenPort("webusb:303a:1001:AA", mode8N1)
	require.NoError(t, err)
	p.Close()

	_, err = OpenPort("webusb:0000:0000:", mode8N1)
	assert.Error(t, err)
}
