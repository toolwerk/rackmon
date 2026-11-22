package server

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// RenderText stellt den Stand aller Rechner als reine ASCII-Tabelle dar, ohne Graphen. Nur die Werte.
func RenderText(snap Snapshot) string {
	var b strings.Builder
	rows := [][]string{{"HOST", "STATE", "CPU", "LOAD", "MEM", "MEM-USED", "SWAP", "DISK", "TEMP", "RX", "TX", "JOBS", "POWER", "UPTIME", "SEEN"}}
	for _, h := range snap.Hosts {
		s := h.Sample
		if s == nil {
			rows = append(rows, append([]string{h.ID, h.State}, dashes(13)...))
			continue
		}
		load, temp, power := "-", "-", "-"
		if s.Load1 != nil {
			load = fmt.Sprintf("%.2f", *s.Load1)
		}
		if s.Temp != nil {
			temp = fmt.Sprintf("%.0fC", *s.Temp)
		}
		if s.PowerWarn != nil {
			power = "ok"
			if *s.PowerWarn {
				power = "WARN"
			}
		}
		rows = append(rows, []string{
			h.ID, h.State,
			fmt.Sprintf("%.0f%%", s.CPU), load,
			fmt.Sprintf("%.0f%%", s.Mem), gib(s.MemUsed) + "/" + gib(s.MemTotal),
			fmt.Sprintf("%.0f%%", s.Swap), fmt.Sprintf("%.0f%%", s.Disk), temp,
			rate(s.NetRx), rate(s.NetTx),
			fmt.Sprintf("%d", s.CIJobs), power, uptime(s.Uptime), ago(snap.Now - h.LastSeen),
		})
	}
	width := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			if len(c) > width[i] {
				width[i] = len(c)
			}
		}
	}
	for _, r := range rows {
		for i, c := range r {
			if i > 0 {
				b.WriteString("  ")
			}
			fmt.Fprintf(&b, "%-*s", width[i], c)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func dashes(n int) []string {
	d := make([]string, n)
	for i := range d {
		d[i] = "-"
	}
	return d
}

func gib(b uint64) string { return fmt.Sprintf("%.1fG", float64(b)/(1<<30)) }

func rate(b float64) string {
	switch {
	case b < 1000:
		return fmt.Sprintf("%.0fB/s", b)
	case b < 1e6:
		return fmt.Sprintf("%.0fKB/s", b/1024)
	}
	return fmt.Sprintf("%.1fMB/s", b/1048576)
}

func uptime(s uint64) string {
	d, h, m := s/86400, s%86400/3600, s%3600/60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd%dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func ago(sec int64) string {
	switch {
	case sec < 0:
		return "0s"
	case sec < 120:
		return fmt.Sprintf("%ds", sec)
	case sec < 7200:
		return fmt.Sprintf("%dm", sec/60)
	}
	return fmt.Sprintf("%dh", sec/3600)
}

// ConsoleLoop schreibt den Stand im Abstand every als Textblock nach w, bis ctx endet.
func (s *Server) ConsoleLoop(ctx context.Context, w io.Writer, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fmt.Fprintf(w, "--- %s ---\n%s", time.Now().Format("2006-01-02 15:04:05"), RenderText(s.store.Snapshot()))
		}
	}
}
