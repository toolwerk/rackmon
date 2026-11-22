package server

import (
	"sync"
	"time"

	"github.com/toolwerk/rackmon/internal/model"
)

// Zustände eines Rechners.
const (
	StateOK      = "ok"      // Paket ist frisch
	StateStale   = "stale"   // eine Weile nichts gehört
	StateOffline = "offline" // länger nichts gehört oder noch nie
)

const (
	histPoints = 60              // Punkte pro Verlauf
	histEvery  = 5 * time.Second // ein Punkt alle 5 Sekunden, das sind 5 Minuten
)

type hostState struct {
	cfg      HostCfg
	last     *model.Sample
	lastSeen time.Time
	lastHist time.Time
	cpuHist  []float64
	tempHist []float64
}

// Store hält den letzten Stand und den kurzen Verlauf aller Rechner im Arbeitsspeicher.
type Store struct {
	mu    sync.RWMutex
	cfg   *Config
	hosts map[string]*hostState
	now   func() time.Time
}

// NewStore legt für jeden konfigurierten Rechner einen leeren Eintrag an.
func NewStore(cfg *Config) *Store {
	s := &Store{cfg: cfg, hosts: map[string]*hostState{}, now: time.Now}
	for _, h := range cfg.Hosts {
		s.hosts[h.ID] = &hostState{cfg: h}
	}
	return s
}

// Update trägt ein Paket ein. Die Uhr des Agenten zählt nicht, der Server setzt seine eigene Zeit.
func (s *Store) Update(id string, sm model.Sample) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.hosts[id]
	if !ok {
		return false
	}
	now := s.now()
	sm.TS = now.Unix()
	h.last = &sm
	h.lastSeen = now
	if now.Sub(h.lastHist) >= histEvery {
		h.lastHist = now
		h.cpuHist = appendCap(h.cpuHist, sm.CPU)
		if sm.Temp != nil {
			h.tempHist = appendCap(h.tempHist, *sm.Temp)
		}
	}
	return true
}

func appendCap(a []float64, v float64) []float64 {
	a = append(a, v)
	if len(a) > histPoints {
		a = a[len(a)-histPoints:]
	}
	return a
}

// HostView ist die Sicht eines Rechners für das Dashboard.
type HostView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Role     string        `json:"role"`
	State    string        `json:"state"`
	LastSeen int64         `json:"last_seen"` // 0 = noch nie
	Sample   *model.Sample `json:"sample,omitempty"`
	CPUHist  []float64     `json:"cpu_hist"`
	TempHist []float64     `json:"temp_hist"`
}

// Snapshot ist der gesamte Stand, den das Dashboard sekündlich bekommt.
type Snapshot struct {
	Now        int64       `json:"now"`
	Hosts      []HostView  `json:"hosts"`
	Screens    []ScreenCfg `json:"screens"`
	Thresholds Thresholds  `json:"thresholds"`
}

func (s *Store) state(h *hostState, now time.Time) string {
	if h.last == nil {
		return StateOffline
	}
	age := now.Sub(h.lastSeen)
	switch {
	case age <= s.cfg.StaleAfter:
		return StateOK
	case age <= s.cfg.OfflineAfter:
		return StateStale
	}
	return StateOffline
}

// Snapshot liefert eine Kopie des Stands, die der Aufrufer frei lesen darf.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	out := Snapshot{Now: now.Unix(), Screens: s.cfg.Screens, Thresholds: s.cfg.Thresholds}
	for _, c := range s.cfg.Hosts {
		h := s.hosts[c.ID]
		v := HostView{ID: c.ID, Name: c.Name, Role: c.Role, State: s.state(h, now),
			CPUHist: append([]float64{}, h.cpuHist...), TempHist: append([]float64{}, h.tempHist...)}
		if h.last != nil {
			cp := *h.last
			v.Sample = &cp
			v.LastSeen = h.lastSeen.Unix()
		}
		out.Hosts = append(out.Hosts, v)
	}
	return out
}

// Seed füllt den Verlauf vor, damit eine Demo sofort Graphen zeigt.
func (s *Store) Seed(id string, cpu, temp []float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.hosts[id]; ok {
		h.cpuHist = append([]float64{}, cpu...)
		h.tempHist = append([]float64{}, temp...)
	}
}
