package main

import (
	"fmt"
	"strings"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

func infoCommand(name string, args []string, fn func(f *espflasher.Flasher) error) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand(name))
	c.register(fs)
	if _, err := parseFlags(fs, args, 0, 0); err != nil {
		return err
	}
	return c.withFlasher(nil, fn)
}

func runChipID(args []string) error {
	return infoCommand("chip-id", args, func(f *espflasher.Flasher) error {
		if rev, err := f.ChipRevision(); err == nil {
			fmt.Printf("Revision: %s\n", rev)
		}
		if features, err := f.ChipFeatures(); err == nil {
			fmt.Printf("Features: %s\n", strings.Join(features, ", "))
		}
		mac, err := f.MAC()
		if err != nil {
			return err
		}
		fmt.Printf("MAC: %s\n", mac)
		return nil
	})
}

func runFlashID(args []string) error {
	return infoCommand("flash-id", args, func(f *espflasher.Flasher) error {
		mfg, dev, err := f.FlashID()
		if err != nil {
			return err
		}
		fmt.Printf("Manufacturer: %02x\n", mfg)
		fmt.Printf("Device: %04x\n", dev)
		if size, err := flashSizeFromID(dev); err == nil {
			fmt.Printf("Detected flash size: %s\n", formatSize(size))
		}
		return nil
	})
}

func formatSize(n uint32) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%dMB", n>>20)
	}
	return fmt.Sprintf("%dKB", n>>10)
}

func runReadMAC(args []string) error {
	return infoCommand("read-mac", args, func(f *espflasher.Flasher) error {
		mac, err := f.MAC()
		if err != nil {
			return err
		}
		fmt.Printf("MAC: %s\n", mac)
		return nil
	})
}

func runSecurityInfo(args []string) error {
	var c commonFlags
	fs := newFlagSet(lookupCommand("get-security-info"))
	c.register(fs)
	if _, err := parseFlags(fs, args, 0, 0); err != nil {
		return err
	}
	opts, err := c.options()
	if err != nil {
		return err
	}
	// GET_SECURITY_INFO is only answered by the ROM loader, not the stub.
	opts.SkipStub = true
	return c.withFlasher(opts, func(f *espflasher.Flasher) error {
		si, err := f.GetSecurityInfo()
		if err != nil {
			return err
		}
		printSecurityInfo(si)
		return nil
	})
}

func printSecurityInfo(si *espflasher.SecurityInfo) {
	fmt.Printf("Flags: 0x%08x\n", si.Flags)
	fmt.Printf("Flash crypt count: 0x%x\n", si.FlashCryptCnt)
	purposes := make([]string, len(si.KeyPurposes))
	for i, p := range si.KeyPurposes {
		purposes[i] = fmt.Sprint(p)
	}
	fmt.Printf("Key purposes: %s\n", strings.Join(purposes, ", "))
	if si.ChipID != nil {
		fmt.Printf("Chip ID: %d\n", *si.ChipID)
	}
	if si.APIVersion != nil {
		fmt.Printf("API version: %d\n", *si.APIVersion)
	}
	p := si.ParsedFlags
	flags := []struct {
		name string
		set  bool
	}{
		{"Secure boot", p.SecureBootEn},
		{"Secure boot aggressive revoke", p.SecureBootAggressiveRevoke},
		{"Secure download", p.SecureDownloadEnable},
		{"Secure boot key 0 revoked", p.SecureBootKeyRevoke0},
		{"Secure boot key 1 revoked", p.SecureBootKeyRevoke1},
		{"Secure boot key 2 revoked", p.SecureBootKeyRevoke2},
		{"JTAG soft disabled", p.SoftDisJtag},
		{"JTAG hard disabled", p.HardDisJtag},
		{"USB disabled", p.DisUSB},
		{"Download DCache disabled", p.DisDownloadDcache},
		{"Download ICache disabled", p.DisDownloadIcache},
	}
	for _, fl := range flags {
		fmt.Printf("%s: %t\n", fl.name, fl.set)
	}
}
