package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/toolwerk/rackmon/internal/model"
	"github.com/toolwerk/rackmon/internal/web"
)

const maxBody = 32 << 10 // 32 KB reichen für ein Paket

// Server verbindet Speicher und HTTP.
type Server struct {
	cfg    *Config
	store  *Store
	tokens map[string]string // Hash des Tokens -> Rechner-ID
	logf   func(string, ...any)

	mu       sync.Mutex
	lastWarn time.Time
}

// New baut den Server aus einer geprüften Konfiguration.
func New(cfg *Config, store *Store) *Server {
	s := &Server{cfg: cfg, store: store, tokens: map[string]string{}, logf: log.Printf}
	for _, h := range cfg.Hosts {
		s.tokens[hashToken(h.Token)] = h.ID
	}
	return s
}

// Handler liefert alle Routen.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ingest", s.ingest)
	mux.HandleFunc("GET /api/state", s.apiState)
	mux.HandleFunc("GET /events", s.events)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok\n") })
	static, _ := fs.Sub(web.Files, "static")
	files := http.FileServer(http.FS(static))
	mux.Handle("GET /static/", http.StripPrefix("/static/", files))
	mux.HandleFunc("GET /screen/{n}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := strconv.Atoi(r.PathValue("n")); err != nil {
			http.NotFound(w, r)
			return
		}
		serveIndex(w)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/screen/1", http.StatusFound)
	})
	return securityHeaders(mux)
}

func serveIndex(w http.ResponseWriter) {
	b, err := web.Files.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index fehlt", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// bearer liest das Token aus dem Authorization-Kopf.
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) > len(p) && strings.EqualFold(h[:len(p)], p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

func (s *Server) hostForToken(tok string) (string, bool) {
	if tok == "" {
		return "", false
	}
	id, ok := s.tokens[hashToken(tok)]
	if !ok {
		return "", false
	}
	// Zweite, zeitkonstante Prüfung gegen das echte Token.
	for _, h := range s.cfg.Hosts {
		if h.ID == id && subtle.ConstantTimeCompare([]byte(h.Token), []byte(tok)) == 1 {
			return id, true
		}
	}
	return "", false
}

func (s *Server) warnLimited(format string, a ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastWarn) < 30*time.Second {
		return
	}
	s.lastWarn = time.Now()
	s.logf(format, a...)
}

func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	id, ok := s.hostForToken(bearer(r))
	if !ok {
		s.warnLimited("ingest: ungültiges Token von %s", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	var sm model.Sample
	if err := json.NewDecoder(r.Body).Decode(&sm); err != nil {
		http.Error(w, "ungültiges JSON", http.StatusBadRequest)
		return
	}
	if err := sm.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.store.Update(id, sm)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(s.store.Snapshot())
}

// events schickt jede Sekunde den ganzen Stand als Server-Sent Event.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming nicht möglich", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "keep-alive")
	send := func() error {
		b, err := json.Marshal(s.store.Snapshot())
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		fl.Flush()
		return nil
	}
	if send() != nil {
		return
	}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
			if send() != nil {
				return
			}
		}
	}
}

// ListenAndServe startet den Server auf allen Adressen aus Listen (durch Komma getrennt)
// und beendet ihn sauber, wenn ctx endet. Scheitert eine Adresse, bricht alles ab.
func (s *Server) ListenAndServe(ctx context.Context) error {
	var srvs []*http.Server
	errc := make(chan error, 8)
	for _, addr := range strings.Split(s.cfg.Listen, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		srv := &http.Server{
			Addr:              addr,
			Handler:           s.Handler(),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		srvs = append(srvs, srv)
		go func() { errc <- srv.ListenAndServe() }()
	}
	if len(srvs) == 0 {
		return errors.New("listen ist leer")
	}
	var first error
	select {
	case first = <-errc:
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, srv := range srvs {
		srv.Shutdown(sctx)
	}
	if errors.Is(first, http.ErrServerClosed) {
		return nil
	}
	return first
}
