//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
