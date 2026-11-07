//go:build linux && navigator

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yesh/internal/yeshcore"
)

func TestNavigatorFindMeasurementHistoryReportsOnlyMatchingSavedPoints(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	statePath, err := defaultServiceResourceStateDir("api.service")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Minute)
	state := serviceResourceState{
		Schema: serviceResourceStateSchema, Kind: "service-resource-measurement",
		Manager: "system", Unit: "api.service", Cgroup: "/system.slice/api.service",
		StartedAt: now, FinishedAt: now.Add(time.Second), Complete: true,
		Samples: []serviceResourceStateSample{{ObservedAt: now, Metrics: map[string]string{"memory.current": "10"}}},
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := yeshcore.WriteServiceResourceState(statePath, state); err != nil {
		t.Fatal(err)
	}
	history, err := navigatorFindMeasurementHistory("system", "api.service")
	if err != nil {
		t.Fatal(err)
	}
	if history.count != 1 || history.path != filepath.Join(home, ".local", "yesh", "api.service.jsonl") {
		t.Fatalf("history = %#v", history)
	}
	if !strings.Contains(navigatorMeasurementHistoryText(history), "1 saved measurement") {
		t.Fatalf("history text = %q", navigatorMeasurementHistoryText(history))
	}
}

func TestNavigatorFindMeasurementHistoryReportsIncompleteTail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".local", "yesh", "api.service.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":1`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := navigatorFindMeasurementHistory("system", "api.service"); err == nil || !strings.Contains(err.Error(), "incomplete record") {
		t.Fatalf("history error = %v", err)
	}
}

func TestNavigatorMeasurementRowsMarkComparableMemoryChanges(t *testing.T) {
	now := time.Now().UTC()
	state := func(invocation, cgroup, memory string) serviceResourceState {
		return serviceResourceState{
			StartedAt: now, FinishedAt: now.Add(time.Second), InvocationID: invocation, Cgroup: cgroup,
			Complete: true, Samples: []serviceResourceStateSample{{ObservedAt: now, Metrics: map[string]string{"memory.current": memory}}},
		}
	}
	rows := make([]navigatorMeasurementRow, 0, 3)
	rows = append(rows, navigatorMeasurementRowFromState(state("one", "/a", "100"), rows))
	rows = append(rows, navigatorMeasurementRowFromState(state("one", "/a", "200"), rows))
	rows = append(rows, navigatorMeasurementRowFromState(state("two", "/a", "300"), rows))
	if rows[0].marker != "?" || rows[1].marker != "↑" || rows[2].marker != "?" {
		t.Fatalf("markers = %q, %q, %q", rows[0].marker, rows[1].marker, rows[2].marker)
	}
	stable := navigatorMeasurementRowFromState(state("one", "/a", "200"), rows[:2])
	if stable.marker != "=" {
		t.Fatalf("stable marker = %q, want =", stable.marker)
	}
}
