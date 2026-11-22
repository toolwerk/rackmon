// Package server nimmt Messpakete an, hält den letzten Stand und liefert das Dashboard aus.
package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// HostCfg beschreibt einen überwachten Rechner.
type HostCfg struct {
	ID    string `yaml:"id" json:"id"`
	Name  string `yaml:"name" json:"name"`
	Role  string `yaml:"role" json:"role"`
	Token string `yaml:"token" json:"-"`
}

// ScreenCfg beschreibt einen Bildschirm: welche Rechner in welcher Reihenfolge, und wie schnell geblättert wird.
type ScreenCfg struct {
	ID          int      `yaml:"id" json:"id"`
	Hosts       []string `yaml:"hosts" json:"hosts"`
	PerPage     int      `yaml:"per_page" json:"per_page"`
	PageSeconds int      `yaml:"page_seconds" json:"page_seconds"`
}

// Thresholds sind die Grenzen für die Farben im Dashboard.
type Thresholds struct {
	Warn     float64 `yaml:"warn" json:"warn"`           // Prozent, ab hier gelb
	Crit     float64 `yaml:"crit" json:"crit"`           // Prozent, ab hier rot
	TempWarn float64 `yaml:"temp_warn" json:"temp_warn"` // Grad
	TempCrit float64 `yaml:"temp_crit" json:"temp_crit"`
}

// Config ist die Serverkonfiguration.
type Config struct {
	Listen string `yaml:"listen"`
	// ConsoleInterval: alle empfangenen Werte in diesem Abstand als Text in die Standardausgabe schreiben (0 = aus).
	ConsoleInterval time.Duration `yaml:"console_interval"`
	StaleAfter      time.Duration `yaml:"stale_after"`
	OfflineAfter    time.Duration `yaml:"offline_after"`
	Hosts           []HostCfg     `yaml:"hosts"`
	Screens         []ScreenCfg   `yaml:"screens"`
	Thresholds      Thresholds    `yaml:"thresholds"`
}

// LoadConfig liest eine Konfigurationsdatei, setzt Vorgaben und prüft sie.
func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.Normalize(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

// MinTokenLen ist die kürzeste erlaubte Tokenlänge.
const MinTokenLen = 24

// Normalize setzt Vorgaben und prüft die Konfiguration.
func (c *Config) Normalize() error {
	if c.Listen == "" {
		c.Listen = "127.0.0.1:8470"
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = 6 * time.Second
	}
	if c.OfflineAfter <= 0 {
		c.OfflineAfter = 30 * time.Second
	}
	if c.ConsoleInterval < 0 || (c.ConsoleInterval > 0 && c.ConsoleInterval < time.Second) {
		return errors.New("console_interval muss 0 oder mindestens 1s sein")
	}
	if c.OfflineAfter <= c.StaleAfter {
		return errors.New("offline_after muss größer als stale_after sein")
	}
	t := &c.Thresholds
	if t.Warn == 0 {
		t.Warn = 70
	}
	if t.Crit == 0 {
		t.Crit = 90
	}
	if t.TempWarn == 0 {
		t.TempWarn = 70
	}
	if t.TempCrit == 0 {
		t.TempCrit = 85
	}
	if len(c.Hosts) == 0 {
		return errors.New("keine Rechner (hosts) eingetragen")
	}
	ids := map[string]bool{}
	tokens := map[string]bool{}
	for i := range c.Hosts {
		h := &c.Hosts[i]
		if h.ID == "" || strings.ContainsAny(h.ID, " /?#") {
			return fmt.Errorf("Rechner %d: id fehlt oder enthält ungültige Zeichen", i+1)
		}
		if ids[h.ID] {
			return fmt.Errorf("Rechner %q kommt doppelt vor", h.ID)
		}
		ids[h.ID] = true
		if h.Name == "" {
			h.Name = h.ID
		}
		h.Token = os.ExpandEnv(h.Token)
		if len(h.Token) < MinTokenLen {
			return fmt.Errorf("Rechner %q: token zu kurz (mindestens %d Zeichen, erzeugen mit: rackmon token)", h.ID, MinTokenLen)
		}
		if tokens[h.Token] {
			return fmt.Errorf("Rechner %q: token wird schon von einem anderen Rechner genutzt", h.ID)
		}
		tokens[h.Token] = true
	}
	if len(c.Screens) == 0 {
		s := ScreenCfg{ID: 1}
		for _, h := range c.Hosts {
			s.Hosts = append(s.Hosts, h.ID)
		}
		c.Screens = []ScreenCfg{s}
	}
	seen := map[int]bool{}
	for i := range c.Screens {
		s := &c.Screens[i]
		if s.ID == 0 {
			s.ID = i + 1
		}
		if seen[s.ID] {
			return fmt.Errorf("Bildschirm %d kommt doppelt vor", s.ID)
		}
		seen[s.ID] = true
		if s.PerPage == 0 {
			s.PerPage = 4
		}
		if s.PerPage < 1 || s.PerPage > 8 {
			return fmt.Errorf("Bildschirm %d: per_page muss 1 bis 8 sein", s.ID)
		}
		if s.PageSeconds == 0 {
			s.PageSeconds = 8
		}
		if s.PageSeconds < 2 {
			return fmt.Errorf("Bildschirm %d: page_seconds muss mindestens 2 sein", s.ID)
		}
		if len(s.Hosts) == 0 {
			return fmt.Errorf("Bildschirm %d: keine Rechner", s.ID)
		}
		for _, id := range s.Hosts {
			if !ids[id] {
				return fmt.Errorf("Bildschirm %d: unbekannter Rechner %q", s.ID, id)
			}
		}
	}
	return nil
}

// hashToken liefert den Schlüssel, unter dem ein Token abgelegt wird. So hängt die Suchzeit
// nicht vom Inhalt des Tokens ab.
func hashToken(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:])
}
