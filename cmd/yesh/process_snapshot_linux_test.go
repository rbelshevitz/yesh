//go:build linux

package main

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestReadProcessInfoSelf(t *testing.T) {
	info, err := readProcessInfo(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if info.pid != os.Getpid() || info.name == "" || info.startTime == 0 {
		t.Fatalf("invalid identity: %+v", info)
	}
	if info.uid == 0 && os.Getuid() != 0 {
		t.Fatalf("unexpected uid: %+v", info)
	}
	if info.rssBytes == 0 || info.cgroup == "" {
		t.Fatalf("missing resource fields: %+v", info)
	}
}

func TestParseProcStatPreservesComplexName(t *testing.T) {
	line := "42 (worker (backup)) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22"
	info, err := parseProcStat(line)
	if err != nil {
		t.Fatal(err)
	}
	if info.name != "worker (backup)" || info.state != 'S' || info.ppid != 1 || info.startTime != 19 {
		t.Fatalf("unexpected stat: %+v", info)
	}
}

func TestReadProcessInfoMissingPID(t *testing.T) {
	_, err := readProcessInfo(999999999)
	if err == nil || !strings.Contains(err.Error(), strconv.Itoa(999999999)) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListProcessInfoIsBoundedAndIncludesSelf(t *testing.T) {
	processes, err := listProcessInfo(128)
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) == 0 || len(processes) > 128 {
		t.Fatalf("unexpected process count: %d", len(processes))
	}
	found := false
	for _, process := range processes {
		if process.pid == os.Getpid() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("snapshot does not contain test process: %+v", processes)
	}
}

func TestListProcessInfoRejectsUnsafeLimit(t *testing.T) {
	for _, limit := range []int{0, maxProcessSnapshot + 1} {
		if _, err := listProcessInfo(limit); err == nil {
			t.Fatalf("limit %d was accepted", limit)
		}
	}
}

func TestFindProcessInfoUsesExactName(t *testing.T) {
	self, err := readProcessInfo(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	processes, err := findProcessInfo(self.name, 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) == 0 {
		t.Fatalf("no process matched %q", self.name)
	}
	for _, process := range processes {
		if process.name != self.name {
			t.Fatalf("non-exact match: %+v", process)
		}
	}
	if _, err := findProcessInfo("", 128); err == nil {
		t.Fatal("empty process name was accepted")
	}
}

func TestLinuxPracticalNginxProcessSet(t *testing.T) {
	processes, err := findProcessInfo("nginx", 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) == 0 {
		t.Skip("nginx is not running")
	}
	seen := make(map[int]struct{}, len(processes))
	for _, process := range processes {
		if _, exists := seen[process.pid]; exists {
			t.Fatalf("duplicate PID in snapshot: %+v", process)
		}
		seen[process.pid] = struct{}{}
		if process.startTime == 0 || process.rssBytes == 0 {
			t.Fatalf("incomplete nginx process identity: %+v", process)
		}
	}
	t.Logf("found %d nginx processes", len(processes))
}

func TestProcessQueryReadsExactName(t *testing.T) {
	self, err := readProcessInfo(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	r := &runtime{values: map[string]value{}, filename: "processes.yesh"}
	v, err := r.eval("processes where name is "+strconv.Quote(self.name), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !v.isProcesses || len(v.processes) == 0 {
		t.Fatalf("unexpected process set: %#v", v)
	}
}

func TestProcessTableRejectsUnknownField(t *testing.T) {
	var output bytes.Buffer
	if err := writeProcessTable(&output, nil, []string{"pid", "command_line"}); err == nil {
		t.Fatal("unknown process field was accepted")
	}
}

func TestProcessSummaryCalculatesRSSWithoutOverflow(t *testing.T) {
	var output bytes.Buffer
	processes := []processInfo{{rssBytes: 10}, {rssBytes: 30}}
	if err := writeProcessSummary(&output, processes, "rss"); err != nil {
		t.Fatal(err)
	}
	want := "PROCESS_COUNT\t2\nRSS_TOTAL\t40B (40 bytes)\nRSS_MAX\t30B (30 bytes)\nRSS_AVERAGE\t20B (20 bytes)\n"
	if output.String() != want {
		t.Fatalf("summary = %q, want %q", output.String(), want)
	}
	if err := writeProcessSummary(&output, processes, "pss"); err == nil {
		t.Fatal("unsupported memory metric was accepted")
	}
}

func TestProcessMemoryConditionUsesRSSAggregate(t *testing.T) {
	r := &runtime{
		values:   map[string]value{"workers": {isProcesses: true, processes: []processInfo{{rssBytes: 10}, {rssBytes: 30}}}},
		filename: "memory.yesh",
	}
	ok, err := r.condition("memory $workers rss at_most 40B", 1)
	if err != nil || !ok {
		t.Fatalf("at-most condition: ok=%v err=%v", ok, err)
	}
	ok, err = r.condition("memory $workers rss at_most 39B", 2)
	if err != nil || ok {
		t.Fatalf("failing at-most condition: ok=%v err=%v", ok, err)
	}
	if _, err := r.condition("memory $missing rss at_most 40B", 3); err == nil {
		t.Fatal("missing process set was accepted")
	}
}
