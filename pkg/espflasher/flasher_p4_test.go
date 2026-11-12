package espflasher

import (
	"crypto/md5"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// p4FlashMock builds a mockConnection suitable for a full FlashImages run in
// ROM mode: SPI attach, SPI params, flash begin/data all succeed.
func p4FlashMock() *mockConnection {
	m := &mockConnection{
		spiAttachFunc:  func(uint32) error { return nil },
		spiSetParamsFunc: func(uint32, uint32, uint32, uint32) error { return nil },
		flashBeginFunc: func(uint32, uint32, bool) error { return nil },
		flashDataFunc:  func([]byte, uint32) error { return nil },
	}
	return m
}

// espImage returns a minimal valid ESP image (magic + header) for flashing.
func espImage() []byte {
	img := make([]byte, 32)
	img[0] = espImageMagic // 0xE9
	return img
}

// TestFlashImagesP4ROMSendsFlashEndReboot verifies that flashing a P4 rev3
// chip in ROM mode finalizes with FLASH_END { reboot: true }.
func TestFlashImagesP4ROMSendsFlashEndReboot(t *testing.T) {
	mock := p4FlashMock()
	var rebootArgs []bool
	mock.flashEndFunc = func(reboot bool) error {
		rebootArgs = append(rebootArgs, reboot)
		return nil
	}

	f := &Flasher{
		conn: mock,
		chip: defESP32P4Rev3,
		opts: &FlasherOptions{FlashSize: "4MB"},
	}

	err := f.FlashImages([]ImagePart{{Data: espImage(), Offset: 0x10000}}, nil)
	require.NoError(t, err)

	require.Len(t, rebootArgs, 1, "ROM flash should send exactly one FLASH_END")
	assert.True(t, rebootArgs[0], "P4 ROM FLASH_END must request a reboot")
}

// TestFlashImagesP4StubKeepsBarrierFlashEnd verifies that in stub mode the
// post-loop reboot FLASH_END is skipped: the stub uses a non-reboot FLASH_END
// as its per-part write barrier instead.
func TestFlashImagesP4StubKeepsBarrierFlashEnd(t *testing.T) {
	// A single full stub write-size block so the flashed block equals the
	// data verifyMD5 checks, letting flashMD5Func return a matching digest.
	img := make([]byte, flashWriteSizeStub)
	img[0] = espImageMagic

	var blocks [][]byte
	mock := p4FlashMock()
	mock.stubMode = true
	var rebootArgs []bool
	mock.flashEndFunc = func(reboot bool) error {
		rebootArgs = append(rebootArgs, reboot)
		return nil
	}
	mock.flashDataFunc = func(block []byte, seq uint32) error {
		cp := make([]byte, len(block))
		copy(cp, block)
		blocks = append(blocks, cp)
		return nil
	}
	mock.flashMD5Func = func(uint32, uint32) ([]byte, error) {
		var full []byte
		for _, b := range blocks {
			full = append(full, b...)
		}
		sum := md5.Sum(full)
		return sum[:], nil
	}

	f := &Flasher{
		conn: mock,
		chip: defESP32P4Rev3,
		opts: &FlasherOptions{FlashSize: "4MB"},
	}

	err := f.FlashImages([]ImagePart{{Data: img, Offset: 0x10000}}, nil)
	require.NoError(t, err)

	for _, r := range rebootArgs {
		assert.False(t, r, "stub mode must not send a reboot FLASH_END")
	}
}

// TestFlashImagesNonP4ROMNoRebootFlashEnd verifies that a non-P4 chip flashed
// in ROM mode does not receive the P4-specific reboot FLASH_END.
func TestFlashImagesNonP4ROMNoRebootFlashEnd(t *testing.T) {
	mock := p4FlashMock()
	var endCalls int
	mock.flashEndFunc = func(reboot bool) error {
		endCalls++
		return nil
	}

	f := &Flasher{
		conn: mock,
		chip: defESP32, // a normal chip, ROM mode
		opts: &FlasherOptions{FlashSize: "4MB"},
	}

	err := f.FlashImages([]ImagePart{{Data: espImage(), Offset: 0x10000}}, nil)
	require.NoError(t, err)
	assert.Zero(t, endCalls, "ROM flash of a non-P4 chip sends no FLASH_END")
}

// TestConnectAutoSkipsStubForP4 verifies that connect() honors a target's
// SkipStub flag on its own, without the user setting FlasherOptions.SkipStub:
// selecting the P4 rev3 target must skip the stub loader (P4 flashes in ROM
// mode). We select the target explicitly so the test is deterministic — the
// rev1 and rev3 targets both report chip ID 18, so auto-detection cannot tell
// them apart.
func TestConnectAutoSkipsStubForP4(t *testing.T) {
	var loadStubCalled bool
	mc := &mockConnection{
		syncFunc:       func() (uint32, error) { return 0, nil },
		readRegFunc:    func(uint32) (uint32, error) { return 0, nil },
		loadStubFunc: func(*stub) error {
			loadStubCalled = true
			return nil
		},
	}

	f := &Flasher{
		port:    &mockPort{},
		conn:    mc,
		opts:    &FlasherOptions{ChipType: ChipESP32P4Rev3, ResetMode: ResetNoReset, ConnectAttempts: 7},
	}

	err := f.connect()
	require.NoError(t, err)
	require.NotNil(t, f.chip, "connect() should select the P4 rev3 target")
	assert.Equal(t, ChipESP32P4Rev3, f.chip.ChipType)
	if loadStubCalled {
		t.Error("loadStub should not be called for P4 rev3 (target SkipStub)")
	}
}
