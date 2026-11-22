package model

import (
	"math"
	"testing"
)

func f(v float64) *float64 { return &v }

func TestValidate(t *testing.T) {
	ok := Sample{OS: "linux", CPU: 10, Mem: 20, Swap: 0, Disk: 99.9, NetRx: 1, Temp: f(55), Load1: f(0.4)}
	if err := ok.Validate(); err != nil {
		t.Fatalf("gültiges Paket abgelehnt: %v", err)
	}
	bad := map[string]Sample{
		"cpu zu hoch":  {CPU: 100.1},
		"cpu negativ":  {CPU: -1},
		"NaN":          {CPU: math.NaN()},
		"Inf":          {Mem: math.Inf(1)},
		"net negativ":  {NetRx: -5},
		"temp extrem":  {Temp: f(500)},
		"load negativ": {Load1: f(-1)},
		"jobs":         {CIJobs: -1},
		"text lang":    {OS: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
	}
	for name, s := range bad {
		if s.Validate() == nil {
			t.Errorf("%s: Fehler erwartet", name)
		}
	}
}
