package main

import (
	"errors"
	"flag"
	"io"
	"reflect"
	"testing"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

func TestLookupCommand(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"write-flash", "write-flash"},
		{"write_flash", "write-flash"},
		{"READ_FLASH", "read-flash"},
		{"read-reg", "read-mem"},
		{"write_reg", "write-mem"},
		{"get_security_info", "get-security-info"},
		{"firmware.bin", ""},
		{"-port", ""},
	}
	for _, tt := range tests {
		c := lookupCommand(tt.in)
		got := ""
		if c != nil {
			got = c.name
		}
		if got != tt.want {
			t.Errorf("lookupCommand(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestReorderArgs(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("port", "", "")
	fs.Bool("no-stub", false, "")
	got := reorderArgs(fs, []string{"0x0", "fw.bin", "-port", "COM3", "-no-stub", "-fm=dout"})
	want := []string{"-port", "COM3", "-no-stub", "-fm=dout", "--", "0x0", "fw.bin"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseFlagsPositionalAfterFlags(t *testing.T) {
	var c commonFlags
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c.register(fs)
	pos, err := parseFlags(fs, []string{"0x1000", "-port", "/dev/ttyUSB0", "0x2000"}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if c.port != "/dev/ttyUSB0" || !reflect.DeepEqual(pos, []string{"0x1000", "0x2000"}) {
		t.Errorf("port %q pos %v", c.port, pos)
	}
}

func TestParseFlagsArgCount(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if _, err := parseFlags(fs, []string{"a"}, 2, 2); !errors.Is(err, errUsage) {
		t.Errorf("got %v, want errUsage", err)
	}
}

func TestParseHex(t *testing.T) {
	tests := []struct {
		in      string
		want    uint32
		wantErr bool
	}{
		{"0x10000", 0x10000, false},
		{"0X8000", 0x8000, false},
		{"1000", 0x1000, false},
		{"", 0, true},
		{"0x", 0, true},
		{"zz", 0, true},
	}
	for _, tt := range tests {
		got, err := parseHex(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("parseHex(%q) = %x, %v", tt.in, got, err)
		}
	}
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		in      string
		want    uint32
		wantErr bool
	}{
		{"0x1000", 0x1000, false},
		{"4096", 4096, false},
		{"0xFFFFFFFF", 0xFFFFFFFF, false},
		{"0x100000000", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseNumber(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("parseNumber(%q) = %x, %v", tt.in, got, err)
		}
	}
}

func TestParseChipType(t *testing.T) {
	tests := map[string]espflasher.ChipType{
		"auto":         espflasher.ChipAuto,
		"ESP32-S3":     espflasher.ChipESP32S3,
		"esp32c6":      espflasher.ChipESP32C6,
		"esp32p4":      espflasher.ChipESP32P4Rev3,
		"esp32p4-rev1": espflasher.ChipESP32P4Rev1,
	}
	for in, want := range tests {
		got, err := parseChipType(in)
		if err != nil || got != want {
			t.Errorf("parseChipType(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := parseChipType("esp99"); err == nil {
		t.Error("expected error for unknown chip")
	}
}

func TestParseAfter(t *testing.T) {
	for in, want := range map[string]bool{"hard-reset": true, "hard_reset": true, "no-reset": false} {
		got, err := parseAfter(in)
		if err != nil || got != want {
			t.Errorf("parseAfter(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := parseAfter("soft-reset"); err == nil {
		t.Error("expected error")
	}
}

func TestParseOffsetFilePairs(t *testing.T) {
	got, err := parseOffsetFilePairs([]string{"0x0", "bl.bin", "0x8000", "pt.bin"})
	if err != nil {
		t.Fatal(err)
	}
	want := []offsetFile{{0, "bl.bin"}, {0x8000, "pt.bin"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for _, bad := range [][]string{nil, {"0x0"}, {"x", "a.bin"}} {
		if _, err := parseOffsetFilePairs(bad); err == nil {
			t.Errorf("expected error for %v", bad)
		}
	}
}

func TestWriteParts(t *testing.T) {
	w := writeFlags{offset: "0x1000"}
	parts, err := w.parts([]string{"fw.bin"})
	if err != nil || len(parts) != 1 || parts[0].offset != 0x1000 {
		t.Errorf("single file: %v, %v", parts, err)
	}

	parts, err = w.parts([]string{"0x0", "a.bin", "0x10000", "b.bin"})
	if err != nil || len(parts) != 2 || parts[1].offset != 0x10000 {
		t.Errorf("pairs: %v, %v", parts, err)
	}

	w = writeFlags{bootloader: "bl.bin", app: "app.bin", appOffset: "0x10000"}
	parts, err = w.parts(nil)
	if err != nil || len(parts) != 2 || !parts[0].bootloaderDefault || parts[1].offset != 0x10000 {
		t.Errorf("multi: %v, %v", parts, err)
	}

	if _, err := w.parts([]string{"fw.bin"}); err == nil {
		t.Error("expected error mixing flags and positional files")
	}
	if _, err := (&writeFlags{}).parts(nil); err == nil {
		t.Error("expected error with no files")
	}
}

func TestFlashSizeFromID(t *testing.T) {
	got, err := flashSizeFromID(0x4016)
	if err != nil || got != 4<<20 {
		t.Errorf("got %d, %v", got, err)
	}
	if _, err := flashSizeFromID(0x4000); err == nil {
		t.Error("expected error")
	}
}

func TestMaskedValue(t *testing.T) {
	if got := maskedValue(0xAAAA5555, 0x0000FFFF, 0x00FF00FF); got != 0xAA0055FF {
		t.Errorf("got %08x", got)
	}
}

func TestLegacyDetection(t *testing.T) {
	err := run([]string{"-offset", "0x0", "fw.bin"})
	if err == nil || err.Error() != "-port is required" {
		t.Errorf("got %v", err)
	}
}
