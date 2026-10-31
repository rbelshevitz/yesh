//go:build !windows

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func posixFixture(t *testing.T, body string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "fixture")
	if err := os.WriteFile(name, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return name
}
func posixRuntime() *runtime { return &runtime{values: map[string]value{}, filename: "posix.yesh"} }
func readTestFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func writeTestFile(t *testing.T, name string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(name, data, mode); err != nil {
		t.Fatal(err)
	}
}
func runPOSIX(t *testing.T, r *runtime, command string, fail bool) error {
	t.Helper()
	err := r.runWithin(command, 21, true, 4*time.Second)
	if (err != nil) != fail {
		t.Fatalf("%s: expected failure=%v, got %v", command, fail, err)
	}
	return err
}

func TestPOSIXBinaryStreams(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "input"), filepath.Join(dir, "output")
	data := bytes.Repeat([]byte{0, 1, 2, 10, 13, 127, 128, 255}, 256*1024)
	writeTestFile(t, in, data, 0600)
	for _, n := range []int{1, 2, 3, 8, maxPipelineStages} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			runPOSIX(t, posixRuntime(), "cat < "+in+strings.Repeat(" | cat", n-1)+" > "+out, false)
			if got := readTestFile(t, out); !bytes.Equal(got, data) {
				t.Fatalf("binary stream changed: %d bytes", len(got))
			}
		})
	}
}

func TestPOSIXRedirectionSemantics(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "input with spaces"), filepath.Join(dir, "output with spaces")
	writeTestFile(t, in, []byte("payload\n"), 0600)
	r := posixRuntime()
	r.values["input"] = value{scalar: in}
	r.values["output"] = value{scalar: out}
	runPOSIX(t, r, "cat < $input > $output", false)
	runPOSIX(t, r, "cat < $input >> $output", false)
	if got := string(readTestFile(t, out)); got != "payload\npayload\n" {
		t.Fatal(got)
	}
	runPOSIX(t, r, "true > $output", false)
	if len(readTestFile(t, out)) != 0 {
		t.Fatal("truncate did not empty output")
	}
	runPOSIX(t, r, fmt.Sprintf("cat < %q > %q", in, out), false)
	if got := string(readTestFile(t, out)); got != "payload\n" {
		t.Fatal(got)
	}
}

func TestPOSIXRedirectionRejectsUnsafeFiles(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	original := []byte("do not truncate\n")
	writeTestFile(t, data, original, 0600)
	sym, hard, fifo := filepath.Join(dir, "symlink"), filepath.Join(dir, "hardlink"), filepath.Join(dir, "fifo")
	if err := os.Symlink(data, sym); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(data, hard); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		"cat < " + data + " > " + data,
		"cat < " + sym + " > " + data,
		"true > " + sym,
		"true > " + hard,
		"true >> " + hard,
		"cat < " + fifo,
		"true > " + fifo,
		"true >> " + fifo,
		"cat < " + dir,
		"true > " + dir,
		"cat < /dev/zero",
		"true > /dev/zero",
	} {
		t.Run(command, func(t *testing.T) {
			start := time.Now()
			runPOSIX(t, posixRuntime(), command, true)
			if time.Since(start) > time.Second {
				t.Fatal("special file open blocked")
			}
			if got := readTestFile(t, data); !bytes.Equal(got, original) {
				t.Fatalf("input changed: %q", got)
			}
		})
	}
	// An input symlink to a regular file is valid, unlike output symlinks.
	runPOSIX(t, posixRuntime(), "cat < "+sym+" > /dev/null", false)
}

func TestPOSIXPreflightHasNoExecutionOrTruncation(t *testing.T) {
	dir := t.TempDir()
	marker, out := filepath.Join(dir, "started"), filepath.Join(dir, "out")
	fixture := posixFixture(t, "echo started > "+marker)
	writeTestFile(t, out, []byte("original"), 0600)
	for _, tail := range []string{"/missing-yesh-command", "sh", "xargs rm", "cat < missing", "cat > a > b", ""} {
		t.Run(tail, func(t *testing.T) {
			runPOSIX(t, posixRuntime(), fixture+" | "+tail+" > "+out, true)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("first stage ran: %v", err)
			}
			if got := string(readTestFile(t, out)); got != "original" {
				t.Fatalf("preflight truncated output: %q", got)
			}
		})
	}
	r := posixRuntime()
	r.plan = true
	runPOSIX(t, r, fixture+" > "+out, false)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("plan ran fixture: %v", err)
	}
	if got := string(readTestFile(t, out)); got != "original" {
		t.Fatal("plan truncated output")
	}
}

