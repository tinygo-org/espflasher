# espflasher

[![Test](https://github.com/tinygo-org/espflasher/actions/workflows/test.yml/badge.svg)](https://github.com/tinygo-org/espflasher/actions/workflows/test.yml)

A Go command-line tool and library for flashing firmware to Espressif ESP8266 and ESP32-family microcontrollers over a serial (UART) connection.

## Supported Chips

- ESP8266
- ESP32
- ESP32-S2
- ESP32-S3
- ESP32-C2 (ESP8684)
- ESP32-C3
- ESP32-C6
- ESP32-C6-LCD-1.47 by Waveshare
- ESP32-H2

## CLI Tool

### Installation

You can download and install one of the prebuilt binaries for your operating system under "Releases" or install from source:

```bash
go install tinygo.org/x/espflasher@latest
```

### CLI Usage

```bash
# Install
go install tinygo.org/x/espflasher@latest

# Flash a single binary
espflasher -port /dev/ttyUSB0 firmware.bin

# Flash with specific offset and chip
espflasher -port /dev/ttyUSB0 -offset 0x10000 -chip esp32s3 app.bin

# Flash multiple images (bootloader + partitions + app)
espflasher -port /dev/ttyUSB0 \
    -bootloader bootloader.bin \
    -partitions partitions.bin \
    -app application.bin

# Erase flash before writing
espflasher -port /dev/ttyUSB0 -erase-all firmware.bin
```

## Library

### Installation

```bash
go get tinygo.org/x/espflasher/pkg/espflasher
```

### Quick Start

```go
package main

import (
    "fmt"
    "log"
    "os"

    "tinygo.org/x/espflasher/pkg/espflasher"
)

func main() {
    // Connect to the ESP device
    flasher, err := espflasher.NewFlasher("/dev/ttyUSB0", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer flasher.Close()

    fmt.Printf("Connected to %s\n", flasher.ChipName())

    // Read the firmware binary
    data, err := os.ReadFile("firmware.bin")
    if err != nil {
        log.Fatal(err)
    }

    // Flash with progress reporting
    err = flasher.FlashImage(data, 0x0, func(current, total int) {
        fmt.Printf("\rFlashing: %d/%d bytes (%.0f%%)", current, total,
            float64(current)/float64(total)*100)
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println()

    // Reset the device to run the new firmware
    flasher.Reset()
    fmt.Println("Done!")
}
```

## Features

- **Auto-detection**: Automatically identifies the connected ESP chip
- **Compressed transfers**: Uses zlib compression for significantly faster flashing
- **Multi-image support**: Flash bootloader, partition table, and application in one operation
- **Progress callbacks**: Monitor flash progress in real-time
- **MD5 verification**: Verifies written data integrity after flashing
- **Configurable**: Customize baud rate, compression, reset mode, and more

## API Overview

### Creating a Flasher

```go
// With default options (115200 baud, auto-detect, compressed)
flasher, err := espflasher.NewFlasher("/dev/ttyUSB0", nil)

// With custom options
opts := espflasher.DefaultOptions()
opts.FlashBaudRate = 921600
opts.ChipType = espflasher.ChipESP32S3
opts.Logger = &espflasher.StdoutLogger{W: os.Stdout}
flasher, err := espflasher.NewFlasher("/dev/ttyUSB0", opts)
```

### Flashing a Single Binary

```go
data, _ := os.ReadFile("firmware.bin")
err := flasher.FlashImage(data, 0x0, progressCallback)
```

### Flashing Multiple Images

```go
images := []espflasher.ImagePart{
    {Data: bootloaderBin, Offset: 0x1000},
    {Data: partitionsBin, Offset: 0x8000},
    {Data: applicationBin, Offset: 0x10000},
}
err := flasher.FlashImages(images, progressCallback)
```

### Other Operations

```go
// Erase entire flash
err := flasher.EraseFlash()

// Erase a specific region (must be sector-aligned)
err := flasher.EraseRegion(0x10000, 0x100000)

// Read a hardware register
val, err := flasher.ReadRegister(0x3FF00050)

// Hard reset the device
flasher.Reset()
```

## Architecture

The library is organized in layers:

| Layer | File(s) | Description |
|-------|---------|-------------|
| SLIP | `pkg/espflasher/slip.go` | Serial Line Internet Protocol framing |
| Protocol | `pkg/espflasher/protocol.go` | ROM bootloader command/response protocol |
| Chip | `pkg/espflasher/chip.go`, `pkg/espflasher/target_*.go` | Per-target definitions and detection |
| Reset | `pkg/espflasher/reset.go` | Hardware reset strategies |
| Flasher | `pkg/espflasher/flasher.go` | High-level flash/verify/reset API |
| CLI | `main.go` | Command-line interface |

## Protocol Reference

This library implements the ESP serial bootloader protocol as documented by Espressif's [esptool](https://github.com/espressif/esptool). Key protocol features:

- SLIP framing (RFC 1055) over UART
- 8-byte command/response headers with opcodes
- XOR checksum verification
- Compressed flash writes via zlib deflate
- MD5 verification of flash contents
- SPI flash parameter configuration
