package tools

import (
	"testing"
)

func TestCountUnicode(t *testing.T) {
	for _, str := range []string{
		"1234567890",
		"nihao，世界！",
	} {
		n := CountUnicode(str)
		t.Logf("str:%s, n:%d", str, n)
	}
}
