//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Fixtures use /bin/sh only to arrange real Unix processes, never as a
// runtime backend. Every runtime invocation has a bounded deadline.
func linuxFixture(t *testing.T, body string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "fixture")
	if err := os.WriteFile(name, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return name
}

func linuxRuntime() *runtime {
	return &runtime{values: map[string]value{}, filename: "linux-test.yesh"}
}

func TestLinuxProcessGroup(t *testing.T) {
	out := filepath.Join(t.TempDir(), "pids")
	fixture := linuxFixture(t, `printf '%s ' "$$"; ps -o pgid= -p "$$"; cat`)
	err := linuxRuntime().runWithin(fmt.Sprintf("%s | %s > %s", fixture, fixture, out), 1, true, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(data))
	if len(fields) != 4 {
		t.Fatalf("pid/pgid pairs: %q", data)
	}
	if fields[1] != fields[3] || fields[2] != fields[3] {
		t.Fatalf("pipeline must share first stage's process group: %q", data)
	}
	pgid, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatal(err)
	}
	if pgid == syscall.Getpgrp() {
		t.Fatal("job shares test runner's process group")
	}
	for _, field := range []string{fields[0], fields[2]} {
		pid, _ := strconv.Atoi(field)
		var status syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil); err != syscall.ECHILD {
			t.Fatalf("pid %d was not reaped: %v", pid, err)
		}
	}
}

func TestLinuxTimeoutEscalation(t *testing.T) {
	for _, suffix := range []string{"", " | cat"} {
		t.Run("command"+suffix, func(t *testing.T) {
			pidfile := filepath.Join(t.TempDir(), "pid")
			fixture := linuxFixture(t, `trap '' TERM; echo $$ > "$1"; while :; do :; done`)
			start := time.Now()
			err := linuxRuntime().runWithin(fixture+" "+pidfile+suffix, 1, true, 500*time.Millisecond)
			if err == nil {
				t.Fatal("timeout must fail")
			}
			if elapsed := time.Since(start); elapsed < 2*time.Second || elapsed > 6*time.Second {
				t.Fatalf("unexpected escalation duration: %s", elapsed)
			}
			data, err := os.ReadFile(pidfile)
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Kill(-pid, 0); err != syscall.ESRCH {
				t.Fatalf("process group survived timeout: %v", err)
			}
		})
	}
}

func TestLinuxSIGPIPE(t *testing.T) {
	for _, name := range []string{"yes", "head"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip(err)
		}
	}
	start := time.Now()
	err := linuxRuntime().runWithin("yes | head -n 1 > /dev/null", 1, false, 5*time.Second)
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected writer signal failure, got %v", err)
	}
	status := exit.Sys().(syscall.WaitStatus)
	if !status.Signaled() || status.Signal() != syscall.SIGPIPE {
		t.Fatalf("expected SIGPIPE: %v", status)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("writer waited for timeout")
	}
}

func TestLinuxTimeoutDescendantCleanup(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "child")
	fixture := linuxFixture(t, `trap 'wait; exit 0' TERM
sleep 30 &
echo $! > "$1"
wait`)
	err := linuxRuntime().runWithin(fixture+" "+pidfile, 1, true, 500*time.Millisecond)
	if err == nil {
		t.Fatal("timeout must fail even when leader handles TERM successfully")
	}
	data, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("descendant survived group termination or was not reaped: %v", err)
	}
}

func TestLinuxCleanupDescriptors(t *testing.T) {
	fixture := linuxFixture(t, "cat")
	out := filepath.Join(t.TempDir(), "out")
	count := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	// Warm up Go's process/context poller before measuring descriptors.
	r := linuxRuntime()
	if err := r.runWithin(fixture+" < /dev/null > "+out, 1, true, time.Second); err != nil {
		t.Fatal(err)
	}
	before := count()
	for i := 0; i < 20; i++ {
		for _, tc := range []struct {
			command string
			fails   bool
		}{
			{fixture + " < /dev/null | " + fixture + " > " + out, false},
			{fixture + " < /dev/null | /nonexistent-yesh-test-command > " + out, true},
			{fixture + " < /dev/null > " + out + "/missing", true},
		} {
			err := r.runWithin(tc.command, 1, true, time.Second)
			if tc.fails != (err != nil) {
				t.Fatalf("%s: %v", tc.command, err)
			}
		}
	}
	if after := count(); after != before {
		t.Fatalf("descriptor leak: before=%d after=%d", before, after)
	}
}

func TestLinuxRepeatedCleanupAfterCancellation(t *testing.T) {
	count := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	r := linuxRuntime()
	if err := r.runWithin("cat < /dev/null | cat > /dev/null", 1, true, time.Second); err != nil {
		t.Fatal(err)
	}
	beforeFD, beforeGo := count(), goruntime.NumGoroutine()
	for i := 0; i < 12; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		r.ctx = ctx
		err := r.runWithin("sleep 30 | cat > /dev/null", 1, false, time.Second)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
	until := time.Now().Add(2 * time.Second)
	for goruntime.NumGoroutine() > beforeGo+1 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if after := count(); after != beforeFD {
		t.Fatalf("FD leak after cancellations: %d -> %d", beforeFD, after)
	}
	if after := goruntime.NumGoroutine(); after > beforeGo+1 {
		t.Fatalf("goroutine leak after cancellations: %d -> %d", beforeGo, after)
	}
	var status syscall.WaitStatus
	if pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil); err != syscall.ECHILD {
		t.Fatalf("unreaped child: %d %v", pid, err)
	}
}
