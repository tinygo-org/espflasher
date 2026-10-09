package espflasher

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func syncReply(val uint32) []byte {
	pkt := make([]byte, 10)
	pkt[0] = respDirectionResp
	pkt[1] = cmdSync
	binary.LittleEndian.PutUint16(pkt[2:4], 2)
	binary.LittleEndian.PutUint32(pkt[4:8], val)
	return slipEncode(pkt)
}

func TestSyncDetectsStub(t *testing.T) {
	for _, tt := range []struct {
		name    string
		replies []uint32
		want    bool
	}{
		{"rom", []uint32{0x20120707, 0x20120707, 0x20120707}, false},
		{"stub", []uint32{0}, true},
		{"rom extra reply non-zero", []uint32{0, 0x20120707}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			for _, v := range tt.replies {
				buf.Write(syncReply(v))
			}
			mock := &mockPort{readFunc: func(p []byte) (int, error) { return buf.Read(p) }}
			c := &conn{port: mock, reader: newSlipReader(mock)}

			if _, err := c.sync(); err != nil {
				t.Fatalf("sync() error = %v", err)
			}
			if c.isStub() != tt.want {
				t.Errorf("isStub() = %v, want %v", c.isStub(), tt.want)
			}
		})
	}
}

// TestDetectChipStubSkipsSecurityInfo checks that with the stub running,
// detection uses the magic value without sending GET_SECURITY_INFO.
func TestDetectChipStubSkipsSecurityInfo(t *testing.T) {
	mc := &mockConnection{
		stubMode: true,
		securityInfoFunc: func() ([]byte, error) {
			t.Fatal("GET_SECURITY_INFO sent to the stub")
			return nil, nil
		},
		readRegFunc: func(addr uint32) (uint32, error) {
			if addr == chipDetectMagicRegAddr {
				return 0x1B31506F, nil
			}
			return 0, nil
		},
	}
	f := &Flasher{conn: mc, opts: &FlasherOptions{}}

	def, err := f.detectChip()
	if err != nil {
		t.Fatalf("detectChip() error = %v", err)
	}
	if def.ChipType != ChipESP32C3 {
		t.Errorf("detectChip() = %s, want %s", def.ChipType, ChipESP32C3)
	}
}
