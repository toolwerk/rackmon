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

func TestPickLHM(t *testing.T) {
	const js = `{"Text":"Sensor","Children":[{"Text":"PC","Children":[
	 {"Text":"AMD Ryzen 7","ImageURL":"images_icon/cpu.png","Children":[
	  {"Text":"Temperatures","ImageURL":"images_icon/temperature.png","Children":[
	   {"Text":"Core #1","Value":"58,0 °C","Type":"Temperature"},
	   {"Text":"Core (Tctl/Tdie)","Value":"52,5 °C","Type":"Temperature"}]}]},
	 {"Text":"NVMe","ImageURL":"images_icon/hdd.png","Children":[
	  {"Text":"Temperature","Value":"71,0 °C","Type":"Temperature"}]}]}]}`
	var root lhmNode
	if err := json.Unmarshal([]byte(js), &root); err != nil {
		t.Fatal(err)
	}
	got := pickLHM(root)
	if got == nil || *got != 52.5 {
		t.Fatalf("erwartet Tctl mit 52,5, bekam %v", got)
	}
	if pickLHM(lhmNode{}) != nil {
		t.Fatal("leerer Baum muss nil geben")
	}
}

func TestFetchLHM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"Text":"x","Children":[{"Text":"cpu","ImageURL":"cpu.png","Children":[{"Text":"CPU Package","Value":"61.0 °C","Type":"Temperature"}]}]}`))
	}))
	defer ts.Close()
	got, err := fetchLHM(context.Background(), ts.Client(), ts.URL)
	if err != nil || got == nil || *got != 61 {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestValidateTempURL(t *testing.T) {
	for _, ok := range []string{"http://127.0.0.1:8085/data.json", "http://localhost:8085/data.json", "http://[::1]:8085/data.json"} {
		if err := validateTempURL(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://192.168.1.5:8085/data.json", "http://example.org/data.json", "ftp://127.0.0.1/x", ""} {
		if validateTempURL(bad) == nil {
			t.Errorf("%q hätte abgelehnt werden müssen", bad)
		}
	}
}

func TestParseLHMValue(t *testing.T) {
	for in, want := range map[string]float64{"52,5 °C": 52.5, "52.5 °C": 52.5, "58,8 \xb0C": 58.8, "61": 61} {
		if got, ok := parseLHMValue(in); !ok || got != want {
			t.Errorf("%q: %v %v", in, got, ok)
		}
	}
	if _, ok := parseLHMValue("n/a"); ok {
		t.Error("n/a darf keine Zahl sein")
	}
}
