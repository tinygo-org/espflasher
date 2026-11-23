package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

// flashSizeFromID returns the flash size in bytes from the JEDEC capacity byte.
func flashSizeFromID(devID uint16) (uint32, error) {
	capByte := devID & 0xFF
	if capByte < 18 || capByte > 27 {
		return 0, fmt.Errorf("unknown flash capacity byte 0x%02X", capByte)
	}
	return 1 << capByte, nil
}

func runReadFlash(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("read-flash"))
	c.register(fs)
	pos, err := parseFlags(fs, args, 3, 3)
	if err != nil {
		return err
	}
	offset, err := parseNumber(pos[0])
	if err != nil {
		return err
	}
	readAll := strings.EqualFold(pos[1], "all")
	var size uint32
	if !readAll {
		if size, err = parseNumber(pos[1]); err != nil {
			return err
		}
	}
	file := pos[2]

	return c.withFlasher(nil, func(f *espflasher.Flasher) error {
		if readAll {
			_, devID, err := f.FlashID()
			if err != nil {
				return fmt.Errorf("detect flash size: %w", err)
			}
			total, err := flashSizeFromID(devID)
			if err != nil {
				return err
			}
			if offset >= total {
				return fmt.Errorf("offset 0x%X is past end of flash (0x%X)", offset, total)
			}
			size = total - offset
		}
		fmt.Printf("Reading %d bytes at 0x%08X...\n", size, offset)
		data, err := f.ReadFlash(offset, size, progressBar)
		if err != nil {
			return fmt.Errorf("read failed: %w", err)
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote %d bytes to %s\n", len(data), file)
		return nil
	})
}

func runVerifyFlash(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("verify-flash"))
	c.register(fs)
	pos, err := parseFlags(fs, args, 2, -1)
	if err != nil {
		return err
	}
	pairs, err := parseOffsetFilePairs(pos)
	if err != nil {
		return err
	}
	sums := make([]string, len(pairs))
	sizes := make([]uint32, len(pairs))
	for i, p := range pairs {
		data, err := os.ReadFile(p.file)
		if err != nil {
			return err
		}
		sum := md5.Sum(data)
		sums[i] = hex.EncodeToString(sum[:])
		sizes[i] = uint32(len(data))
	}

	return c.withFlasher(nil, func(f *espflasher.Flasher) error {
		failed := 0
		for i, p := range pairs {
			got, err := f.GetFlashMD5(p.offset, sizes[i], nil)
			if err != nil {
				return fmt.Errorf("flash MD5 at 0x%08X: %w", p.offset, err)
			}
			if strings.EqualFold(got, sums[i]) {
				fmt.Printf("Verified %s at 0x%08X\n", p.file, p.offset)
				continue
			}
			failed++
			fmt.Printf("MISMATCH %s at 0x%08X: file %s, flash %s\n", p.file, p.offset, sums[i], got)
		}
		if failed > 0 {
			return fmt.Errorf("%d of %d files did not match", failed, len(pairs))
		}
		return nil
	})
}
