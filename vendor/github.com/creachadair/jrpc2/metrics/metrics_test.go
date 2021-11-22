package metrics_test

import (
	"testing"

	"github.com/creachadair/jrpc2/metrics"
)

func getCount(m *metrics.M, name string) int64 {
	c := make(map[string]int64)
	m.Snapshot(metrics.Snapshot{Counter: c})
	return c[name]
}

func getMax(m *metrics.M, name string) int64 {
	mv := make(map[string]int64)
	m.Snapshot(metrics.Snapshot{MaxValue: mv})
	return mv[name]
}

func getLabel(m *metrics.M, name string) interface{} {
	vs := make(map[string]interface{})
	m.Snapshot(metrics.Snapshot{Label: vs})
	return vs[name]
}

func TestMetrics(t *testing.T) {
	m := metrics.New()
	wantCount := func(name string, want int64) {
		t.Helper()
		got := getCount(m, name)
		if got != want {
			t.Errorf("Counter %q: got %d, want %d", name, got, want)
		}
	}
	wantMax := func(name string, want int64) {
		t.Helper()
		got := getMax(m, name)
		if got != want {
			t.Errorf("MaxValue %q: got %d, want %d", name, got, want)
		}
	}
	wantLabel := func(name string, want interface{}) {
		t.Helper()
		got := getLabel(m, name)
		if got != want {
			t.Errorf("Label %q: got %v, want %v", name, got, want)
		}
	}

	wantCount("foo", 0)
	m.Count("foo", 1)
	wantCount("foo", 1)
	m.Count("foo", 4)
	wantCount("foo", 5)
	m.Count("foo", -3)
	wantCount("foo", 2)

	wantMax("max", 0)
	m.SetMaxValue("max", 10)
	wantMax("max", 10)
	m.SetMaxValue("max", 5)
	wantMax("max", 10)
	m.SetMaxValue("max", 12)
	wantMax("max", 12)

	m.CountAndSetMax("bar", 1)
	wantCount("bar", 1)
	wantMax("bar", 1)
	m.CountAndSetMax("bar", 4)
	wantCount("bar", 5)
	wantMax("bar", 4)
	m.CountAndSetMax("bar", -3)
	wantCount("bar", 2)
	wantMax("bar", 4)
	m.CountAndSetMax("bar", 3)
	wantCount("bar", 5)
	wantMax("bar", 4)

	wantLabel("hey", nil)
	m.SetLabel("hey", "you")
	wantLabel("hey", "you")
	m.SetLabel("hey", nil)
	wantLabel("hey", nil)

	var numCalls int
	m.SetLabel("dyno", func() interface{} {
		numCalls++
		return numCalls
	})
	wantLabel("dyno", 1)
	wantLabel("dyno", 2)
	wantLabel("dyno", 3)
	m.SetLabel("dyno", nil)
	wantLabel("dyno", nil)

	wantLabel("quux", nil)
	m.EditLabel("quux", func(v interface{}) interface{} {
		return "x"
	})
	wantLabel("quux", "x")
	m.EditLabel("quux", func(v interface{}) interface{} {
		return v.(string) + "2"
	})
	wantLabel("quux", "x2")
}
