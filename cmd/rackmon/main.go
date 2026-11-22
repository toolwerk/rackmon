// Rackmon: Agent und Server in einem Programm.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/toolwerk/rackmon/internal/agent"
	"github.com/toolwerk/rackmon/internal/server"
)

// version wird beim Bauen gesetzt (-ldflags "-X main.version=...").
var version = "dev"

const usage = `Rackmon %s

Aufruf:
  rackmon agent  -server URL -token TOKEN [-interval 2s]
  rackmon server -config datei.yaml [-listen adresse]
  rackmon server -demo [-listen adresse]
  rackmon top    [-server URL] [-interval 2s] [-once]
  rackmon token
  rackmon version

Einstellungen des Agenten können auch über RACKMON_SERVER, RACKMON_TOKEN und
RACKMON_INTERVAL kommen. Das Token gehört nicht auf die Kommandozeile, wenn andere
Benutzer die Prozessliste sehen können: dann die Umgebung oder -token-file nehmen.
`

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "agent":
		err = runAgentCmd(os.Args[2:])
	case "server":
		err = runServerCmd(os.Args[2:])
	case "top":
		err = runTopCmd(os.Args[2:])
	case "token":
		err = printToken()
	case "version", "-v", "--version":
		fmt.Println("rackmon", version)
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, "unbekannter Befehl %q\n\n", os.Args[1])
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	if err != nil {
		log.Fatalf("Fehler: %v", err)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func runAgentCmd(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	srv := fs.String("server", envOr("RACKMON_SERVER", ""), "Adresse des Servers, zum Beispiel http://server:8470")
	tok := fs.String("token", os.Getenv("RACKMON_TOKEN"), "Token dieses Rechners")
	tokFile := fs.String("token-file", "", "Datei, die das Token enthält")
	iv := fs.Duration("interval", 0, "Abstand der Pakete (Vorgabe 2s)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *tokFile != "" {
		b, err := os.ReadFile(*tokFile)
		if err != nil {
			return err
		}
		*tok = trimSpace(string(b))
	}
	if *iv == 0 {
		if v := os.Getenv("RACKMON_INTERVAL"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("RACKMON_INTERVAL: %w", err)
			}
			*iv = d
		}
	}
	cfg := agent.Config{Server: *srv, Token: *tok, Interval: *iv, Version: version, Logf: log.Printf}
	if err := cfg.Validate(); err != nil {
		return err
	}
	return runAgentOrService(cfg)
}

func runServerCmd(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Konfigurationsdatei (YAML)")
	listen := fs.String("listen", "", "Adresse, überschreibt die Konfiguration")
	demo := fs.Bool("demo", false, "erfundene Werte anzeigen, keine Konfiguration nötig")
	console := fs.Duration("console", 0, "alle empfangenen Werte in diesem Abstand als Text ausgeben (zum Beispiel 10s)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var cfg *server.Config
	switch {
	case *demo:
		cfg = server.DemoConfig()
	case *cfgPath != "":
		c, err := server.LoadConfig(*cfgPath)
		if err != nil {
			return err
		}
		cfg = c
	default:
		return errors.New("-config oder -demo angeben")
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	ctx, stop := signalContext()
	defer stop()
	st := server.NewStore(cfg)
	if *demo {
		go server.RunDemo(ctx, st)
		log.Printf("Demo-Modus: erfundene Werte")
	}
	srv := server.New(cfg, st)
	if *console == 0 {
		*console = cfg.ConsoleInterval
	}
	if *console > 0 {
		go srv.ConsoleLoop(ctx, os.Stdout, *console)
	}
	log.Printf("Rackmon %s hört auf http://%s (Bildschirm: /screen/1)", version, cfg.Listen)
	return srv.ListenAndServe(ctx)
}

func printToken() error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	fmt.Println(base64.RawURLEncoding.EncodeToString(b))
	return nil
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}

// runTopCmd zeigt den Stand eines laufenden Servers als Text, ohne Graphen. Wie `top`, nur für alle Rechner.
func runTopCmd(args []string) error {
	fs := flag.NewFlagSet("top", flag.ContinueOnError)
	srv := fs.String("server", envOr("RACKMON_SERVER", "http://127.0.0.1:8470"), "Adresse des Servers")
	iv := fs.Duration("interval", 2*time.Second, "Abstand der Aktualisierung")
	once := fs.Bool("once", false, "einmal ausgeben und beenden")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signalContext()
	defer stop()
	client := &http.Client{Timeout: 5 * time.Second}
	for {
		snap, err := fetchState(ctx, client, strings.TrimRight(*srv, "/"))
		if err != nil {
			return err
		}
		if !*once {
			fmt.Print("\033[H\033[2J") // Bildschirm leeren
		}
		fmt.Printf("Rackmon %s, %s\n\n%s", *srv, time.Now().Format("2006-01-02 15:04:05"), server.RenderText(snap))
		if *once {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(*iv):
		}
	}
}

func fetchState(ctx context.Context, c *http.Client, base string) (server.Snapshot, error) {
	var snap server.Snapshot
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/state", nil)
	if err != nil {
		return snap, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return snap, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return snap, fmt.Errorf("Server antwortet mit %s", resp.Status)
	}
	return snap, json.NewDecoder(resp.Body).Decode(&snap)
}
