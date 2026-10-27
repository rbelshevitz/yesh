//go:build !windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestPOSIXChmodArgumentsAndModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file with spaces")
	writeTestFile(t, path, []byte("contents"), 0600)
	r := posixRuntime()
	r.values["target"] = value{scalar: path}
	for _, tc := range []struct {
		mode string
		want os.FileMode
	}{
		{"0600", 0600}, {"0640", 0640}, {"0750", 0750}, {"u=rw,g=r,o=", 0640},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			r.values["mode"] = value{scalar: tc.mode}
			runPOSIX(t, r, "chmod mode target", false)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != tc.want {
				t.Fatalf("mode=%04o want=%04o", info.Mode().Perm(), tc.want)
			}
			if string(readTestFile(t, path)) != "contents" {
				t.Fatal("chmod changed content")
			}
		})
	}
	runPOSIX(t, r, "chmod invalid-mode target", true)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Fatal("failed chmod changed permissions")
	}
}

func TestPOSIXChownExistingIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owned file")
	writeTestFile(t, path, []byte("contents"), 0600)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat := info.Sys().(*syscall.Stat_t)
	command := fmt.Sprintf("chown %d:%d target", stat.Uid, stat.Gid)
	r := posixRuntime()
	r.values["target"] = value{scalar: path}
	runPOSIX(t, r, command, false)
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	got := after.Sys().(*syscall.Stat_t)
	if got.Uid != stat.Uid || got.Gid != stat.Gid {
		t.Fatal("ownership changed unexpectedly")
	}
	if string(readTestFile(t, path)) != "contents" {
		t.Fatal("chown changed content")
	}
}

func TestPOSIXRedirectionHonorsUmask(t *testing.T) {
	dir := t.TempDir()
	script, out := filepath.Join(dir, "script"), filepath.Join(dir, "output")
	writeTestFile(t, script, []byte("cat < /dev/null > "+out+"\n"), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := posixCLI(t, ctx, script)
	cmd.Env = append(cmd.Env, "YESH_TEST_UMASK=077")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, output)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("creation ignored umask: %04o", info.Mode().Perm())
	}
}

func TestPOSIXStickyDirectoryBit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	r := posixRuntime()
	r.values["target"] = value{scalar: dir}
	runPOSIX(t, r, "chmod 1777 target", false)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0777 || info.Mode()&os.ModeSticky == 0 {
		t.Fatalf("sticky lost: %v", info.Mode())
	}
	// Creation and deletion by the same owner remain possible in a sticky directory.
	file := filepath.Join(dir, "owned")
	writeTestFile(t, file, []byte("contents"), 0600)
	if err := os.Remove(file); err != nil {
		t.Fatalf("sticky blocked owner's unlink: %v", err)
	}
	runPOSIX(t, r, "chmod -t target", false)
	info, err = os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSticky != 0 {
		t.Fatalf("sticky was not cleared: %v", info.Mode())
	}
}

func TestPOSIXStickyForeignEntries(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs a second UID; no privilege escalation is performed")
	}
	base, err := os.MkdirTemp("", "yesh-sticky-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	if err := os.Chmod(base, 0755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "shared")
	if err := os.Mkdir(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, os.ModeSticky|0777); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "root-owned")
	writeTestFile(t, file, []byte("contents"), 0600)
	actor := func(name string, args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		return cmd.Run()
	}
	if err := actor("true"); err != nil {
		t.Skipf("second UID unavailable: %v", err)
	}
	for _, operation := range []string{"remove", "rename"} {
		t.Run(operation, func(t *testing.T) {
			var err error
			if operation == "remove" {
				err = actor("rm", "--", file)
			} else {
				err = actor("mv", "--", file, file+"-renamed")
			}
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("expected denied %s, got %v", operation, err)
			}
			if _, err := os.Stat(file); err != nil {
				t.Fatalf("foreign entry changed: %v", err)
			}
		})
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if err := actor("rm", "--", file); err != nil {
		t.Fatalf("unlink without sticky should succeed: %v", err)
	}
}
