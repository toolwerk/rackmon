//go:build linux

package agent

import "testing"

func TestParseThrottled(t *testing.T) {
	for in, want := range map[string]uint64{"throttled=0x50005\n": 0x50005, "0x0": 0, " 0x1 ": 1} {
		v, ok := parseThrottled(in)
		if !ok || v != want {
			t.Errorf("%q: %v %v", in, v, ok)
		}
	}
	for _, in := range []string{"", "abc", "0xZZ", "1"} {
		if _, ok := parseThrottled(in); ok {
			t.Errorf("%q sollte scheitern", in)
		}
	}
}
