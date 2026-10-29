//go:build !darwin

package tray

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestPNGToICO(t *testing.T) {
	png := encodePNG(colorIcon(true))
	ico := pngToICO(png, iconSize)
	le := binary.LittleEndian
	if le.Uint16(ico[2:]) != 1 || le.Uint16(ico[4:]) != 1 {
		t.Fatalf("bad header % x", ico[:6])
	}
	if ico[6] != iconSize || ico[7] != iconSize {
		t.Errorf("size = %dx%d", ico[6], ico[7])
	}
	size, offset := le.Uint32(ico[14:]), le.Uint32(ico[18:])
	if int(size) != len(png) || offset != 22 || !bytes.Equal(ico[offset:], png) {
		t.Errorf("entry size %d offset %d, want %d at 22", size, offset, len(png))
	}
}
