//go:build linux

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestLinuxServiceSummaryRejectsCorruptAndMismatchedState(t *testing.T) {
	dir := t.TempDir()
	r := &runtime{values: map[string]value{}, filename: "summary.yesh"}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("not-json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.runServiceResourceSummary("user", "api.service", dir, time.Hour); err == nil {
		t.Fatal("accepted corrupt resource state")
	}

	if err := os.Remove(filepath.Join(dir, "corrupt.json")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	state := serviceResourceState{
		Schema: serviceResourceStateSchema, Kind: "service-resource-measurement",
		Manager: "user", Unit: "other.service", Cgroup: "/user.slice/other.service",
		StartedAt: now.Add(-time.Minute), FinishedAt: now.Add(-50 * time.Second), Complete: true,
		Samples: []serviceResourceStateSample{
			{ObservedAt: now.Add(-time.Minute), Metrics: map[string]string{"memory.current": "10"}},
			{ObservedAt: now.Add(-50 * time.Second), Metrics: map[string]string{"memory.current": "20"}},
		},
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mismatched.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.runServiceResourceSummary("user", "api.service", dir, time.Hour); err == nil {
		t.Fatal("accepted summary without a matching service measurement")
	}
}

func TestLinuxCgroupIOStatKeepsDeviceIdentityAndCounters(t *testing.T) {
	dir := t.TempDir()
	data := "8:0 rbytes=100 wbytes=200 rios=3 wios=4 dbytes=5 dios=6\n8:16 wios=9 rbytes=10\n"
	if err := os.WriteFile(filepath.Join(dir, "io.stat"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readCgroupIOStat(dir)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"io.stat.8:0.rbytes":  "100",
		"io.stat.8:0.wbytes":  "200",
		"io.stat.8:0.rios":    "3",
		"io.stat.8:0.wios":    "4",
		"io.stat.8:0.dbytes":  "5",
		"io.stat.8:0.dios":    "6",
		"io.stat.8:16.wios":   "9",
		"io.stat.8:16.rbytes": "10",
	} {
		if got[key] != want {
			t.Fatalf("%s = %q, want %q", key, got[key], want)
		}
	}
}

func TestLinuxCgroupIOStatRejectsMalformedDeviceOrCounter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "io.stat"), []byte("not-a-device rbytes=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCgroupIOStat(dir); err == nil {
		t.Fatal("accepted malformed io.stat device")
	}
	if err := os.WriteFile(filepath.Join(dir, "io.stat"), []byte("8:0 rbytes=not-a-number\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCgroupIOStat(dir); err == nil {
		t.Fatal("accepted malformed io.stat counter")
	}
	missing := t.TempDir()
	metrics, err := readCgroupIOStat(missing)
	if err != nil || metrics != nil {
		t.Fatalf("missing io.stat = %#v, %v; want unavailable marker", metrics, err)
	}
}

func TestLinuxServiceIODeltasKeepDeviceIdentity(t *testing.T) {
	first := map[string]string{
		"io.stat.available":  "yes",
		"io.stat.8:0.rbytes": "100", "io.stat.8:0.wbytes": "200",
		"io.stat.8:0.rios": "3", "io.stat.8:0.wios": "4",
	}
	last := map[string]string{
		"io.stat.available":  "yes",
		"io.stat.8:0.rbytes": "110", "io.stat.8:0.wbytes": "260",
		"io.stat.8:0.rios": "5", "io.stat.8:0.wios": "7",
	}
	deltas, available, complete := compareServiceIOStats(first, last)
	if !available || !complete {
		t.Fatalf("available=%t complete=%t, want true true", available, complete)
	}
	if got := deltas["8:0"]; got.rbytes != 10 || got.wbytes != 60 || got.rios != 2 || got.wios != 3 {
		t.Fatalf("I/O delta = %+v", got)
	}
	last["io.stat.8:16.rbytes"] = "1"
	if _, _, complete := compareServiceIOStats(first, last); complete {
		t.Fatal("accepted changed device set as complete")
	}
}

func TestLinuxServiceStatusUsesAllowlistedSnapshot(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(systemctl, []byte("#!/bin/sh\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=active' 'SubState=running'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service status api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxServiceStatusReportsSystemctlFailure(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(systemctl, []byte("#!/bin/sh\necho 'Unit api.service could not be found.' >&2\nexit 5\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service status api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil {
		t.Fatal("expected systemctl failure")
	}
}

func TestLinuxServiceResourcesUsesBoundedSnapshot(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=inactive' 'SubState=dead' 'MainPID=0' 'ControlGroup=' 'NRestarts=2'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service resources api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxServiceResourcesRejectsUnexpectedUnit(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nprintf '%s\\n' 'Id=other.service' 'LoadState=loaded' 'ActiveState=active' 'SubState=running' 'MainPID=1' 'ControlGroup=/system.slice/other.service' 'NRestarts=0'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service resources api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil {
		t.Fatal("expected unexpected unit to be rejected")
	}
}

func TestLinuxServiceResourcesPlanDoesNotQuerySystemctl(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh", plan: true}
	items, err := parse("service resources api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxServiceMeasurementPlanDoesNotWriteState(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "missing", "state")
	t.Setenv("PATH", t.TempDir())
	r := &runtime{values: map[string]value{}, filename: "service.yesh", plan: true}
	items, err := parse("measure service api.service for 2s every 1s save to \""+stateDir+"\"\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("plan created state directory: %v", err)
	}
}

func TestLinuxServiceResourceJournalAppendsAndIgnoresPartialTail(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Add(-time.Minute)
	state := serviceResourceState{
		Schema: serviceResourceStateSchema, Kind: "service-resource-measurement",
		Manager: "system", Unit: "api.service", Cgroup: "/system.slice/api.service",
		StartedAt: now, FinishedAt: now.Add(time.Second), Complete: true,
		Samples: []serviceResourceStateSample{
			{ObservedAt: now, Metrics: map[string]string{"memory.current": "10"}},
			{ObservedAt: now.Add(time.Second), Metrics: map[string]string{"memory.current": "20"}},
		},
	}
	r := &runtime{values: map[string]value{}, filename: "journal.yesh"}
	if _, err := writeServiceResourceState(r, dir, state); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("state directory entries after first append = %d, want journal and lock", len(entries))
	}
	state.StartedAt = now.Add(2 * time.Second)
	state.FinishedAt = now.Add(3 * time.Second)
	for index := range state.Samples {
		state.Samples[index].ObservedAt = state.Samples[index].ObservedAt.Add(2 * time.Second)
	}
	if _, err := writeServiceResourceState(r, dir, state); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("state directory entries after second append = %d, want journal and lock", len(entries))
	}
	journal := filepath.Join(dir, serviceResourceJournalName)
	file, err := os.OpenFile(journal, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"schema":1,"kind":"service-resource-measurement"}`); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	states, err := readServiceResourceJournal(journal)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("journal states = %d, want 2", len(states))
	}
}

func TestLinuxServiceResourceJournalRejectsMalformedCompleteRecord(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, serviceResourceJournalName)
	if err := os.WriteFile(journal, []byte(`{"schema":1,"kind":"service-resource-measurement"}`+"\n"+`{"broken"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readServiceResourceJournal(journal); err == nil {
		t.Fatal("accepted malformed complete JSONL record")
	}
}

func TestLinuxServiceResourceJournalSerializesConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	r := &runtime{values: map[string]value{}, filename: "concurrent-journal.yesh"}
	const writers = 8
	const recordsPerWriter = 20
	var wg sync.WaitGroup
	errors := make(chan error, writers)
	successes := make(chan struct{}, writers*recordsPerWriter)
	for writer := 0; writer < writers; writer++ {
		writer := writer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for record := 0; record < recordsPerWriter; record++ {
				start := time.Unix(int64(writer*recordsPerWriter+record), 0)
				state := serviceResourceState{
					Schema: serviceResourceStateSchema, Kind: "service-resource-measurement",
					Manager: "system", Unit: "api.service", Cgroup: "/system.slice/api.service",
					StartedAt: start, FinishedAt: start.Add(time.Second), Complete: true,
					Samples: []serviceResourceStateSample{
						{ObservedAt: start, Metrics: map[string]string{"memory.current": "10"}},
						{ObservedAt: start.Add(time.Second), Metrics: map[string]string{"memory.current": "20"}},
					},
				}
				if _, err := writeServiceResourceState(r, dir, state); err != nil {
					if !strings.Contains(err.Error(), "already held") {
						errors <- err
						return
					}
					continue
				}
				successes <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(errors)
	close(successes)
	for err := range errors {
		t.Fatal(err)
	}
	states, err := readServiceResourceJournal(filepath.Join(dir, serviceResourceJournalName))
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != len(successes) || len(states) == 0 {
		t.Fatalf("journal records = %d, successful writes = %d", len(states), len(successes))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("state directory entries = %d, want journal and lock", len(entries))
	}
}

func TestLinuxServiceResourceRetentionKeepsLastDayAndUsesOneJournal(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	makeState := func(start time.Time) serviceResourceState {
		return serviceResourceState{
			Schema: serviceResourceStateSchema, Kind: "service-resource-measurement",
			Manager: "system", Unit: "api.service", Cgroup: "/system.slice/api.service",
			StartedAt: start, FinishedAt: start.Add(time.Second), Complete: true,
			Samples: []serviceResourceStateSample{
				{ObservedAt: start, Metrics: map[string]string{"memory.current": "10"}},
				{ObservedAt: start.Add(time.Second), Metrics: map[string]string{"memory.current": "20"}},
			},
		}
	}
	r := &runtime{values: map[string]value{}, filename: "retention.yesh"}
	if _, err := writeServiceResourceState(r, dir, makeState(now.Add(-48*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := writeServiceResourceState(r, dir, makeState(now.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	removed, err := pruneServiceResourceHistory(r, dir, "system", "api.service", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	states, err := readServiceResourceJournal(filepath.Join(dir, serviceResourceJournalName))
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || !states[0].StartedAt.Equal(now.Add(-time.Hour)) {
		t.Fatalf("retained states = %#v, want the recent state", states)
	}
}

func TestLinuxDefaultServiceResourceStatePathIsUserOwned(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := defaultServiceResourceStateDir("api.service")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", ".yesh", "resources", "api.service")
	if got != want {
		t.Fatalf("default state path = %q, want %q", got, want)
	}
}

func TestServiceSummaryTrendIsExplicitAndNonCausal(t *testing.T) {
	for _, test := range []struct {
		name                 string
		start, end           uint64
		present              bool
		wantDelta, wantTrend string
	}{
		{name: "rising", start: 10, end: 25, present: true, wantDelta: "+15", wantTrend: "rising"},
		{name: "falling", start: 25, end: 10, present: true, wantDelta: "-15", wantTrend: "falling"},
		{name: "stable", start: 10, end: 10, present: true, wantDelta: "+0", wantTrend: "stable"},
		{name: "unavailable", present: false, wantDelta: "unavailable", wantTrend: "unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := summaryDelta(test.start, test.end, test.present); got != test.wantDelta {
				t.Fatalf("delta = %q, want %q", got, test.wantDelta)
			}
			if got := summaryTrend(test.start, test.end, test.present); got != test.wantTrend {
				t.Fatalf("trend = %q, want %q", got, test.wantTrend)
			}
		})
	}
}

func TestAnalyzeServiceGaugeUsesAllSamples(t *testing.T) {
	start := time.Unix(100, 0)
	for _, test := range []struct {
		name, want string
		values     []float64
	}{
		{name: "rising", values: []float64{10, 20, 30}, want: "rising"},
		{name: "falling", values: []float64{30, 20, 10}, want: "falling"},
		{name: "stable", values: []float64{10, 10, 10}, want: "stable"},
		{name: "volatile", values: []float64{10, 30, 20, 40}, want: "volatile"},
	} {
		t.Run(test.name, func(t *testing.T) {
			points := make([]serviceGaugePoint, len(test.values))
			for index, value := range test.values {
				points[index] = serviceGaugePoint{at: start.Add(time.Duration(index) * time.Second), value: value}
			}
			slope, trend, ok := analyzeServiceGauge(points)
			if !ok || slope == 0 && test.want != "stable" || trend != test.want {
				t.Fatalf("slope=%v trend=%q ok=%v, want trend %q", slope, trend, ok, test.want)
			}
		})
	}
}

func TestLinuxServiceSummaryPrintsVolatileEvents(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().UTC().Add(-time.Minute)
	state := serviceResourceState{
		Schema: serviceResourceStateSchema, Kind: "service-resource-measurement",
		Manager: "user", Unit: "api.service", Cgroup: "/user.slice/api.service",
		StartedAt: start, FinishedAt: start.Add(3 * time.Second), Complete: true,
	}
	for index, value := range []string{"10", "30", "20", "40"} {
		state.Samples = append(state.Samples, serviceResourceStateSample{
			ObservedAt: start.Add(time.Duration(index) * time.Second),
			Metrics:    map[string]string{"memory.current": value}, RSSBytes: uint64(100 + index*10),
		})
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "volatile.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = write
	runErr := (&runtime{values: map[string]value{}, filename: "summary.yesh"}).runServiceResourceSummary("user", "api.service", dir, time.Hour)
	_ = write.Close()
	os.Stdout = oldStdout
	output, readErr := io.ReadAll(read)
	_ = read.Close()
	if runErr != nil || readErr != nil {
		t.Fatalf("summary error=%v read error=%v", runErr, readErr)
	}
	text := string(output)
	for _, want := range []string{"Service api.service was observed", "Memory:", "trend volatile", "RSS:", "Process RSS for api.service:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary output missing %q: %s", want, text)
		}
	}
}

func TestLinuxServiceSummaryRejectsDifferentInvocationIDs(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Add(-time.Minute)
	for index, invocation := range []string{"invocation-1", "invocation-2"} {
		start := now.Add(time.Duration(index) * time.Second)
		state := serviceResourceState{
			Schema: serviceResourceStateSchema, Kind: "service-resource-measurement",
			Manager: "user", Unit: "api.service", Cgroup: "/user.slice/api.service",
			InvocationID: invocation, StartedAt: start, FinishedAt: start.Add(time.Second), Complete: true,
			Samples: []serviceResourceStateSample{
				{ObservedAt: start, Metrics: map[string]string{"memory.current": "10"}},
				{ObservedAt: start.Add(time.Second), Metrics: map[string]string{"memory.current": "20"}},
			},
		}
		data, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, invocation+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	err := (&runtime{values: map[string]value{}, filename: "summary.yesh"}).runServiceResourceSummary("user", "api.service", dir, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "multiple service identities") {
		t.Fatalf("summary error = %v, want identity refusal", err)
	}
}

func TestCgroupPathContainsOnlySameNodeOrDescendants(t *testing.T) {
	tests := []struct {
		root, candidate string
		want            bool
	}{
		{root: "/system.slice/api.service", candidate: "/system.slice/api.service", want: true},
		{root: "/system.slice/api.service", candidate: "/system.slice/api.service/worker", want: true},
		{root: "/system.slice/api.service", candidate: "/system.slice/other.service", want: false},
		{root: "/system.slice/api.service", candidate: "/system.slice/api.service-old", want: false},
		{root: "/", candidate: "/anything", want: true},
		{root: "relative", candidate: "/relative", want: false},
	}
	for _, test := range tests {
		if got := cgroupPathContains(test.root, test.candidate); got != test.want {
			t.Errorf("cgroupPathContains(%q, %q) = %t, want %t", test.root, test.candidate, got, test.want)
		}
	}
}

func TestServiceProcessSummaryAttributesRSSByPIDAndStartTime(t *testing.T) {
	state := serviceResourceState{
		Samples: []serviceResourceStateSample{
			{Processes: []serviceResourceProcessIdentity{
				{PID: 7, StartTime: 10, Name: "worker", RSSBytes: 10 * 1024 * 1024},
				{PID: 8, StartTime: 20, Name: "old", RSSBytes: 4 * 1024 * 1024},
			}},
			{Processes: []serviceResourceProcessIdentity{
				{PID: 7, StartTime: 10, Name: "worker", RSSBytes: 90 * 1024 * 1024},
				{PID: 9, StartTime: 30, Name: "new", RSSBytes: 2 * 1024 * 1024},
			}},
		},
	}
	var got serviceProcessSummary
	summarizeServiceProcessSamples(&got, []serviceResourceState{state})
	if !got.Available || !got.HasTop || got.TopDelta.PID != 7 || got.TopDelta.Delta != 80*1024*1024 {
		t.Fatalf("unexpected process growth: %+v", got)
	}
	if got.Started != 1 || got.Exited != 1 {
		t.Fatalf("unexpected process churn: %+v", got)
	}

	state.Samples[1].Processes[0].StartTime = 11
	got = serviceProcessSummary{}
	summarizeServiceProcessSamples(&got, []serviceResourceState{state})
	if got.HasTop {
		t.Fatalf("PID reuse was treated as process growth: %+v", got)
	}
}

func TestParseServiceResourceProperties(t *testing.T) {
	got, err := parseServiceResourceProperties([]byte("Id=api.service\nLoadState=loaded\nActiveState=active\nSubState=running\nMainPID=42\nControlGroup=/system.slice/api.service\nNRestarts=0\n"))
	if err != nil || got["ControlGroup"] != "/system.slice/api.service" {
		t.Fatalf("properties=%v err=%v", got, err)
	}
	if _, err := parseServiceResourceProperties([]byte("Id=api.service\nLoadState=loaded\nActiveState=active\nSubState=running\nControlGroup=/x\nUnknown=value\n")); err == nil {
		t.Fatal("accepted unexpected systemctl property")
	}
}

func TestLinuxReadServiceCgroupIncludesCurrentProcess(t *testing.T) {
	cgroup, err := readProcessCgroup("/proc/self/cgroup")
	if err != nil || cgroup == "" {
		t.Skipf("cgroup v2 is unavailable: %v", err)
	}
	processes, metrics, rss, err := readServiceCgroup("/sys/fs/cgroup", cgroup)
	if err != nil {
		t.Skipf("current cgroup is not readable: %v", err)
	}
	found := false
	for _, process := range processes {
		if process.pid == os.Getpid() {
			found = true
			break
		}
	}
	if !found || rss == 0 || metrics["memory.current"] == "" {
		t.Fatalf("current process missing from cgroup snapshot: found=%v rss=%d metrics=%v", found, rss, metrics)
	}
}

func TestLinuxReadServiceCgroupRejectsEscapingPath(t *testing.T) {
	if _, _, _, err := readServiceCgroup(t.TempDir(), "/../../proc"); err == nil {
		t.Fatal("accepted cgroup path escaping root")
	}
}

func TestLinuxReadServiceCgroupMarksMissingNestedProcFileIncomplete(t *testing.T) {
	root := t.TempDir()
	cgroup := filepath.Join(root, "system.slice", "api.service")
	child := filepath.Join(cgroup, "worker")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cgroup, "cgroup.procs"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(child, "removed"), filepath.Join(child, "cgroup.procs")); err != nil {
		t.Fatal(err)
	}

	_, metrics, _, err := readServiceCgroup(root, "/system.slice/api.service")
	if err != nil {
		t.Fatal(err)
	}
	if metrics["processes.incomplete"] != "yes" {
		t.Fatalf("processes.incomplete = %q, want yes", metrics["processes.incomplete"])
	}
}

func TestLinuxCgroupProcessListErrorExplainsThreadedCgroup(t *testing.T) {
	err := cgroupProcessListError("/sys/fs/cgroup/api.service/worker/cgroup.procs", syscall.EOPNOTSUPP)
	if !strings.Contains(err.Error(), "threaded cgroups are not supported") {
		t.Fatalf("error = %q", err)
	}
}

func TestReadCgroupMetricReadsFlatKeyAndReportsUnavailable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.events"), []byte("oom 2\noom_kill 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := readCgroupMetric(dir, "memory.events", "oom_kill"); got != "1" {
		t.Fatalf("oom_kill = %q, want 1", got)
	}
	if got := readCgroupMetric(dir, "memory.events", "high"); got != "unavailable" {
		t.Fatalf("missing key = %q, want unavailable", got)
	}
	if got := readCgroupMetric(dir, "memory.events.local", "oom"); got != "unavailable" {
		t.Fatalf("missing file = %q, want unavailable", got)
	}
}

func TestReadCgroupPressureMetricReadsWindowAndReportsUnavailable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.pressure"), []byte("some avg10=1.25 avg60=2.50 avg300=3.75 total=42\nfull avg10=0.00 avg60=0.10 avg300=0.20 total=7\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := readCgroupPressureMetric(dir, "memory.pressure", "some", "avg60"); got != "2.50" {
		t.Fatalf("avg60 = %q, want 2.50", got)
	}
	if got := readCgroupPressureMetric(dir, "memory.pressure", "some", "avg5"); got != "unavailable" {
		t.Fatalf("missing window = %q, want unavailable", got)
	}
	if got := readCgroupPressureMetric(dir, "io.pressure", "some", "avg10"); got != "unavailable" {
		t.Fatalf("missing file = %q, want unavailable", got)
	}
}

func TestCompareServiceResourceSnapshotsUsesCounterDeltas(t *testing.T) {
	first := serviceResourceSnapshot{
		properties: map[string]string{"Id": "api.service", "NRestarts": "2"},
		cgroup:     "/user.slice/api.service",
		observedAt: time.Unix(100, 0),
		metrics: map[string]string{
			"processes.incomplete":   "no",
			"cpu.stat.usage_usec":    "100",
			"memory.events.oom_kill": "1",
			"pids.events.max":        "0",
		},
	}
	last := first
	last.observedAt = time.Unix(105, 0)
	last.metrics = map[string]string{
		"processes.incomplete":   "no",
		"cpu.stat.usage_usec":    "350",
		"memory.events.oom_kill": "2",
		"pids.events.max":        "unavailable",
	}
	got, err := compareServiceResourceSnapshots(first, last)
	if err != nil {
		t.Fatal(err)
	}
	if got.elapsed != 5*time.Second || got.deltas["cpu.stat.usage_usec"] != 250 || got.deltas["memory.events.oom_kill"] != 1 {
		t.Fatalf("interval = %+v", got)
	}
	if _, ok := got.deltas["pids.events.max"]; ok || got.incomplete {
		t.Fatalf("unavailable counter or complete interval reported incorrectly: %+v", got)
	}
}

func TestCompareServiceResourceSnapshotsDetectsRestartAndPIDReuse(t *testing.T) {
	first := serviceResourceSnapshot{
		properties: map[string]string{"Id": "api.service", "NRestarts": "2"},
		cgroup:     "/api.service",
		observedAt: time.Unix(100, 0),
		metrics:    map[string]string{"processes.incomplete": "no"},
		processes:  []processInfo{{pid: 7, startTime: 10}},
	}
	last := first
	last.observedAt = time.Unix(101, 0)
	last.properties = map[string]string{"Id": "api.service", "NRestarts": "3"}
	last.processes = []processInfo{{pid: 7, startTime: 11}}
	got, err := compareServiceResourceSnapshots(first, last)
	if err != nil {
		t.Fatal(err)
	}
	started, exited := countProcessChurn(first.processes, last.processes)
	if started != 1 || exited != 1 || !got.incomplete {
		t.Fatalf("started=%d exited=%d interval=%+v", started, exited, got)
	}
}

func TestCompareServiceResourceSnapshotsSeparatesInvocationIDs(t *testing.T) {
	first := serviceResourceSnapshot{
		properties: map[string]string{"Id": "api.service", "NRestarts": "0", "InvocationID": "invocation-1"},
		cgroup:     "/api.service",
		observedAt: time.Unix(100, 0),
		metrics:    map[string]string{"processes.incomplete": "no"},
	}
	last := first
	last.observedAt = time.Unix(101, 0)
	last.properties = map[string]string{"Id": "api.service", "NRestarts": "0", "InvocationID": "invocation-2"}
	got, err := compareServiceResourceSnapshots(first, last)
	if err != nil {
		t.Fatal(err)
	}
	if !got.incomplete {
		t.Fatalf("different invocation IDs were treated as one complete interval: %+v", got)
	}
}

func TestLinuxServiceMeasurementWritesIncompleteRecordOnInvocationChange(t *testing.T) {
	root := t.TempDir()
	cgroup := filepath.Join(root, "system.slice", "api.service")
	if err := os.MkdirAll(cgroup, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cgroup, "cgroup.procs"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	countFile := filepath.Join(bin, "count")
	systemctl := filepath.Join(bin, "systemctl")
	script := "#!/bin/sh\n" +
		"count=0\n" +
		"if [ -f \"" + countFile + "\" ]; then read count < \"" + countFile + "\"; fi\n" +
		"count=$((count + 1))\n" +
		"printf '%s\\n' \"$count\" > \"" + countFile + "\"\n" +
		"invocation=invocation-1\n" +
		"if [ \"$count\" -gt 1 ]; then invocation=invocation-2; fi\n" +
		"printf '%s\\n' \"Id=api.service\" \"LoadState=loaded\" \"ActiveState=active\" \"SubState=running\" \"MainPID=0\" \"ControlGroup=/system.slice/api.service\" \"NRestarts=0\" \"InvocationID=$invocation\"\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	stateDir := filepath.Join(t.TempDir(), "missing", "state")
	r := &runtime{values: map[string]value{}, filename: "identity-change.yesh", cgroupRoot: root}
	if err := r.runServiceMeasurement("system", "api.service", 20*time.Millisecond, 1*time.Millisecond, 0, stateDir); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Dir(stateDir), stateDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat created state directory %q: %v", path, err)
		}
		if mode := info.Mode().Perm(); mode != 0700 {
			t.Fatalf("state directory %q mode = %o, want 700", path, mode)
		}
	}
	states, err := readServiceResourceJournal(filepath.Join(stateDir, serviceResourceJournalName))
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Fatalf("states = %d, want 1", len(states))
	}
	state := states[0]
	if state.Complete || len(state.Samples) != 2 || len(state.IncompleteReasons) != 1 || state.IncompleteReasons[0] != "identity_changed" {
		t.Fatalf("unexpected identity-change state: %+v", state)
	}
	if state.InvocationID != "invocation-1" {
		t.Fatalf("record invocation = %q, want invocation-1", state.InvocationID)
	}
}

func TestServiceResourceStateValidatesBoundedIdentityAndSamples(t *testing.T) {
	start := time.Unix(100, 0)
	state := serviceResourceState{
		Schema:       serviceResourceStateSchema,
		Kind:         "service-resource-measurement",
		Manager:      "user",
		Unit:         "api.service",
		Cgroup:       "/user.slice/api.service",
		InvocationID: "invocation-1",
		StartedAt:    start,
		FinishedAt:   time.Unix(105, 0),
		Complete:     true,
		Samples: []serviceResourceStateSample{
			{ObservedAt: start, ProcessCount: 1, Processes: []serviceResourceProcessIdentity{{PID: 7, StartTime: 10}}},
			{ObservedAt: time.Unix(105, 0), ProcessCount: 1, Processes: []serviceResourceProcessIdentity{{PID: 7, StartTime: 10}}},
		},
	}
	if err := state.validate(); err != nil {
		t.Fatal(err)
	}
	state.Samples[1].Processes[0].StartTime = 0
	if err := state.validate(); err == nil {
		t.Fatal("accepted process identity without start time")
	}
}

func TestServiceResourceObservationExplainsIncompleteScope(t *testing.T) {
	for _, test := range []struct {
		name, loadState, cgroup, want, reason string
	}{
		{name: "missing unit", loadState: "not-found", want: "unavailable", reason: "unit_not_found"},
		{name: "no cgroup", loadState: "loaded", want: "unavailable", reason: "no_cgroup"},
		{name: "missing metric", loadState: "loaded", cgroup: "/api.service", want: "partial", reason: "metric_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := serviceResourceSnapshot{properties: map[string]string{"LoadState": test.loadState}, cgroup: test.cgroup, metrics: map[string]string{"memory.current": "unavailable", "processes.incomplete": "no"}}
			if test.cgroup == "" && test.loadState == "loaded" {
				snapshot.metrics = map[string]string{}
			}
			got, reason := serviceResourceObservation(snapshot)
			if got != test.want || reason != test.reason {
				t.Fatalf("observation=%q reason=%q, want %q/%q", got, reason, test.want, test.reason)
			}
		})
	}
}

func TestServiceResourceObservationMessageIsOperatorReadable(t *testing.T) {
	got := serviceResourceObservationMessage("unavailable", "metric_unavailable")
	if got != "some resource files were unavailable; unavailable values are not zero" {
		t.Fatalf("message = %q", got)
	}
}

func TestServiceMeasurementDoesNotFailForUnavailableOptionalMetric(t *testing.T) {
	complete := true
	reasons := []string{}
	snapshot := serviceResourceSnapshot{
		properties: map[string]string{"LoadState": "loaded"},
		cgroup:     "/api.service",
		metrics:    map[string]string{"memory.current": "unavailable", "processes.incomplete": "no"},
	}
	appendServiceMeasurementCompleteness(&complete, &reasons, snapshot)
	appendServiceMeasurementCompleteness(&complete, &reasons, snapshot)
	if !complete || len(reasons) != 0 {
		t.Fatalf("complete=%t reasons=%v", complete, reasons)
	}
}

func TestLinuxSystemdUserProfileIsVisibleInArgv(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	trace := filepath.Join(dir, "trace")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + trace + "'\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=active' 'SubState=running'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service status user api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	want := "--user\nshow\n--no-pager\n--property=Id,LoadState,ActiveState,SubState\napi.service\n"
	if string(data) != want {
		t.Fatalf("argv trace = %q, want %q", data, want)
	}
}

func TestLinuxServiceStatusRejectsUnexpectedUnit(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(systemctl, []byte("#!/bin/sh\nprintf '%s\\n' 'Id=other.service' 'LoadState=loaded' 'ActiveState=active' 'SubState=running'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service status api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil {
		t.Fatal("expected unexpected unit to be rejected")
	}
}

func TestLinuxServiceRestartChecksPostcondition(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nif [ \"$1\" = restart ]; then exit 0; fi\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=active' 'SubState=running'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service restart api.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxServiceStartChecksPostcondition(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nif [ \"$1\" = start ]; then exit 0; fi\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=active' 'SubState=running'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service start api.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxServiceTransitionRejectsTerminalInactiveOneshot(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nif [ \"$1\" = start ]; then exit 0; fi\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=inactive' 'SubState=dead'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service start api.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = r.execute(items)
	if err == nil || !strings.Contains(err.Error(), "cannot reach active") {
		t.Fatalf("unexpected terminal inactive error: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("terminal inactive state waited too long: %s", elapsed)
	}
}

func TestLinuxServiceRestartAllowsEmptyOptionalFieldsAndWaits(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nif [ \"$1\" = restart ]; then exit 0; fi\ncount=0\nif [ -f \"$WAIT_COUNT\" ]; then read count < \"$WAIT_COUNT\"; fi\ncount=$((count + 1))\nprintf '%s' \"$count\" > \"$WAIT_COUNT\"\nstate=activating\nif [ \"$count\" -ge 3 ]; then state=active; fi\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' \"ActiveState=$state\" 'SubState=running' 'Result=' 'ExecMainCode=' 'ExecMainStatus=' 'ActiveEnterTimestampMonotonic='\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("WAIT_COUNT", filepath.Join(dir, "count"))
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service restart api.service until active within 1s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "count")); err != nil || string(got) != "3" {
		t.Fatalf("restart status polls = %q, error = %v", got, err)
	}
}

func TestLinuxServiceRestartFailureIncludesSnapshot(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nif [ \"$1\" = restart ]; then exit 5; fi\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=failed' 'SubState=failed' 'Result=exit-code' 'ExecMainCode=1' 'ExecMainStatus=23' 'ActiveEnterTimestampMonotonic=0'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service restart api.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	err = r.execute(items)
	if err == nil || !strings.Contains(err.Error(), "result=exit-code") || !strings.Contains(err.Error(), "exec_main_status=23") {
		t.Fatalf("unexpected restart failure: %v", err)
	}
}

func TestLinuxServiceWaitPollsUntilActive(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\ncount=0\nif [ -f \"$WAIT_COUNT\" ]; then read count < \"$WAIT_COUNT\"; fi\ncount=$((count + 1))\nprintf '%s' \"$count\" > \"$WAIT_COUNT\"\nstate=activating\nif [ \"$count\" -ge 2 ]; then state=active; fi\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' \"ActiveState=$state\" 'SubState=running'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("WAIT_COUNT", filepath.Join(dir, "count"))
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service wait api.service until active within 1s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "count")); err != nil || string(got) != "2" {
		t.Fatalf("status polls = %q, error = %v", got, err)
	}
}

func TestLinuxServiceWaitTimesOutWithLastState(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=activating' 'SubState=start'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service wait api.service until active within 120ms\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "active_state=activating") {
		t.Fatalf("unexpected timeout error: %v", err)
	}
}

func TestLinuxServiceWaitReportsFailedImmediately(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	trace := filepath.Join(dir, "trace")
	script := "#!/bin/sh\nprintf '%s\\n' x >> \"$TRACE_FILE\"\nprintf '%s\\n' 'Id=api.service' 'LoadState=loaded' 'ActiveState=failed' 'SubState=failed'\n"
	if err := os.WriteFile(systemctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TRACE_FILE", trace)
	r := &runtime{values: map[string]value{}, filename: "service.yesh"}
	items, err := parse("service wait api.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil || !strings.Contains(err.Error(), "active_state=failed") {
		t.Fatalf("unexpected failed-state error: %v", err)
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "x\n" {
		t.Fatalf("expected one state query, got %q", data)
	}
}

func TestLinuxJournalQueryIsBoundedAndStructured(t *testing.T) {
	dir := t.TempDir()
	journalctl := filepath.Join(dir, "journalctl")
	script := "#!/bin/sh\nprintf '%s\\n' '{\"MESSAGE\":\"ready\"}'\n"
	if err := os.WriteFile(journalctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "journal.yesh"}
	items, err := parse("journal api.service since 15m lines 20\n", "journal.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxJournalQueryRejectsDiagnosticOnSuccess(t *testing.T) {
	dir := t.TempDir()
	journalctl := filepath.Join(dir, "journalctl")
	script := "#!/bin/sh\necho 'permission warning' >&2\nprintf '%s\\n' '{\"MESSAGE\":\"partial\"}'\n"
	if err := os.WriteFile(journalctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "journal.yesh"}
	items, err := parse("journal api.service since 15m lines 20\n", "journal.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil {
		t.Fatal("expected journal diagnostic to fail the query")
	}
}

func TestLinuxJournalUserProfileIsVisibleInArgv(t *testing.T) {
	dir := t.TempDir()
	journalctl := filepath.Join(dir, "journalctl")
	trace := filepath.Join(dir, "trace")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + trace + "'\nprintf '%s\\n' '{\"MESSAGE\":\"ready\"}'\n"
	if err := os.WriteFile(journalctl, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &runtime{values: map[string]value{}, filename: "journal.yesh"}
	items, err := parse("journal user api.service since 15m lines 2\n", "journal.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	want := "--user\n--no-pager\n--output=json\n--unit\napi.service\n--since\n-15m\n--lines\n2\n"
	if string(data) != want {
		t.Fatalf("argv trace = %q, want %q", data, want)
	}
}
