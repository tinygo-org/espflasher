//go:build js && wasm

package webusb

import (
	"errors"
	"syscall/js"
)

// await blocks the calling goroutine until promise settles. It must not be
// called from inside a JS callback.
func await(promise js.Value) (js.Value, error) {
	type result struct {
		v   js.Value
		err error
	}
	ch := make(chan result, 1)
	var then, catch js.Func
	then = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- result{v: args[0]}
		return nil
	})
	catch = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- result{err: jsError(args[0])}
		return nil
	})
	promise.Call("then", then, catch)
	r := <-ch
	then.Release()
	catch.Release()
	return r.v, r.err
}

func jsError(v js.Value) error {
	if v.Type() == js.TypeObject {
		if msg := v.Get("message"); msg.Type() == js.TypeString {
			return errors.New(v.Get("name").String() + ": " + msg.String())
		}
	}
	return errors.New(v.String())
}

func usb() (js.Value, error) {
	u := js.Global().Get("navigator").Get("usb")
	if u.IsUndefined() {
		return js.Value{}, errors.New("webusb: navigator.usb is not available")
	}
	return u, nil
}
