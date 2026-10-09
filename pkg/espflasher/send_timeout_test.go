package espflasher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// blockingPort models a USB CDC port whose device doesn't read.
type blockingPort struct {
	recordingPort
	blockWrite bool
	release    chan struct{}
}

func (b *blockingPort) Write(p []byte) (int, error) {
	if b.blockWrite {
		<-b.release
	}
	return len(p), nil
}

func (b *blockingPort) Drain() error {
	<-b.release
	return nil
}

func TestSendCommandTimeout(t *testing.T) {
	for _, tt := range []struct {
		name       string
		blockWrite bool
		usesUSB    bool
	}{
		{"drain blocks", false, false},
		{"write blocks", true, false},
		{"usb write blocks", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			port := &blockingPort{blockWrite: tt.blockWrite, release: make(chan struct{})}
			defer close(port.release)
			c := newConn(port)
			c.setUSB(tt.usesUSB)

			start := time.Now()
			err := c.sendCommand(cmdSync, make([]byte, 36), 0)
			assert.ErrorIs(t, err, errSendTimeout)
			assert.Less(t, time.Since(start), 2*time.Second)
		})
	}
}

func TestSendTimeoutCoversLargeFrames(t *testing.T) {
	assert.Equal(t, time.Second+50*time.Millisecond, sendTimeout(48))
	// A worst case SLIP encoded 16 KiB flash block at 9600 baud.
	assert.Greater(t, sendTimeout(2*0x4000+64), 34*time.Second)
}
