//go:build js && wasm

// Command webusb flashes an ESP board from the browser over WebUSB.
package main

import (
	"fmt"
	"syscall/js"

	"tinygo.org/x/espflasher/pkg/espflasher"
	"tinygo.org/x/espflasher/pkg/webusb"
)

type jsLogger struct{ fn js.Value }

func (l jsLogger) Logf(format string, args ...any) {
	l.fn.Invoke(fmt.Sprintf(format, args...))
}

// flash(device, data, offset, onLog, onProgress) returns a Promise.
// A negative offset uses the chip's bootloader offset.
func flash(this js.Value, args []js.Value) any {
	if len(args) != 5 {
		return js.Global().Get("Promise").Call("reject", "flash: expected 5 arguments")
	}
	dev, onLog, onProgress := args[0], args[3], args[4]
	offset := args[2].Int()
	data := make([]byte, args[1].Get("length").Int())
	js.CopyBytesToGo(data, args[1])

	var executor js.Func
	executor = js.FuncOf(func(this js.Value, p []js.Value) any {
		resolve, reject := p[0], p[1]
		go func() {
			defer executor.Release()
			if err := run(dev, data, offset, onLog, onProgress); err != nil {
				reject.Invoke(err.Error())
				return
			}
			resolve.Invoke()
		}()
		return nil
	})
	return js.Global().Get("Promise").New(executor)
}

func run(dev js.Value, data []byte, offset int, onLog, onProgress js.Value) error {
	opts := webusb.Options()
	opts.Logger = jsLogger{onLog}
	f, err := espflasher.New(webusb.PortName(dev), opts)
	if err != nil {
		return err
	}
	defer f.Close()
	if offset < 0 {
		off, ok := f.BootloaderFlashOffset()
		if !ok {
			return fmt.Errorf("no default offset for %s", f.ChipName())
		}
		offset = int(off)
	}
	opts.Logger.Logf("Flashing %d bytes at 0x%x", len(data), offset)
	err = f.FlashImage(data, uint32(offset), func(cur, total int) {
		onProgress.Invoke(cur, total)
	})
	if err != nil {
		return err
	}
	f.Reset()
	return nil
}

func main() {
	js.Global().Set("espflasherFilters", js.FuncOf(func(js.Value, []js.Value) any {
		return webusb.Filters()
	}))
	js.Global().Set("espflasherFlash", js.FuncOf(flash))
	select {}
}
