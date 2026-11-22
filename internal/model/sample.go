// Package model enthält das Messpaket, das jeder Agent an den Server schickt.
package model

import (
	"errors"
	"fmt"
	"math"
)

// Sample ist ein Messpaket eines Rechners. Felder, die ein Rechner nicht liefern kann,
// sind Zeiger und bleiben dann leer. Das Dashboard zeigt dafür einen Strich.
type Sample struct {
	Host      string   `json:"host,omitempty"` // Rechnername laut Agent, nur zur Kontrolle
	OS        string   `json:"os"`             // linux, darwin, windows
	Version   string   `json:"version,omitempty"`
	TS        int64    `json:"ts"`  // Sekunden seit 1970, der Server überschreibt es mit seiner Uhr
	CPU       float64  `json:"cpu"` // Prozent
	Load1     *float64 `json:"load1,omitempty"`
	Mem       float64  `json:"mem"` // Prozent belegt
	MemUsed   uint64   `json:"mem_used"`
	MemTotal  uint64   `json:"mem_total"`
	Swap      float64  `json:"swap"` // Prozent
	Disk      float64  `json:"disk"` // Prozent des Hauptlaufwerks
	DiskUsed  uint64   `json:"disk_used"`
	DiskTotal uint64   `json:"disk_total"`
	NetRx     float64  `json:"net_rx"` // Bytes pro Sekunde
	NetTx     float64  `json:"net_tx"`
	Uptime    uint64   `json:"uptime"`               // Sekunden
	Temp      *float64 `json:"temp,omitempty"`       // Grad Celsius
	CIJobs    int      `json:"ci_jobs"`              // laufende Jobs eines GitHub-Runners
	PowerWarn *bool    `json:"power_warn,omitempty"` // Raspberry Pi: Unterspannung oder Drosselung
}

// Validate prüft ein Paket aus dem Netz auf plausible Werte. Der Server vertraut keinem Agenten blind.
func (s *Sample) Validate() error {
	for name, v := range map[string]float64{"cpu": s.CPU, "mem": s.Mem, "swap": s.Swap, "disk": s.Disk} {
		if bad(v) || v < 0 || v > 100 {
			return fmt.Errorf("%s außerhalb 0 bis 100", name)
		}
	}
	for name, v := range map[string]float64{"net_rx": s.NetRx, "net_tx": s.NetTx} {
		if bad(v) || v < 0 {
			return fmt.Errorf("%s ungültig", name)
		}
	}
	if s.Temp != nil && (bad(*s.Temp) || *s.Temp < -50 || *s.Temp > 200) {
		return errors.New("temp außerhalb -50 bis 200")
	}
	if s.Load1 != nil && (bad(*s.Load1) || *s.Load1 < 0) {
		return errors.New("load1 ungültig")
	}
	if s.CIJobs < 0 || s.CIJobs > 1000 {
		return errors.New("ci_jobs ungültig")
	}
	if len(s.OS) > 16 || len(s.Host) > 128 || len(s.Version) > 64 {
		return errors.New("Text zu lang")
	}
	return nil
}

func bad(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) }
