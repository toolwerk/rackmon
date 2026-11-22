package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/sensors"
)

func TestPickTemp(t *testing.T) {
	if pickTemp(nil) != nil {
		t.Fatal("ohne Sensoren muss nil kommen")
	}
	got := pickTemp([]sensors.TemperatureStat{
		{SensorKey: "nvme_composite", Temperature: 71},
		{SensorKey: "k10temp_tctl", Temperature: 55},
		{SensorKey: "coretemp_core_0", Temperature: 52},
		{SensorKey: "unsinn", Temperature: 0},
	})
	if got == nil || *got != 55 {
		t.Fatalf("erwartet CPU-Sensor mit 55, bekam %v", got)
	}
	got = pickTemp([]sensors.TemperatureStat{{SensorKey: "irgendwas", Temperature: 40}, {SensorKey: "anderes", Temperature: 44}})
	if got == nil || *got != 44 {
		t.Fatalf("Rückfall auf höchsten Wert: %v", got)
	}
}

func TestIsRunnerWorker(t *testing.T) {
	for name, want := range map[string]bool{"Runner.Worker": true, "runner.worker.exe": true, "Runner.Listener": false, "bash": false} {
		if isRunnerWorker(name) != want {
			t.Errorf("%s: erwartet %v", name, want)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	bad := []Config{{}, {Server: "ftp://x", Token: "t"}, {Server: "http://x"}, {Server: "http://x", Token: "t", Interval: time.Millisecond}}
	for i, c := range bad {
		if c.Validate() == nil {
			t.Errorf("Fall %d: Fehler erwartet", i)
		}
	}
	c := Config{Server: "http://x:8470/", Token: "t"}
	if err := c.Validate(); err != nil || c.Server != "http://x:8470" || c.Interval != 2*time.Second {
		t.Fatalf("Vorgaben: %v %+v", err, c)
	}
}

func TestCollectAndSend(t *testing.T) {
	var got atomic.Int32
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Errorf("JSON: %v", err)
		}
		if m["os"] == "" || m["mem_total"] == nil {
			t.Errorf("Paket unvollständig: %v", m)
		}
		got.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, Config{Server: srv.URL, Token: "geheim", Interval: time.Second}) }()
	for got.Load() < 2 && ctx.Err() == nil {
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got.Load() < 2 {
		t.Fatalf("nur %d Pakete angekommen", got.Load())
	}
	if gotAuth.Load() != "Bearer geheim" {
		t.Fatalf("Authorization = %v", gotAuth.Load())
	}
}
