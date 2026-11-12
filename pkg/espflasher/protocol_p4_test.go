package espflasher

import (
	"encoding/binary"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeResp builds a minimal SLIP response frame for the given opcode.
func makeResp(op byte) []byte {
	resp := make([]byte, 10)
	resp[0] = respDirectionResp
	resp[1] = op
	binary.LittleEndian.PutUint16(resp[2:4], 2) // data len
	binary.LittleEndian.PutUint32(resp[4:8], 0xABCD)
	resp[8] = 0x00 // status OK
	resp[9] = 0x00 // error 0
	return resp
}

// writeOpcodes extracts the opcode (decoded[1]) from each captured write.
func writeOpcodes(writes [][]byte) []byte {
	var ops []byte
	for _, w := range writes {
		d := slipDecode(w)
		if len(d) >= 2 {
			ops = append(ops, d[1])
		}
	}
	return ops
}

// framingPort is a mockPort that records writes and replays a queued stream of
// SLIP-framed responses on Read.
type framingPort struct {
	mockPort
	writes [][]byte
	stream []byte
	pos    int
}

func (p *framingPort) Write(b []byte) (int, error) {
	p.writes = append(p.writes, append([]byte{}, b...))
	return len(b), nil
}
func (p *framingPort) Read(b []byte) (int, error) {
	if p.pos >= len(p.stream) {
		return 0, io.EOF
	}
	n := copy(b, p.stream[p.pos:])
	p.pos += n
	return n, nil
}

func TestCommandSendsPokeInROMMode(t *testing.T) {
	port := &framingPort{stream: slipEncode(makeResp(cmdReadReg))}
	c := &conn{port: port, reader: newSlipReader(port), stub: false}

	_, err := c.command(cmdReadReg, make([]byte, 4), 0, defaultTimeout, true)
	require.NoError(t, err, "ROM command should return the flushed reply")

	ops := writeOpcodes(port.writes)
	// The command must be sent, followed (later) by a SYNC poke.
	cmdIdx, pokeIdx := -1, -1
	for i, op := range ops {
		if op == cmdReadReg && cmdIdx < 0 {
			cmdIdx = i
		}
		if op == cmdSync && pokeIdx < 0 {
			pokeIdx = i
		}
	}
	require.GreaterOrEqual(t, cmdIdx, 0, "command frame should be written")
	require.Greater(t, pokeIdx, cmdIdx, "SYNC poke should follow the command")
}

func TestCommandNoPokeInStubMode(t *testing.T) {
	port := &framingPort{stream: slipEncode(makeResp(cmdReadReg))}
	c := &conn{port: port, reader: newSlipReader(port), stub: true}

	_, err := c.command(cmdReadReg, make([]byte, 4), 0, defaultTimeout, true)
	require.NoError(t, err)

	ops := writeOpcodes(port.writes)
	for _, op := range ops {
		assert.NotEqual(t, cmdSync, op, "stub mode must not emit a SYNC poke")
	}
}

func TestCommandDrainsStaleBeforeMatching(t *testing.T) {
	// A stale (wrong-opcode) frame arrives first, then the real reply.
	stream := append(slipEncode(makeResp(0x77)), slipEncode(makeResp(cmdReadReg))...)
	port := &framingPort{stream: stream}
	c := &conn{port: port, reader: newSlipReader(port), stub: false}

	resp, err := c.command(cmdReadReg, make([]byte, 4), 0, defaultTimeout, true)
	require.NoError(t, err, "stale frames must be skipped, then the matching reply returned")
	require.NotNil(t, resp)
	assert.Equal(t, uint32(0xABCD), resp.Value)
}

func TestCommandFailsFastOnSilentChip(t *testing.T) {
	// A silent chip never answers: Read always hits EOF. The loop must give
	// up near p4CommandDeadline rather than looping forever.
	orig := p4CommandDeadline
	p4CommandDeadline = 60 * time.Millisecond
	defer func() { p4CommandDeadline = orig }()

	port := &framingPort{} // empty stream -> Read returns EOF
	c := &conn{port: port, reader: newSlipReader(port), stub: false}

	start := time.Now()
	_, err := c.command(cmdReadReg, make([]byte, 4), 0, 0, true) // 0 timeout -> p4CommandDeadline
	elapsed := time.Since(start)

	require.Error(t, err, "a silent chip must cause command() to fail")
	assert.Less(t, elapsed, 2*time.Second, "must fail fast near p4CommandDeadline, not loop forever")
}
