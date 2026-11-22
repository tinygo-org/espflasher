package webusb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriverFor(t *testing.T) {
	assert.Equal(t, driverCP210x, driverFor(0x10C4, 0xEA60))
	assert.Equal(t, driverCH34x, driverFor(0x1A86, 0x7523))
	assert.Equal(t, driverCDC, driverFor(0x1A86, 0x55D4))
	assert.Equal(t, driverCDC, driverFor(0x303A, 0x1001))
}

func TestCDCLineCoding(t *testing.T) {
	assert.Equal(t, []byte{0x00, 0xC2, 0x01, 0x00, 0, 0, 8}, cdcLineCoding(115200))
}

func TestLineStates(t *testing.T) {
	assert.Equal(t, uint16(3), cdcLineState(true, true))
	assert.Equal(t, uint16(0x0301), cp210xMHS(true, false))
	assert.Equal(t, uint16(0x0302), cp210xMHS(false, true))
	assert.Equal(t, uint16(0xFFDF), ch34xModemCtrl(true, false))
	assert.Equal(t, uint16(0xFFBF), ch34xModemCtrl(false, true))
	assert.Equal(t, uint16(0xFFFF), ch34xModemCtrl(false, false))
}

func TestCH34xDivisor(t *testing.T) {
	tests := []struct {
		baud int
		want uint16
	}{
		{9600, 0xB202},
		{115200, 0xCC03},
		{230400, 0xE603},
		{460800, 0xF303},
		{921600, 0xF307},
	}
	for _, tt := range tests {
		got, err := ch34xDivisor(tt.baud)
		require.NoError(t, err)
		assert.Equalf(t, tt.want, got, "baud %d", tt.baud)
	}
}

func TestPortName(t *testing.T) {
	assert.Equal(t, "webusb:303a:1001:F4:12", portName(0x303A, 0x1001, "F4:12"))
}
