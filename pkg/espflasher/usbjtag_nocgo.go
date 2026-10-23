//go:build darwin && !cgo

package espflasher

// portVIDPID can't enumerate USB devices on macOS without cgo; callers fall
// back to reading UARTDEV_BUF_NO out of ROM.
var portVIDPID = func(name string) (vid, pid string, ok bool) { return "", "", false }

var findUSBJTAGPort = func(prefer string) (name string, ok bool) { return "", false }
