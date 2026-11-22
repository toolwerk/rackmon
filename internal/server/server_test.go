package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/toolwerk/rackmon/internal/model"
)

const tokA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const tokB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func testConfig(t *testing.T) *Config {
	t.Helper()
	c := &Config{Hosts: []HostCfg{{ID: "a", Name: "A", Token: tokA}, {ID: "b", Token: tokB}}}
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNormalizeDefaults(t *testing.T) {
	c := testConfig(t)
	if c.Listen != "127.0.0.1:8470" || c.StaleAfter != 6*time.Second || c.OfflineAfter != 30*time.Second {
		t.Fatalf("Vorgaben falsch: %+v", c)
	}
	if len(c.Screens) != 1 || len(c.Screens[0].Hosts) != 2 || c.Screens[0].PerPage != 4 {
		t.Fatalf("Standard-Bildschirm falsch: %+v", c.Screens)
	}
	if c.Hosts[1].Name != "b" {
		t.Fatal("Name sollte auf die ID zurückfallen")
	}
}

func TestNormalizeErrors(t *testing.T) {
	cases := map[string]*Config{
		"keine Rechner":   {},
		"Token kurz":      {Hosts: []HostCfg{{ID: "a", Token: "kurz"}}},
		"doppelte ID":     {Hosts: []HostCfg{{ID: "a", Token: tokA}, {ID: "a", Token: tokB}}},
		"doppeltes Token": {Hosts: []HostCfg{{ID: "a", Token: tokA}, {ID: "b", Token: tokA}}},
		"ungültige ID":    {Hosts: []HostCfg{{ID: "a b", Token: tokA}}},
		"unbekannt im Bildschirm": {Hosts: []HostCfg{{ID: "a", Token: tokA}},
			Screens: []ScreenCfg{{Hosts: []string{"x"}}}},
		"per_page": {Hosts: []HostCfg{{ID: "a", Token: tokA}}, Screens: []ScreenCfg{{Hosts: []string{"a"}, PerPage: 9}}},
	}
	for name, c := range cases {
		if err := c.Normalize(); err == nil {
			t.Errorf("%s: Fehler erwartet", name)
		}
	}
}

func TestStateTransitions(t *testing.T) {
	c := testConfig(t)
	st := NewStore(c)
	now := time.Unix(1_000_000, 0)
	st.now = func() time.Time { return now }

	if got := st.Snapshot().Hosts[0].State; got != StateOffline {
		t.Fatalf("ohne Paket: %s", got)
	}
	st.Update("a", model.Sample{CPU: 10})
	for _, tc := range []struct {
		after time.Duration
		want  string
	}{{0, StateOK}, {6 * time.Second, StateOK}, {7 * time.Second, StateStale}, {30 * time.Second, StateStale}, {31 * time.Second, StateOffline}} {
		now = time.Unix(1_000_000, 0).Add(tc.after)
		if got := st.Snapshot().Hosts[0].State; got != tc.want {
			t.Errorf("nach %v: %s, erwartet %s", tc.after, got, tc.want)
		}
	}
}

func TestUpdateUsesServerClockAndHistory(t *testing.T) {
	st := NewStore(testConfig(t))
	now := time.Unix(2_000_000, 0)
	st.now = func() time.Time { return now }
	temp := 50.0
	st.Update("a", model.Sample{TS: 5, CPU: 1, Temp: &temp})
	now = now.Add(2 * time.Second) // zu früh für einen neuen Verlaufspunkt
	st.Update("a", model.Sample{CPU: 2})
	now = now.Add(5 * time.Second)
	st.Update("a", model.Sample{CPU: 3})
	v := st.Snapshot().Hosts[0]
	if v.Sample.TS != now.Unix() {
		t.Errorf("TS = %d, erwartet Serveruhr", v.Sample.TS)
	}
	if len(v.CPUHist) != 2 || v.CPUHist[0] != 1 || v.CPUHist[1] != 3 {
		t.Errorf("Verlauf = %v", v.CPUHist)
	}
	if len(v.TempHist) != 1 {
		t.Errorf("Temperaturverlauf = %v", v.TempHist)
	}
	if st.Update("unbekannt", model.Sample{}) {
		t.Error("unbekannter Rechner darf nicht angenommen werden")
	}
}

func TestHistoryIsCapped(t *testing.T) {
	st := NewStore(testConfig(t))
	now := time.Unix(3_000_000, 0)
	st.now = func() time.Time { return now }
	for i := 0; i < histPoints+20; i++ {
		now = now.Add(histEvery)
		st.Update("a", model.Sample{CPU: float64(i % 100)})
	}
	if n := len(st.Snapshot().Hosts[0].CPUHist); n != histPoints {
		t.Fatalf("Verlauf hat %d Punkte, erwartet %d", n, histPoints)
	}
}

func post(h http.Handler, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/ingest", strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestIngest(t *testing.T) {
	c := testConfig(t)
	st := NewStore(c)
	srv := New(c, st)
	srv.logf = func(string, ...any) {}
	h := srv.Handler()

	if w := post(h, "", `{"os":"linux"}`); w.Code != 401 {
		t.Errorf("ohne Token: %d", w.Code)
	}
	if w := post(h, "falsch", `{"os":"linux"}`); w.Code != 401 {
		t.Errorf("falsches Token: %d", w.Code)
	}
	if w := post(h, tokA, `kein json`); w.Code != 400 {
		t.Errorf("kaputtes JSON: %d", w.Code)
	}
	if w := post(h, tokA, `{"os":"linux","cpu":150}`); w.Code != 400 {
		t.Errorf("unplausibler Wert: %d", w.Code)
	}
	if w := post(h, tokA, `{"os":"linux","cpu":12.5,"mem":40}`); w.Code != 204 {
		t.Fatalf("gültiges Paket: %d", w.Code)
	}
	if got := st.Snapshot().Hosts[0].Sample; got == nil || got.CPU != 12.5 {
		t.Fatalf("Paket nicht gespeichert: %+v", got)
	}
	if st.Snapshot().Hosts[1].Sample != nil {
		t.Fatal("Token A darf nur Rechner A füllen")
	}
	big := `{"os":"` + strings.Repeat("x", maxBody) + `"}`
	if w := post(h, tokA, big); w.Code != 400 {
		t.Errorf("zu großes Paket: %d", w.Code)
	}
}

func TestAPIAndPages(t *testing.T) {
	c := testConfig(t)
	srv := New(c, NewStore(c))
	h := srv.Handler()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/state", nil))
	var snap Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil || len(snap.Hosts) != 2 {
		t.Fatalf("api/state: %v %s", err, w.Body.String())
	}
	if strings.Contains(w.Body.String(), tokA) {
		t.Fatal("Token darf nie in der Ausgabe stehen")
	}
	for path, want := range map[string]int{"/screen/1": 200, "/screen/x": 404, "/static/app.js": 200, "/healthz": 200, "/": 302} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Errorf("%s: %d, erwartet %d", path, w.Code, want)
		}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/screen/1", nil))
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Error("Sicherheitsköpfe fehlen")
	}
}

