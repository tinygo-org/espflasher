package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

type commonFlags struct {
	port   string
	baud   int
	chip   string
	reset  string
	after  string
	noStub bool
}

func (c *commonFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&c.port, "port", "", "Serial port (e.g. /dev/ttyUSB0, COM3)")
	fs.IntVar(&c.baud, "baud", 460800, "Baud rate used after connecting")
	fs.StringVar(&c.chip, "chip", "auto", "Chip type: auto, esp8266, esp32, esp32s2, esp32s3, esp32c2, esp32c3, esp32c5, esp32c6, esp32h2, esp32p4-rev1, esp32p4 (rev3)")
	fs.StringVar(&c.reset, "reset", "default", "Reset mode before connecting: default, no-reset, usb-jtag, auto")
	fs.StringVar(&c.after, "after", "hard-reset", "Action after the command: hard-reset, no-reset")
	fs.BoolVar(&c.noStub, "no-stub", false, "Do not load the flasher stub")
}

func (c *commonFlags) options() (*espflasher.FlasherOptions, error) {
	if c.port == "" {
		return nil, fmt.Errorf("-port is required")
	}
	chip, err := parseChipType(c.chip)
	if err != nil {
		return nil, err
	}
	reset, err := parseResetMode(c.reset)
	if err != nil {
		return nil, err
	}
	if _, err := parseAfter(c.after); err != nil {
		return nil, err
	}
	opts := espflasher.DefaultOptions()
	opts.FlashBaudRate = c.baud
	opts.ChipType = chip
	opts.ResetMode = reset
	opts.SkipStub = c.noStub
	opts.Logger = &espflasher.StdoutLogger{W: os.Stdout}
	return opts, nil
}

func (c *commonFlags) connect(opts *espflasher.FlasherOptions) (*espflasher.Flasher, error) {
	f, err := espflasher.New(c.port, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Printf("Chip: %s\n", f.ChipName())
	return f, nil
}

func (c *commonFlags) finish(f *espflasher.Flasher) {
	if hard, _ := parseAfter(c.after); hard {
		fmt.Println("Resetting device...")
		f.Reset()
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close flasher: %v\n", err)
	}
}

// withFlasher connects, runs fn, then does the after action.
func (c *commonFlags) withFlasher(opts *espflasher.FlasherOptions, fn func(f *espflasher.Flasher) error) error {
	if opts == nil {
		var err error
		if opts, err = c.options(); err != nil {
			return err
		}
	}
	f, err := c.connect(opts)
	if err != nil {
		return err
	}
	defer c.finish(f)
	return fn(f)
}

func parseAfter(s string) (bool, error) {
	switch strings.ToLower(strings.ReplaceAll(s, "_", "-")) {
	case "hard-reset":
		return true, nil
	case "no-reset":
		return false, nil
	}
	return false, fmt.Errorf("unknown after action: %s", s)
}

func parseResetMode(s string) (espflasher.ResetMode, error) {
	switch strings.ToLower(s) {
	case "default":
		return espflasher.ResetDefault, nil
	case "no-reset":
		return espflasher.ResetNoReset, nil
	case "usb-jtag":
		return espflasher.ResetUSBJTAG, nil
	case "auto":
		return espflasher.ResetAuto, nil
	}
	return 0, fmt.Errorf("unknown reset mode: %s", s)
}

func parseHex(s string) (uint32, error) {
	t := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if t == "" {
		return 0, fmt.Errorf("invalid hex value: %q", s)
	}
	val, err := strconv.ParseUint(t, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid hex value: %s", s)
	}
	return uint32(val), nil
}

// parseNumber accepts hex with 0x prefix or decimal like esptool.
func parseNumber(s string) (uint32, error) {
	val, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", s)
	}
	return uint32(val), nil
}

func parseChipType(s string) (espflasher.ChipType, error) {
	switch strings.ToLower(s) {
	case "auto":
		return espflasher.ChipAuto, nil
	case "esp8266":
		return espflasher.ChipESP8266, nil
	case "esp32":
		return espflasher.ChipESP32, nil
	case "esp32s2", "esp32-s2":
		return espflasher.ChipESP32S2, nil
	case "esp32s3", "esp32-s3":
		return espflasher.ChipESP32S3, nil
	case "esp32c2", "esp32-c2":
		return espflasher.ChipESP32C2, nil
	case "esp32c3", "esp32-c3":
		return espflasher.ChipESP32C3, nil
	case "esp32c5", "esp32-c5":
		return espflasher.ChipESP32C5, nil
	case "esp32c6", "esp32-c6":
		return espflasher.ChipESP32C6, nil
	case "esp32h2", "esp32-h2":
		return espflasher.ChipESP32H2, nil
	case "esp32p4-rev1", "esp32-p4-rev1":
		return espflasher.ChipESP32P4Rev1, nil
	case "esp32p4", "esp32-p4", "esp32p4-rev3", "esp32-p4-rev3":
		return espflasher.ChipESP32P4Rev3, nil
	}
	return espflasher.ChipAuto, fmt.Errorf("unknown chip type: %s", s)
}

func progressBar(current, total int) {
	if total <= 0 {
		return
	}
	pct := float64(current) / float64(total) * 100
	if pct > 100 {
		pct = 100
	}
	bar := int(pct / 2)
	fmt.Printf("\r[%-50s] %6.1f%%", strings.Repeat("#", bar)+strings.Repeat(".", 50-bar), pct)
	if current >= total {
		fmt.Println()
	}
}

// reorderArgs moves positional args after flags so the flag package
// can parse flags that appear after positional args.
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	valueFlags := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !bf.IsBoolFlag() {
			valueFlags[f.Name] = true
		}
	})

	var flagArgs, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flagArgs = append(flagArgs, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		if valueFlags[name] && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	if len(positional) == 0 {
		return flagArgs
	}
	return append(append(flagArgs, "--"), positional...)
}

type offsetFile struct {
	offset uint32
	file   string
}

func parseOffsetFilePairs(args []string) ([]offsetFile, error) {
	if len(args) == 0 || len(args)%2 != 0 {
		return nil, fmt.Errorf("expected <offset> <file> pairs")
	}
	pairs := make([]offsetFile, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		off, err := parseNumber(args[i])
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, offsetFile{offset: off, file: args[i+1]})
	}
	return pairs, nil
}
