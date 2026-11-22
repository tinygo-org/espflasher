// Package webusb provides a serial.Port backed by the browser WebUSB API, so
// espflasher can flash boards when compiled to wasm with Go or TinyGo.
//
// Supported devices are USB CDC-ACM (including the ESP USB-Serial/JTAG and
// USB-OTG ports), Silicon Labs CP210x and WCH CH34x bridges.
package webusb
