// Package agent misst die Werte eines Rechners und schickt sie an den Server.
package agent

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"

	"github.com/toolwerk/rackmon/internal/model"
)

// Collector sammelt ein Messpaket. Er merkt sich die Zählerstände des Netzes und die Zahl der Jobs zwischen den Aufrufen.
type Collector struct {
	Version string

	lastRx, lastTx uint64
	lastNet        time.Time
	jobs           int
	jobsAt         time.Time
}

const jobsEvery = 10 * time.Second

// Collect misst jetzt. Einzelne Messungen, die scheitern, bleiben leer, das Paket geht trotzdem raus.
func (c *Collector) Collect(ctx context.Context) model.Sample {
	sm := model.Sample{OS: runtime.GOOS, Version: c.Version}
	sm.Host, _ = os.Hostname()

	if p, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(p) == 1 {
		sm.CPU = clamp(p[0])
	}
	if runtime.GOOS != "windows" {
		if l, err := load.AvgWithContext(ctx); err == nil {
			v := l.Load1
			sm.Load1 = &v
		}
	}
	if m, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		sm.Mem, sm.MemUsed, sm.MemTotal = clamp(m.UsedPercent), m.Used, m.Total
	}
	if s, err := mem.SwapMemoryWithContext(ctx); err == nil {
		sm.Swap = clamp(s.UsedPercent)
	}
	if d, err := disk.UsageWithContext(ctx, mainDisk()); err == nil {
		sm.Disk, sm.DiskUsed, sm.DiskTotal = clamp(d.UsedPercent), d.Used, d.Total
	}
	sm.NetRx, sm.NetTx = c.netRates(ctx)
	if u, err := host.UptimeWithContext(ctx); err == nil {
		sm.Uptime = u
	}
	sm.Temp = cpuTemp(ctx)
	sm.CIJobs = c.runnerJobs(ctx)
	sm.PowerWarn = powerWarn(ctx)
	return sm
}

// mainDisk ist das Laufwerk, dessen Füllstand angezeigt wird.
func mainDisk() string {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("SystemDrive"); d != "" {
			return d + `\`
		}
		return `C:\`
	}
	return "/"
}

func (c *Collector) netRates(ctx context.Context) (rx, tx float64) {
	st, err := net.IOCountersWithContext(ctx, false)
	if err != nil || len(st) == 0 {
		return 0, 0
	}
	now := time.Now()
	r, t := st[0].BytesRecv, st[0].BytesSent
	if !c.lastNet.IsZero() && r >= c.lastRx && t >= c.lastTx {
		if dt := now.Sub(c.lastNet).Seconds(); dt > 0 {
			rx, tx = float64(r-c.lastRx)/dt, float64(t-c.lastTx)/dt
		}
	}
	c.lastRx, c.lastTx, c.lastNet = r, t, now
	return rx, tx
}

// cpuKeys sind Sensornamen, die zur Prozessortemperatur gehören.
var cpuKeys = []string{"coretemp", "k10temp", "cpu_thermal", "cpu-thermal", "cpu", "soc", "tctl", "package", "acpitz"}

// cpuTemp gibt die Prozessortemperatur zurück oder nil, wenn das System keine liefert.
func cpuTemp(ctx context.Context) *float64 {
	ts, _ := sensors.TemperaturesWithContext(ctx) // Teilergebnisse sind auch bei Fehlern möglich
	return pickTemp(ts)
}

func pickTemp(ts []sensors.TemperatureStat) *float64 {
	var best, any float64
	var found, foundAny bool
	for _, t := range ts {
		v := t.Temperature
		if v <= 0 || v > 150 {
			continue
		}
		if !foundAny || v > any {
			any, foundAny = v, true
		}
		k := strings.ToLower(t.SensorKey)
		for _, key := range cpuKeys {
			if strings.Contains(k, key) {
				if !found || v > best {
					best, found = v, true
				}
				break
			}
		}
	}
	switch {
	case found:
		return &best
	case foundAny:
		return &any
	}
	return nil
}

// runnerJobs zählt laufende Jobs eines GitHub-Runners. Dafür reicht die Zahl der Prozesse "Runner.Worker".
func (c *Collector) runnerJobs(ctx context.Context) int {
	if time.Since(c.jobsAt) < jobsEvery {
		return c.jobs
	}
	c.jobsAt = time.Now()
	n := 0
	if ps, err := process.ProcessesWithContext(ctx); err == nil {
		for _, p := range ps {
			if name, err := p.NameWithContext(ctx); err == nil && isRunnerWorker(name) {
				n++
			}
		}
	}
	c.jobs = n
	return n
}

func isRunnerWorker(name string) bool {
	return strings.EqualFold(strings.TrimSuffix(name, ".exe"), "Runner.Worker")
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
