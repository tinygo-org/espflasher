package main

import (
	"fmt"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

func runReadMem(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("read-mem"))
	c.register(fs)
	pos, err := parseFlags(fs, args, 1, 1)
	if err != nil {
		return err
	}
	addr, err := parseNumber(pos[0])
	if err != nil {
		return err
	}
	return c.withFlasher(nil, func(f *espflasher.Flasher) error {
		val, err := f.ReadRegister(addr)
		if err != nil {
			return err
		}
		fmt.Printf("0x%08x = 0x%08x\n", addr, val)
		return nil
	})
}

func runWriteMem(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("write-mem"))
	c.register(fs)
	pos, err := parseFlags(fs, args, 2, 3)
	if err != nil {
		return err
	}
	addr, err := parseNumber(pos[0])
	if err != nil {
		return err
	}
	value, err := parseNumber(pos[1])
	if err != nil {
		return err
	}
	mask := uint32(0xFFFFFFFF)
	if len(pos) == 3 {
		if mask, err = parseNumber(pos[2]); err != nil {
			return err
		}
	}
	return c.withFlasher(nil, func(f *espflasher.Flasher) error {
		if mask != 0xFFFFFFFF {
			old, err := f.ReadRegister(addr)
			if err != nil {
				return err
			}
			value = maskedValue(old, value, mask)
		}
		if err := f.WriteRegister(addr, value); err != nil {
			return err
		}
		fmt.Printf("Wrote 0x%08x to 0x%08x\n", value, addr)
		return nil
	})
}

func maskedValue(old, value, mask uint32) uint32 {
	return (old &^ mask) | (value & mask)
}

func runRun(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("run"))
	c.register(fs)
	if _, err := parseFlags(fs, args, 0, 0); err != nil {
		return err
	}
	c.after = "hard-reset"
	return c.withFlasher(nil, func(f *espflasher.Flasher) error { return nil })
}

func runVersion(args []string) error {
	fmt.Printf("espflasher %s\n", espflasher.Version)
	return nil
}