func TestPOSIXExitCodesAndConditions(t *testing.T) {
	for _, code := range []int{0, 1, 7, 126, 127} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			fixture := posixFixture(t, fmt.Sprintf("exit %d", code))
			r := posixRuntime()
			err := r.runWithin(fixture, 3, false, time.Second)
			if code == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != code {
					t.Fatalf("exit code lost: %v", err)
				}
			}
			ok, err := r.condition(fixture, 3)
			if err != nil || ok != (code == 0) {
				t.Fatalf("condition: %v %v", ok, err)
			}
		})
	}
}

func TestPOSIXPipelineFailureEveryStage(t *testing.T) {
	good := posixFixture(t, "cat")
	bad := posixFixture(t, "exit 7")
	for _, index := range []int{0, 1, 2} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			stages := []string{good, good, good}
			stages[index] = bad
			err := runPOSIX(t, posixRuntime(), strings.Join(stages, " | ")+" < /dev/null", true)
			// '<' belongs to the last stage here, so also run a correctly redirected pipeline.
			if !strings.Contains(err.Error(), "first stage") {
				t.Fatal(err)
			}
			runPOSIX(t, posixRuntime(), stages[0]+" < /dev/null | "+stages[1]+" | "+stages[2]+" > /dev/null", true)
		})
	}
}

func TestPOSIXPipelineFailureReportsStageAndExitCode(t *testing.T) {
	good := posixFixture(t, "cat")
	bad := posixFixture(t, "exit 7")
	err := runPOSIX(t, posixRuntime(), good+" < /dev/null | "+bad+" | "+good+" > /dev/null", true)
	if !strings.Contains(err.Error(), "pipeline stage 2/3") || !strings.Contains(err.Error(), "exit code 7") {
		t.Fatalf("pipeline failure lost stage or exit code: %v", err)
	}
}

func TestPOSIXCommandDiagnosticPreservesStageExitAndStderr(t *testing.T) {
	fixture := posixFixture(t, "echo diagnostic >&2; exit 23")
	err := runPOSIX(t, posixRuntime(), fixture, true)
	var diagnostic *commandDiagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatalf("structured command diagnostic was lost: %v", err)
	}
	if diagnostic.Stage != 1 || diagnostic.Stages != 1 || diagnostic.ExitCode != 23 || diagnostic.Signal != "" || diagnostic.TimedOut || diagnostic.Stderr != "diagnostic\n" {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("original exit error was lost: %v", err)
	}
	if len(diagnostic.Argv) != 1 || diagnostic.Argv[0] != fixture {
		t.Fatalf("unexpected argv: %#v", diagnostic.Argv)
	}
}

func TestPOSIXCommandDiagnosticBoundsStderr(t *testing.T) {
	fixture := posixFixture(t, "head -c 1048577 /dev/zero >&2; exit 1")
	err := runPOSIX(t, posixRuntime(), fixture, true)
	var diagnostic *commandDiagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatalf("structured command diagnostic was lost: %v", err)
	}
	if len(diagnostic.Stderr) != maxDiagnosticStderr || !diagnostic.StderrTruncated {
		t.Fatalf("stderr was not bounded: len=%d truncated=%v", len(diagnostic.Stderr), diagnostic.StderrTruncated)
	}
}

func TestPOSIXPipelineFirstFailureLastSuccess(t *testing.T) {
	bad := posixFixture(t, "exit 7")
	good := posixFixture(t, "cat")
	err := runPOSIX(t, posixRuntime(), bad+" < /dev/null | "+good+" > /dev/null", true)
	if !strings.Contains(err.Error(), "pipeline stage 1/2") || !strings.Contains(err.Error(), "exit code 7") {
		t.Fatalf("first-stage failure was hidden by successful last stage: %v", err)
	}
}

