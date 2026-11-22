//go:build !windows

package main

import (
	"github.com/toolwerk/rackmon/internal/agent"
)

func runAgentOrService(cfg agent.Config) error {
	ctx, stop := signalContext()
	defer stop()
	return agent.Run(ctx, cfg)
}