func TestDemoConfigIsValid(t *testing.T) {
	c := DemoConfig()
	if len(c.Hosts) != 8 || len(c.Screens) != 1 {
		t.Fatalf("Demo: %d Rechner, %d Bildschirme", len(c.Hosts), len(c.Screens))
	}
}

func TestRenderText(t *testing.T) {
	c := testConfig(t)
	st := NewStore(c)
	temp, load, warn := 55.0, 0.5, true
	st.Update("a", model.Sample{OS: "linux", CPU: 12, Mem: 40, MemUsed: 4 << 30, MemTotal: 10 << 30, Disk: 20, NetRx: 2_500_000, NetTx: 800,
		Uptime: 90061, Temp: &temp, Load1: &load, CIJobs: 2, PowerWarn: &warn})
	out := RenderText(st.Snapshot())
	for _, want := range []string{"HOST", "a ", "ok", "12%", "4.0G/10.0G", "55C", "2.4MB/s", "800B/s", "1d1h", "WARN", "0.50"} {
		if !strings.Contains(out, want) {
			t.Errorf("Ausgabe enthält %q nicht:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "offline") {
		t.Errorf("Rechner ohne Paket muss als offline erscheinen:\n%s", out)
	}
	for _, r := range out {
		if r > 126 {
			t.Fatalf("nicht-ASCII-Zeichen in der Ausgabe: %q", r)
		}
	}
}

func TestExampleConfigLoads(t *testing.T) {
	c, err := LoadConfig("../../configs/server.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Hosts) != 2 || c.ConsoleInterval != 60*time.Second || c.Screens[0].PageSeconds != 8 {
		t.Fatalf("Beispiel falsch gelesen: %+v", c)
	}
}

func TestLoadConfigRejectsUnknownKeys(t *testing.T) {
	p := t.TempDir() + "/x.yaml"
	if err := os.WriteFile(p, []byte("listen: x\nunbekannt: 1\nhosts: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("unbekannter Schlüssel muss abgelehnt werden (Tippfehler sonst unbemerkt)")
	}
}
