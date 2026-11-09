package desktop

import (
	"encoding/base64"
	"testing"
	"unicode/utf16"
)

func TestEncodePowerShell(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString(encodePowerShell("echo 'ã'"))
	if err != nil {
		t.Fatal(err)
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	if got := string(utf16.Decode(u)); got != "echo 'ã'" {
		t.Errorf("round trip = %q", got)
	}
}
