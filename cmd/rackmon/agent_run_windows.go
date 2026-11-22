//go:build windows

package main

import (
	"context"

	"golang.org/x/sys/windows/svc"

	"github.com/toolwerk/rackmon/internal/agent"
)

const serviceName = "RackmonAgent"

// runAgentOrService läuft als Windows-Dienst, wenn der Dienststeuerungs-Manager es startet, sonst im Vordergrund.
func runAgentOrService(cfg agent.Config) error {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isSvc {
		ctx, stop := signalContext()
		defer stop()
		return agent.Run(ctx, cfg)
	}
	return svc.Run(serviceName, &service{cfg: cfg})
}

type service struct{ cfg agent.Config }

func (s *service) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- agent.Run(ctx, s.cfg) }()
	status <- svc.Status{State: svc.Running, Accepts: accepted}
	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case <-done:
			cancel()
			return false, 1
		}
	}
}
