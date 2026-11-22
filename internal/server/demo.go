package server

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/toolwerk/rackmon/internal/model"
)

// DemoConfig liefert eine Konfiguration mit acht erfundenen Rechnern für `rackmon server -demo`.
func DemoConfig() *Config {
	c := &Config{Listen: "127.0.0.1:8470"}
	roles := []struct{ id, name, role string }{
		{"mac-1", "mac-1", "Mac mini · macOS"},
		{"win-1", "win-1", "Mini-PC · Windows"},
		{"lin-1", "lin-1", "Mini-PC · Linux"},
		{"pve-1", "pve-1", "Proxmox · Debian"},
		{"pi-1", "pi-1", "Raspberry Pi"},
		{"pi-2", "pi-2", "Raspberry Pi"},
		{"pi-3", "pi-3", "Raspberry Pi"},
		{"pi-4", "pi-4", "Raspberry Pi"},
	}
	for _, r := range roles {
		c.Hosts = append(c.Hosts, HostCfg{ID: r.id, Name: r.name, Role: r.role, Token: "demo-token-" + r.id + "-not-used-for-anything"})
	}
	if err := c.Normalize(); err != nil {
		panic(err)
	}
	return c
}

type demoHost struct {
	id          string
	os          string
	cpu, temp   float64
	mem, disk   float64
	memTotalGB  float64
	diskTotalGB float64
	jobs        int
	powerWarn   bool
	silent      bool // sendet nie, erscheint als offline
	noTemp      bool
}

// RunDemo füttert den Speicher mit erfundenen Werten, bis ctx endet.
func RunDemo(ctx context.Context, st *Store) {
	hosts := []*demoHost{
		{id: "mac-1", os: "darwin", cpu: 34, temp: 0, mem: 61, disk: 38, memTotalGB: 16, diskTotalGB: 512, jobs: 3, noTemp: true},
		{id: "win-1", os: "windows", cpu: 9, temp: 0, mem: 28, disk: 21, memTotalGB: 16, diskTotalGB: 1000, jobs: 1, noTemp: true},
		{id: "lin-1", os: "linux", cpu: 72, temp: 63, mem: 36, disk: 14, memTotalGB: 16, diskTotalGB: 1000, jobs: 2},
		{id: "pve-1", os: "linux", cpu: 22, temp: 49, mem: 71, disk: 45, memTotalGB: 32, diskTotalGB: 2000},
		{id: "pi-1", os: "linux", cpu: 12, temp: 54, mem: 34, disk: 27, memTotalGB: 4, diskTotalGB: 64},
		{id: "pi-2", os: "linux", cpu: 8, temp: 49, mem: 22, disk: 19, memTotalGB: 4, diskTotalGB: 64},
		{id: "pi-3", os: "linux", cpu: 58, temp: 72, mem: 63, disk: 52, memTotalGB: 4, diskTotalGB: 64, powerWarn: true},
		{id: "pi-4", os: "linux", silent: true},
	}
	rng := rand.New(rand.NewSource(7))
	walk := func(v, amp, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v+(rng.Float64()-0.5)*amp)) }
	for _, h := range hosts {
		if h.silent {
			continue
		}
		var cpuH, tempH []float64
		c, tp := h.cpu, h.temp
		for i := 0; i < histPoints; i++ {
			c = walk(c, 18, 2, 98)
			cpuH = append(cpuH, c)
			if !h.noTemp {
				tp = walk(tp, 3, 38, 88)
				tempH = append(tempH, tp)
			}
		}
		st.Seed(h.id, cpuH, tempH)
	}
	start := time.Now()
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		for _, h := range hosts {
			if h.silent {
				continue
			}
			h.cpu = walk(h.cpu, 16, 2, 98)
			sm := model.Sample{OS: h.os, Host: h.id, CPU: h.cpu, Mem: walk(h.mem, 1, 5, 95), Disk: h.disk, CIJobs: h.jobs,
				Uptime: uint64(time.Since(start).Seconds()) + 86400*uint64(3+len(h.id)),
				NetRx:  float64(rng.Intn(1_500_000)), NetTx: float64(rng.Intn(90_000))}
			sm.MemTotal = uint64(h.memTotalGB * (1 << 30))
			sm.MemUsed = uint64(sm.Mem / 100 * float64(sm.MemTotal))
			sm.DiskTotal = uint64(h.diskTotalGB * (1 << 30))
			sm.DiskUsed = uint64(sm.Disk / 100 * float64(sm.DiskTotal))
			if !h.noTemp {
				h.temp = walk(h.temp, 3, 38, 88)
				v := h.temp
				sm.Temp = &v
			}
			if h.powerWarn {
				b := true
				sm.PowerWarn = &b
			}
			st.Update(h.id, sm)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
