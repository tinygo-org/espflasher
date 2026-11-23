package main

import (
	"fmt"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

func runEraseFlash(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("erase-flash"))
	c.register(fs)
	if _, err := parseFlags(fs, args, 0, 0); err != nil {
		return err
	}
	return c.withFlasher(nil, func(f *espflasher.Flasher) error {
		if err := f.EraseFlash(progressBar); err != nil {
			return fmt.Errorf("erase failed: %w", err)
		}
		return nil
	})
}

func runEraseRegion(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("erase-region"))
	c.register(fs)
	pos, err := parseFlags(fs, args, 2, 2)
	if err != nil {
		return err
	}
	offset, err := parseNumber(pos[0])
	if err != nil {
		return err
	}
	size, err := parseNumber(pos[1])
	if err != nil {
		return err
	}
	return c.withFlasher(nil, func(f *espflasher.Flasher) error {
		if err := f.EraseRegion(offset, size, progressBar); err != nil {
			return fmt.Errorf("erase failed: %w", err)
		}
		fmt.Println("Region erased.")
		return nil
	})
}
