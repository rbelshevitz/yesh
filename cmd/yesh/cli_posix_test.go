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
	"strings"
	"syscall"
	"testing"
	"time"
)

// Exercise the real CLI without building an extra binary or altering the
// parent test's global stdin, environment, cwd, or signal handlers.
func TestPOSIXCLIHelper(t *testing.T) {
	if os.Getenv("YESH_TEST_CLI") != "1" {
		return
	}
	if os.Getenv("YESH_TEST_UMASK") == "077" {
		syscall.Umask(0077)
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"yesh"}, os.Args[i+1:]...)
			os.Exit(runCLI())
		}
	}
	os.Exit(99)
}
func posixCLI(t *testing.T, ctx context.Context, args ...string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, exe, append([]string{"-test.run=^TestPOSIXCLIHelper$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "YESH_TEST_CLI=1")
	cmd.WaitDelay = 3 * time.Second
	return cmd
}

func TestPOSIXCronEnvironment(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "nightly.yesh")
	source := `/usr/bin/printf '%s\n' 'cron payload' | /bin/cat
/usr/bin/env
/bin/pwd
`
	writeTestFile(t, script, []byte(source), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := posixCLI(t, ctx, script)
	cmd.Env = []string{"YESH_TEST_CLI=1", "PATH=/usr/bin:/bin", "HOME=" + dir, "LOGNAME=test", "SHELL=/bin/sh", "LC_ALL=C"}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cron-like execution: %v\n%s", err, out)
	}
	for _, want := range []string{"cron payload\n", "HOME=" + dir, "PATH=/usr/bin:/bin", dir + "\n"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
}

func TestPOSIXCLIStdinAndStderr(t *testing.T) {
	fixture := posixFixture(t, "printf diagnostic >&2; cat")
	script := filepath.Join(t.TempDir(), "stdio.yesh")
	writeTestFile(t, script, []byte(fixture+" | cat\n"), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := posixCLI(t, ctx, script)
	cmd.Stdin = strings.NewReader("input with spaces\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "input with spaces\n" || stderr.String() != "diagnostic" {
		t.Fatalf("stdio mixed/lost: out=%q err=%q", stdout.String(), stderr.String())
	}
}

func TestPOSIXCLIScriptSizeLimit(t *testing.T) {
	script := filepath.Join(t.TempDir(), "oversized.yesh")
	writeTestFile(t, script, bytes.Repeat([]byte{'#'}, maxScriptBytes+1), 0600)
	out, err := posixCLI(t, context.Background(), script).CombinedOutput()
	if err == nil {
		t.Fatal("oversized script unexpectedly succeeded")
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("oversized script exit=%v output=%q", err, out)
	}
	if !bytes.Contains(out, []byte("script exceeds")) {
		t.Fatalf("oversized script diagnostic=%q", out)
	}
}

func TestPOSIXCLIExitAndStop(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		code         int
	}{
		{"success", "true\n", 0}, {"failure", "false\n", 1}, {"parse", "while true:\n    true\n", 1},
		{"usage", "", 2}, {"missing-script", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := filepath.Join(dir, "script")
			marker := filepath.Join(dir, "marker")
			source := tc.source + "touch " + marker + "\n"
			writeTestFile(t, script, []byte(source), 0600)
			args := []string{script}
			if tc.name == "usage" {
				args = nil
			}
			if tc.name == "missing-script" {
				args = []string{filepath.Join(dir, "missing")}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			out, err := posixCLI(t, ctx, args...).CombinedOutput()
			code := 0
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tc.code {
				t.Fatalf("code=%d want=%d: %s", code, tc.code, out)
			}
			_, statErr := os.Stat(marker)
			if (statErr == nil) != (tc.code == 0) {
				t.Fatalf("unexpected following command: %v", statErr)
			}
		})
	}
}

func TestPOSIXCLIPlanExpandedArguments(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	script := filepath.Join(dir, "plan.yesh")
	writeTestFile(t, script, []byte(fmt.Sprintf("args = [one, 'two words']\nprintf $args > %q\n", marker)), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := posixCLI(t, ctx, "--plan", script).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(string(out), `["printf" "one" "two words"]`) {
		t.Fatalf("opaque argv: %s", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("plan created file: %v", err)
	}
}

func TestPOSIXCLIForwardsSignals(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			pidfile := filepath.Join(dir, "pid")
			script := filepath.Join(dir, "script")
			fixture := posixFixture(t, `echo $$ > "$1"; exec sleep 30`)
			writeTestFile(t, script, []byte(fixture+" "+pidfile+"\n"), 0600)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			cmd := posixCLI(t, ctx, script)
			var output bytes.Buffer
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			// Keep cleanup bounded even if readiness or forwarding fails.
			defer func() { _ = cmd.Process.Kill() }()
			var child int
			until := time.Now().Add(3 * time.Second)
			for time.Now().Before(until) {
				data, _ := os.ReadFile(pidfile)
				if _, err := fmt.Sscanf(string(data), "%d", &child); err == nil && child > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if child == 0 {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatal("child did not become ready")
			}
			defer syscall.Kill(-child, syscall.SIGKILL)
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err == nil {
				t.Fatal("interrupted CLI succeeded")
			}
			if ctx.Err() != nil {
				t.Fatalf("signal forwarding stalled: %s", output.String())
			}
			if err := syscall.Kill(-child, 0); err != syscall.ESRCH {
				t.Fatalf("child group survived %s: %v", sig, err)
			}
		})
	}
}

func TestPOSIXMissingPathAndRelativeExecutable(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "script")
	writeTestFile(t, script, []byte("cat < /dev/null\n"), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := posixCLI(t, ctx, script)
	cmd.Env = []string{"YESH_TEST_CLI=1", "PATH="}
	if out, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(out, []byte("resolve cat")) {
		t.Fatalf("missing PATH: %v %s", err, out)
	}
	// PATH=. must not silently pick a writable working-directory executable.
	writeTestFile(t, filepath.Join(dir, "fake"), []byte("#!/bin/sh\nexit 0\n"), 0700)
	writeTestFile(t, script, []byte("fake\n"), 0600)
	cmd = posixCLI(t, ctx, script)
	cmd.Dir = dir
	cmd.Env = []string{"YESH_TEST_CLI=1", "PATH=."}
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("implicit cwd executable accepted: %s", out)
	}
	// An explicitly chosen relative executable is allowed.
	writeTestFile(t, script, []byte("./fake\n"), 0600)
	cmd = posixCLI(t, ctx, script)
	cmd.Dir = dir
	cmd.Env = []string{"YESH_TEST_CLI=1", "PATH="}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("explicit executable: %v %s", err, out)
	}
}