func TestPOSIXFailureStopsHungPeer(t *testing.T) {
	hang := posixFixture(t, `trap '' TERM; while :; do :; done`)
	fail := posixFixture(t, "exit 9")
	for _, command := range []string{hang + " | " + fail, fail + " | " + hang} {
		t.Run(command, func(t *testing.T) {
			start := time.Now()
			err := posixRuntime().runWithin(command, 9, false, 10*time.Second)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 9 {
				t.Fatalf("original failure lost: %v", err)
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("waited for job timeout: %s", elapsed)
			}
		})
	}
}

func TestPOSIXSignalsAreExecutionErrors(t *testing.T) {
	for _, signal := range []string{"TERM", "INT", "HUP", "PIPE"} {
		t.Run(signal, func(t *testing.T) {
			fixture := posixFixture(t, "kill -"+signal+" $$")
			r := posixRuntime()
			err := r.runWithin(fixture, 3, false, time.Second)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ProcessState.Exited() {
				t.Fatalf("signal was not preserved: %v", err)
			}
			if _, err := r.condition(fixture, 3); err == nil {
				t.Fatal("signal treated as false")
			}
		})
	}
}

func TestPOSIXCancellation(t *testing.T) {
	for _, when := range []string{"before", "running"} {
		t.Run(when, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if when == "before" {
				cancel()
			} else {
				timer := time.AfterFunc(100*time.Millisecond, cancel)
				defer timer.Stop()
			}
			r := posixRuntime()
			r.ctx = ctx
			err := r.runWithin("sleep 30", 3, false, 3*time.Second)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			if when == "before" {
				if err := r.run("true", 3, true); !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled builtin: %v", err)
				}
			}
		})
	}
}

func TestPOSIXTimeoutIsNotFalse(t *testing.T) {
	r := posixRuntime()
	err := r.runWithin("sleep 30", 3, false, 50*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline lost: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r.ctx = ctx
	if _, err := r.condition("sleep 30", 3); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout condition: %v", err)
	}
}

func TestPOSIXPartialStartReaps(t *testing.T) {
	// LookPath accepts this executable; exec fails because its interpreter is missing.
	broken := filepath.Join(t.TempDir(), "broken")
	writeTestFile(t, broken, []byte("#!/missing-yesh-interpreter\n"), 0700)
	for _, n := range []int{1, 2, 5} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			command := strings.Repeat("sleep 30 | ", n) + broken
			start := time.Now()
			runPOSIX(t, posixRuntime(), command, true)
			if time.Since(start) > 2*time.Second {
				t.Fatal("partial start cleanup stalled")
			}
			var status syscall.WaitStatus
			if pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil); err != syscall.ECHILD {
				t.Fatalf("unreaped child: pid=%d err=%v", pid, err)
			}
		})
	}
}

func TestPOSIXStubbornWriterAfterReaderExit(t *testing.T) {
	writer := posixFixture(t, `trap '' PIPE TERM
exec 2>/dev/null
while :; do printf 'payload\n'; done`)
	start := time.Now()
	err := posixRuntime().runWithin(writer+" | head -n 1 > /dev/null", 7, false, 200*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stubborn writer timeout: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("writer cleanup stalled: %s", elapsed)
	}
}

func TestPOSIXPlanDoesNotOpenFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	r := posixRuntime()
	r.plan = true
	runPOSIX(t, r, "cat < "+fifo+" > /dev/null", false)
}

func TestPOSIXEmptyProcessGroupIsSafe(t *testing.T) {
	group := &processGroup{}
	if err := group.terminate(); err != nil {
		t.Fatal(err)
	}
	if err := group.kill(); err != nil {
		t.Fatal(err)
	}
}

func TestPOSIXBackpressureWithSlowReader(t *testing.T) {
	in, out := filepath.Join(t.TempDir(), "input"), filepath.Join(t.TempDir(), "output")
	data := bytes.Repeat([]byte("record\n"), 100000)
	writeTestFile(t, in, data, 0600)
	reader := posixFixture(t, "sleep 0.1; cat")
	runPOSIX(t, posixRuntime(), "cat < "+in+" | "+reader+" > "+out, false)
	if !bytes.Equal(readTestFile(t, out), data) {
		t.Fatal("backpressure lost bytes")
	}
}

func TestPOSIXExecFormatNeverFallsBackToShell(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	program := filepath.Join(dir, "not-a-binary")
	writeTestFile(t, program, []byte("touch "+marker+"\n"), 0700)
	runPOSIX(t, posixRuntime(), program, true)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("implicit shell fallback executed file: %v", err)
	}
}
