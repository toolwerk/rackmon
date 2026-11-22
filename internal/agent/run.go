package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config sind die Einstellungen eines Agenten.
type Config struct {
	Server   string        // Adresse des Servers, zum Beispiel http://server:8470
	Token    string        // Token dieses Rechners
	Interval time.Duration // Abstand der Pakete
	Version  string
	Logf     func(string, ...any)
}

// Validate prüft die Einstellungen und setzt Vorgaben.
func (c *Config) Validate() error {
	if c.Server == "" {
		return errors.New("Server-Adresse fehlt (-server oder RACKMON_SERVER)")
	}
	u, err := url.Parse(c.Server)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("Server-Adresse %q ist keine gültige http(s)-Adresse", c.Server)
	}
	if c.Token == "" {
		return errors.New("Token fehlt (-token oder RACKMON_TOKEN)")
	}
	if c.Interval == 0 {
		c.Interval = 2 * time.Second
	}
	if c.Interval < time.Second {
		return errors.New("Takt muss mindestens 1 Sekunde sein")
	}
	if c.Logf == nil {
		c.Logf = func(string, ...any) {}
	}
	c.Server = strings.TrimRight(c.Server, "/")
	return nil
}

// Run misst und sendet, bis ctx endet. Scheitert das Senden, geht der Agent einfach beim nächsten Takt weiter.
// Es gibt keine Warteschlange: Die Anzeige will den aktuellen Stand, keine alten Werte.
func Run(ctx context.Context, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	col := &Collector{Version: c.Version}
	client := &http.Client{Timeout: 5 * time.Second}
	col.Collect(ctx) // erste Messung nur zum Aufwärmen der Zähler
	t := time.NewTicker(c.Interval)
	defer t.Stop()
	var failing bool
	var lastLog time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		sm := col.Collect(ctx)
		body, _ := json.Marshal(sm)
		err := post(ctx, client, c.Server+"/ingest", c.Token, body)
		switch {
		case err != nil && (!failing || time.Since(lastLog) > time.Minute):
			c.Logf("senden fehlgeschlagen: %v", err)
			failing, lastLog = true, time.Now()
		case err == nil && failing:
			c.Logf("Verbindung zum Server wieder da")
			failing = false
		}
	}
}

func post(ctx context.Context, cl *http.Client, url, token string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Server antwortet mit %s", resp.Status)
	}
	return nil
}