func TestPOSIXCLIExplicitExitCodes(t *testing.T) {
	for _, code := range []int{0, 2, 7, 125, 255} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			dir := t.TempDir()
			script, marker := filepath.Join(dir, "script"), filepath.Join(dir, "after")
			writeTestFile(t, script, []byte(fmt.Sprintf("if true:\n    exit %d\ntouch %s\n", code, marker)), 0600)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			out, err := posixCLI(t, ctx, script).CombinedOutput()
			got := 0
			if exit, ok := err.(*exec.ExitError); ok {
				got = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if got != code || len(out) != 0 {
				t.Fatalf("exit=%d want=%d output=%q", got, code, out)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("execution continued: %v", err)
			}
		})
	}
}

func TestPOSIXExistsPermissionAndLinkErrors(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, "hidden")
	if err := os.Mkdir(hidden, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(hidden, "file")
	writeTestFile(t, path, []byte("data"), 0600)
	if os.Geteuid() != 0 {
		if err := os.Chmod(hidden, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(hidden, 0700)
		r := posixRuntime()
		r.values["path"] = value{scalar: path}
		if _, err := r.condition("exists $path", 3); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("EACCES hidden as false: %v", err)
		}
	}
	loop := filepath.Join(dir, "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	r := posixRuntime()
	r.values["path"] = value{scalar: loop}
	if _, err := r.condition("exists $path", 3); err == nil {
		t.Fatal("symlink loop treated as absence")
	}
	dangling := filepath.Join(dir, "dangling")
	if err := os.Symlink("missing", dangling); err != nil {
		t.Fatal(err)
	}
	r.values["path"] = value{scalar: dangling}
	if ok, err := r.condition("exists $path", 3); err != nil || ok {
		t.Fatalf("dangling target: %v %v", ok, err)
	}
}

func TestPOSIXPlanConditionWithQuotedMatches(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	script := filepath.Join(dir, "script")
	fixture := posixFixture(t, "touch "+marker)
	source := fmt.Sprintf("message = 'a matches b'\nif %s $message:\n    echo yes\nelse:\n    echo no\n", fixture)
	writeTestFile(t, script, []byte(source), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := posixCLI(t, ctx, "--plan", script).CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("PLAN IF")) || !bytes.Contains(out, []byte("PLAN ELSE")) {
		t.Fatalf("%v %s", err, out)
	}
	if !bytes.Contains(out, []byte(`"a matches b"`)) {
		t.Fatalf("condition arguments not expanded in plan: %s", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("condition executed in plan: %v", err)
	}
}
