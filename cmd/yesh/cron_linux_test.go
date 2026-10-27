//go:build linux

package main

import (
	"bytes"
	"context"
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

func TestLinuxCronLockConflict(t *testing.T) {
	flock, err := exec.LookPath("flock")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	lock := filepath.Join(dir, "lock")
	marker := filepath.Join(dir, "started")
	script := filepath.Join(dir, "script")
	writeTestFile(t, script, []byte("touch "+marker+"\n"), 0600)
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	helper := posixCLI(t, ctx, script)
	args := append([]string{"-n", "-E", "75", lock}, helper.Args...)
	cmd := exec.CommandContext(ctx, flock, args...)
	cmd.Env = helper.Env
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 75 {
		t.Fatalf("lock conflict: %v %s", err, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("overlapping cron job started: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	cmd = exec.CommandContext(ctx, flock, args...)
	cmd.Env = helper.Env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unlocked job: %v %s", err, out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("unlocked job did not run", err)
	}
}

func TestLinuxCronOuterTimeout(t *testing.T) {
	timeout, err := exec.LookPath("timeout")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "pid")
	script := filepath.Join(dir, "script")
	fixture := posixFixture(t, `echo $$ > "$1"; exec sleep 30`)
	writeTestFile(t, script, []byte(fixture+" "+pidfile+"\n"), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	helper := posixCLI(t, ctx, script)
	args := append([]string{"--signal=TERM", "--kill-after=4s", "1s"}, helper.Args...)
	cmd := exec.CommandContext(ctx, timeout, args...)
	cmd.Env = helper.Env
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 124 {
		t.Fatalf("cron timeout: %v %s", err, out)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(readTestFile(t, pidfile))))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(-pid, syscall.SIGKILL)
	if err := syscall.Kill(-pid, 0); err != syscall.ESRCH {
		t.Fatalf("cron timeout left job group: %v", err)
	}
}

func TestLinuxPackageReadOnlyQueries(t *testing.T) {
	for _, name := range []string{"dpkg-query", "apt-cache"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip(err)
		}
	}
	out := filepath.Join(t.TempDir(), "packages")
	r := posixRuntime()
	runPOSIX(t, r, `dpkg-query --show --showformat='${binary:Package}\t${Version}\t${db:Status-Status}\n' dpkg > `+out, false)
	data := readTestFile(t, out)
	if !bytes.HasPrefix(data, []byte("dpkg\t")) || !bytes.HasSuffix(data, []byte("\tinstalled\n")) {
		t.Fatalf("query format damaged: %q", data)
	}
	runPOSIX(t, r, "apt-cache policy dpkg > "+out, false)
	if !bytes.Contains(readTestFile(t, out), []byte("dpkg:")) {
		t.Fatal("apt-cache output missing package")
	}
}

func TestLinuxUnrelatedProcessSurvivesCancellation(t *testing.T) {
	peer := exec.Command("sleep", "30")
	peer.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := peer.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = peer.Process.Kill(); _ = peer.Wait() }()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r := posixRuntime()
	r.ctx = ctx
	if err := r.runWithin("sleep 30", 3, true, time.Second); err == nil {
		t.Fatal("cancelled job succeeded")
	}
	if err := peer.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("unrelated group killed: %v", err)
	}
}

func TestLinuxChildDoesNotInheritUnrelatedDescriptors(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "parent-only")
	f, err := os.OpenFile(secret, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fixture := posixFixture(t, `for fd in /proc/self/fd/*; do readlink "$fd"; done; exit 0`)
	out := filepath.Join(t.TempDir(), "fds")
	runPOSIX(t, posixRuntime(), fmt.Sprintf("%s > %s", fixture, out), false)
	if bytes.Contains(readTestFile(t, out), []byte(secret)) {
		t.Fatal("parent descriptor inherited by child")
	}
}
