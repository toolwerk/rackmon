//go:build linux

package agent

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Bits von get_throttled beim Raspberry Pi: 0 Unterspannung, 1 Taktbegrenzung, 2 Drosselung.
const throttledNow = 0x7

// powerWarn liest den Stromstatus eines Raspberry Pi. Auf anderen Rechnern gibt es ihn nicht, dann bleibt er leer.
func powerWarn(ctx context.Context) *bool {
	if v, ok := readThrottled(ctx); ok {
		w := v&throttledNow != 0
		return &w
	}
	return nil
}

func readThrottled(ctx context.Context) (uint64, bool) {
	if b, err := os.ReadFile("/sys/devices/platform/soc/soc:firmware/get_throttled"); err == nil {
		if v, ok := parseThrottled(string(b)); ok {
			return v, true
		}
	}
	path, err := exec.LookPath("vcgencmd")
	if err != nil {
		return 0, false
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, path, "get_throttled").Output()
	if err != nil {
		return 0, false
	}
	return parseThrottled(string(out))
}

// parseThrottled versteht "throttled=0x50005" und "0x50005".
func parseThrottled(s string) (uint64, bool) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "throttled="))
	if !strings.HasPrefix(s, "0x") {
		return 0, false
	}
	v, err := strconv.ParseUint(s[2:], 16, 32)
	return v, err == nil
}
