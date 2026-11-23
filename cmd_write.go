package main

import (
	"flag"
	"fmt"
	"os"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

type writeFlags struct {
	common           commonFlags
	offset           string
	noCompress       bool
	eraseAll         bool
	flashMode        string
	flashFreq        string
	flashSize        string
	bootloader       string
	partitions       string
	app              string
	bootloaderOffset string
	partitionsOffset string
	appOffset        string
}

func (w *writeFlags) register(fs *flag.FlagSet) {
	w.common.register(fs)
	fs.StringVar(&w.offset, "offset", "0x0", "Flash offset when a single file is given")
	fs.BoolVar(&w.noCompress, "no-compress", false, "Disable compression")
	fs.BoolVar(&w.eraseAll, "erase-all", false, "Erase entire flash before writing")
	fs.StringVar(&w.flashMode, "fm", "keep", "Flash mode: keep, qio, qout, dio, dout")
	fs.StringVar(&w.flashFreq, "ff", "keep", "Flash frequency: keep, 80m, 40m, 26m, 20m (chip-specific)")
	fs.StringVar(&w.flashSize, "fs", "keep", "Flash size: keep, 1MB, 2MB, 4MB, 8MB, 16MB")
	fs.StringVar(&w.bootloader, "bootloader", "", "Bootloader .bin file")
	fs.StringVar(&w.partitions, "partitions", "", "Partition table .bin file")
	fs.StringVar(&w.app, "app", "", "Application .bin file")
	fs.StringVar(&w.bootloaderOffset, "bootloader-offset", "", "Bootloader offset (default: from detected chip)")
	fs.StringVar(&w.partitionsOffset, "partitions-offset", "0x8000", "Partition table offset")
	fs.StringVar(&w.appOffset, "app-offset", "0x10000", "Application offset")
}

type namedPart struct {
	label  string
	file   string
	offset uint32
	// bootloaderDefault means use the chip bootloader offset once connected.
	bootloaderDefault bool
}

// parts builds the list of files to write from flags and positional args.
func (w *writeFlags) parts(positional []string) ([]namedPart, error) {
	multi := w.bootloader != "" || w.partitions != "" || w.app != ""
	if multi && len(positional) > 0 {
		return nil, fmt.Errorf("cannot use both positional files and -bootloader/-partitions/-app")
	}

	if multi {
		var parts []namedPart
		if w.bootloader != "" {
			p := namedPart{label: "Bootloader", file: w.bootloader, bootloaderDefault: w.bootloaderOffset == ""}
			if !p.bootloaderDefault {
				off, err := parseHex(w.bootloaderOffset)
				if err != nil {
					return nil, err
				}
				p.offset = off
			}
			parts = append(parts, p)
		}
		if w.partitions != "" {
			off, err := parseHex(w.partitionsOffset)
			if err != nil {
				return nil, err
			}
			parts = append(parts, namedPart{label: "Partitions", file: w.partitions, offset: off})
		}
		if w.app != "" {
			off, err := parseHex(w.appOffset)
			if err != nil {
				return nil, err
			}
			parts = append(parts, namedPart{label: "App", file: w.app, offset: off})
		}
		return parts, nil
	}

	switch len(positional) {
	case 0:
		return nil, fmt.Errorf("provide a firmware .bin file or use -bootloader/-partitions/-app flags")
	case 1:
		off, err := parseHex(w.offset)
		if err != nil {
			return nil, err
		}
		return []namedPart{{label: "Image", file: positional[0], offset: off}}, nil
	}

	pairs, err := parseOffsetFilePairs(positional)
	if err != nil {
		return nil, err
	}
	parts := make([]namedPart, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, namedPart{label: "Image", file: p.file, offset: p.offset})
	}
	return parts, nil
}

func (w *writeFlags) run(positional []string) error {
	parts, err := w.parts(positional)
	if err != nil {
		return err
	}

	data := make([][]byte, len(parts))
	for i, p := range parts {
		if data[i], err = os.ReadFile(p.file); err != nil {
			return fmt.Errorf("read %s: %w", p.file, err)
		}
	}

	opts, err := w.common.options()
	if err != nil {
		return err
	}
	opts.Compress = !w.noCompress
	opts.FlashMode = w.flashMode
	opts.FlashFreq = w.flashFreq
	opts.FlashSize = w.flashSize

	return w.common.withFlasher(opts, func(f *espflasher.Flasher) error {
		if w.eraseAll {
			if err := f.EraseFlash(nil); err != nil {
				return fmt.Errorf("erase failed: %w", err)
			}
		}

		images := make([]espflasher.ImagePart, len(parts))
		for i, p := range parts {
			off := p.offset
			if p.bootloaderDefault {
				off, _ = f.BootloaderFlashOffset()
			}
			images[i] = espflasher.ImagePart{Data: data[i], Offset: off}
			fmt.Printf("%s: %s (%d bytes) at 0x%08X\n", p.label, p.file, len(data[i]), off)
		}

		if len(images) == 1 {
			err = f.FlashImage(images[0].Data, images[0].Offset, progressBar)
		} else {
			err = f.FlashImages(images, progressBar)
		}
		if err != nil {
			return fmt.Errorf("flash failed: %w", err)
		}
		return nil
	})
}

func runWriteFlash(args []string) error {
	var w writeFlags
	fs := newFlagSet(lookupCommand("write-flash"))
	w.register(fs)
	positional, err := parseFlags(fs, args, 0, -1)
	if err != nil {
		return err
	}
	return w.run(positional)
}

// runLegacy keeps the original flag only usage working.
func runLegacy(args []string) error {
	var w writeFlags
	fs := flag.NewFlagSet(progName(), flag.ContinueOnError)
	w.register(fs)
	version := fs.Bool("version", false, "Print version and exit")
	fs.Usage = func() {
		out := fs.Output()
		printUsage(out)
		fmt.Fprintln(out, "\nOptions when no command is given:")
		fs.PrintDefaults()
	}

	positional, err := parseFlags(fs, args, 0, 1)
	if err != nil {
		return err
	}
	if *version {
		return runVersion(nil)
	}
	if w.common.port == "" {
		fs.Usage()
		return fmt.Errorf("-port is required")
	}
	return w.run(positional)
}
