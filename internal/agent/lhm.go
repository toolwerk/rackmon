package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Unter Windows gibt es keine eingebaute Prozessortemperatur. Wer LibreHardwareMonitor mit seinem
// Web-Server laufen lässt, kann dessen Seite data.json als Quelle angeben (-temp-url). Der Agent liest nur
// vom eigenen Rechner: Die Adresse muss auf localhost zeigen.

// validateTempURL erlaubt nur Adressen auf diesem Rechner.
func validateTempURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("Temperatur-Adresse %q ist keine gültige http(s)-Adresse", s)
	}
	h := u.Hostname()
	if h != "localhost" {
		if ip := net.ParseIP(h); ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("Temperatur-Adresse muss auf diesen Rechner zeigen (localhost), nicht auf %q", h)
		}
	}
	return nil
}

type lhmNode struct {
	Text     string    `json:"Text"`
	Value    string    `json:"Value"`
	Type     string    `json:"Type"`
	ImageURL string    `json:"ImageURL"`
	Children []lhmNode `json:"Children"`
}

// fetchLHM holt die Prozessortemperatur aus data.json von LibreHardwareMonitor. nil, wenn nichts da ist.
func fetchLHM(ctx context.Context, cl *http.Client, addr string) (*float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Temperaturquelle antwortet mit %s", resp.Status)
	}
	var root lhmNode
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&root); err != nil {
		return nil, fmt.Errorf("Temperaturquelle: %w", err)
	}
	return pickLHM(root), nil
}

// pickLHM wählt aus dem Sensorbaum die Prozessortemperatur: bevorzugt Tctl/Package, sonst den heißesten Kern.
func pickLHM(root lhmNode) *float64 {
	var preferred, hottest float64
	var havePref, haveAny bool
	var walk func(n lhmNode, inCPU bool)
	walk = func(n lhmNode, inCPU bool) {
		if strings.Contains(strings.ToLower(n.ImageURL), "cpu") {
			inCPU = true
		}
		isTemp := n.Type == "Temperature" || strings.Contains(strings.ToLower(n.ImageURL), "temperature")
		if inCPU && isTemp && len(n.Children) == 0 {
			if v, ok := parseLHMValue(n.Value); ok && v > 0 && v < 150 {
				name := strings.ToLower(n.Text)
				if strings.Contains(name, "tctl") || strings.Contains(name, "package") {
					if !havePref || v > preferred {
						preferred, havePref = v, true
					}
				}
				if !haveAny || v > hottest {
					hottest, haveAny = v, true
				}
			}
		}
		for _, c := range n.Children {
			walk(c, inCPU)
		}
	}
	walk(root, false)
	switch {
	case havePref:
		return &preferred
	case haveAny:
		return &hottest
	}
	return nil
}

// parseLHMValue liest Werte wie "52,5 °C" oder "52.5 °C". Nur die Zahl am Anfang zählt, die Einheit wird ignoriert.
func parseLHMValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && (s[end] == ',' || s[end] == '.' || s[end] == '-' || (s[end] >= '0' && s[end] <= '9')) {
		end++
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s[:end], ",", "."), 64)
	return v, err == nil
}

var lhmClient = &http.Client{Timeout: 1500 * time.Millisecond}
