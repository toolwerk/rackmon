//go:build !linux

package agent

import "context"

// powerWarn gibt es nur auf dem Raspberry Pi (Linux).
func powerWarn(context.Context) *bool { return nil }
