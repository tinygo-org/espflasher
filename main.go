// Command espflasher flashes and inspects Espressif ESP8266 and ESP32-family
// microcontrollers over a serial (UART) or USB-JTAG/Serial connection.
//
// Install:
//
//	go install tinygo.org/x/espflasher@latest
//
// Usage:
//
//	espflasher write-flash -port /dev/ttyUSB0 0x0 firmware.bin
//	espflasher read-flash -port /dev/ttyUSB0 0x0 ALL backup.bin
//	espflasher chip-id -port /dev/ttyUSB0
//	espflasher help
//
// The original usage without a command still works:
//
//	espflasher -port /dev/ttyUSB0 -offset 0x0 firmware.bin
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		if !errors.Is(err, errUsage) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		if c := lookupCommand(args[0]); c != nil {
			return c.run(args[1:])
		}
	}
	return runLegacy(args)
}
