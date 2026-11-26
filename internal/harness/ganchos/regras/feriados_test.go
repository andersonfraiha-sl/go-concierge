package regras

import (
	"testing"
	"time"
)

func TestFeriadoNacional(t *testing.T) {
	casos := []struct {
		data string
		quer bool
	}{
		{"2026-02-16", true}, {"2026-02-17", true}, {"2026-04-03", true},
		{"2026-06-04", true}, {"2026-11-15", true}, {"2026-01-01", true},
		{"2026-12-25", true}, {"2025-04-18", true}, {"2025-06-19", true},
		{"2026-11-03", false}, {"2026-02-18", false}, {"2026-04-05", false},
	}
	for _, c := range casos {
		d, err := time.Parse("2006-01-02", c.data)
		if err != nil {
			t.Fatal(err)
		}
		if got := FeriadoNacional(d); got != c.quer {
			t.Errorf("FeriadoNacional(%s) = %v, quer %v", c.data, got, c.quer)
		}
	}
}
