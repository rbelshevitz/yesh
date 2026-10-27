//go:build !windows

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func practicalExample(t *testing.T, name string) string {
	t.Helper()
	name = filepath.Join("..", "..", "examples", name)
	path, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return path
}
func practicalDependencies(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			packageName := map[string]string{"pv": "pv", "rsnapshot": "rsnapshot", "rsync": "rsync", "tar": "tar", "gzip": "gzip", "install": "coreutils", "grep": "grep", "mv": "coreutils", "cat": "coreutils", "cp": "coreutils", "rm": "coreutils", "sort": "coreutils", "df": "coreutils", "du": "coreutils", "findmnt": "util-linux", "lsinitramfs": "initramfs-tools-core", "cpio": "cpio", "zstd": "zstd"}[name]
			t.Skipf("missing utility %s; install package %s on Debian/Ubuntu to run this practical test: %v", name, packageName, err)
		}
	}
}
func practicalCommand(t *testing.T, dir, script string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	t.Cleanup(cancel)
	cmd := posixCLI(t, ctx, script)
	cmd.Dir = dir
	return cmd
}

func TestPOSIXPracticalBackupRoundTrip(t *testing.T) {
	practicalDependencies(t, "tar", "gzip", "mv")
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"settings.ini": []byte("mode=production\n"), "file with spaces": []byte("payload\n"), "binary": {0, 1, 10, 128, 255}}
	for name, data := range files {
		writeTestFile(t, filepath.Join(source, name), data, 0600)
	}
	cmd := practicalCommand(t, dir, practicalExample(t, "backup.yesh"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("backup: %v\n%s", err, out)
	}
	archive, err := os.Open(filepath.Join(dir, "backup.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	restored := map[string][]byte{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(header.Name, "./"), "source/")
		data, err := io.ReadAll(io.LimitReader(reader, 1024*1024))
		if err != nil {
			t.Fatal(err)
		}
		restored[name] = data
	}
	// Drain the gzip stream so its checksum/trailer is checked as well.
	if _, err := io.Copy(io.Discard, gz); err != nil {
		t.Fatal(err)
	}
	if len(restored) != len(files) {
		t.Fatalf("archive members: %v", restored)
	}
	for name, want := range files {
		if !bytes.Equal(restored[name], want) {
			t.Fatalf("restore mismatch for %q", name)
		}
	}
	var state struct {
		Schema  int    `json:"schema"`
		Status  string `json:"status"`
		Archive string `json:"archive"`
	}
	decoder := json.NewDecoder(bytes.NewReader(readTestFile(t, filepath.Join(dir, "backup-state.json"))))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state.Schema != 1 || state.Status != "complete" || state.Archive != "backup.tar.gz" {
		t.Fatalf("bad completion state: %+v", state)
	}
	for _, name := range []string{"backup.tar.gz.partial", "backup-state.json.partial"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("unpublished partial remains: %s %v", name, err)
		}
	}
}

func TestPOSIXPracticalBackupDoesNotPublishFailure(t *testing.T) {
	practicalDependencies(t, "tar", "gzip", "mv")
	if os.Geteuid() == 0 {
		t.Skip("needs unprivileged permission failure")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(source, "data"), []byte("new"), 0600)
	if err := os.Chmod(source, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(source, 0700)
	writeTestFile(t, filepath.Join(dir, "backup.tar.gz"), []byte("previous archive"), 0600)
	writeTestFile(t, filepath.Join(dir, "backup-state.json"), []byte("previous state"), 0600)
	out, err := practicalCommand(t, dir, practicalExample(t, "backup.yesh")).CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("expected pipeline failure: %v %s", err, out)
	}
	if got := string(readTestFile(t, filepath.Join(dir, "backup.tar.gz"))); got != "previous archive" {
		t.Fatalf("failed backup replaced archive: %q", got)
	}
	if got := string(readTestFile(t, filepath.Join(dir, "backup-state.json"))); got != "previous state" {
		t.Fatalf("failed backup published success: %q", got)
	}
}

func TestPOSIXPracticalConfigValidation(t *testing.T) {
	practicalDependencies(t, "grep", "install", "mv")
	for _, tc := range []struct {
		name, source string
		code         int
	}{
		{"valid", "mode=production\n", 0}, {"invalid", "mode=development\n", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			active := filepath.Join(dir, "active.conf")
			writeTestFile(t, active, []byte("previous\n"), 0600)
			writeTestFile(t, filepath.Join(dir, "staged.conf"), []byte(tc.source), 0600)
			out, err := practicalCommand(t, dir, practicalExample(t, "config-apply.yesh")).CombinedOutput()
			code := 0
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tc.code {
				t.Fatalf("code=%d want=%d: %s", code, tc.code, out)
			}
			info, err := os.Stat(active)
			if err != nil {
				t.Fatal(err)
			}
			want, mode := "previous\n", os.FileMode(0600)
			if tc.code == 0 {
				want, mode = tc.source, 0640
			}
			if got := string(readTestFile(t, active)); got != want {
				t.Fatalf("active config=%q want=%q", got, want)
			}
			if info.Mode().Perm() != mode {
				t.Fatalf("active mode=%04o want=%04o", info.Mode().Perm(), mode)
			}
			if _, err := os.Stat(filepath.Join(dir, "active.conf.next")); !os.IsNotExist(err) {
				t.Fatalf("temporary config remains: %v", err)
			}
		})
	}
}

func TestPOSIXPracticalDeployArguments(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rsync", "ssh"} {
		body := "#!/bin/sh\nprintf '%s\\000' " + name + " \"$@\" >> \"$TRACE_FILE\"\n"
		writeTestFile(t, filepath.Join(bin, name), []byte(body), 0700)
	}
	trace := filepath.Join(dir, "trace")
	cmd := practicalCommand(t, dir, practicalExample(t, "deploy.yesh"))
	cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TRACE_FILE="+trace)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("deploy fixture: %v %s", err, out)
	}
	want := []string{"rsync", "-az", "build/current/", "web-1:/srv/app/", "ssh", "web-1", "systemctl", "restart", "api", "ssh", "web-1", "systemctl", "is-active", "api", ""}
	if got := strings.Split(string(readTestFile(t, trace)), "\x00"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected remote argv: %q", got)
	}
}

func TestPOSIXPracticalSSHHealthAndSCPArguments(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ssh", "scp"} {
		body := "#!/bin/sh\nprintf '%s\\000' " + name + " \"$@\" >> \"$TRACE_FILE\"\n"
		writeTestFile(t, filepath.Join(bin, name), []byte(body), 0700)
	}
	trace := filepath.Join(dir, "trace")
	env := []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "TRACE_FILE=" + trace}
	writeTestFile(t, filepath.Join(dir, "backup.tar.gz"), []byte("fixture\n"), 0600)

	health := practicalCommand(t, dir, practicalExample(t, "ssh-health.yesh"))
	health.Env = append(health.Env, env...)
	if out, err := health.CombinedOutput(); err != nil {
		t.Fatalf("ssh health fixture: %v %s", err, out)
	}

	scp := practicalCommand(t, dir, practicalExample(t, "scp-copy.yesh"))
	scp.Env = append(scp.Env, env...)
	if out, err := scp.CombinedOutput(); err != nil {
		t.Fatalf("scp fixture: %v %s", err, out)
	}

	want := []string{
		"ssh", "-p", "22", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "web-1", "hostname",
		"ssh", "-p", "22", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "web-1", "uptime",
		"ssh", "-p", "22", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "web-2", "hostname",
		"ssh", "-p", "22", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "web-2", "uptime",
		"scp", "-P", "22", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "--", "backup.tar.gz", "user@backup-1:/srv/incoming/backup.tar.gz",
		"",
	}
	if got := strings.Split(string(readTestFile(t, trace)), "\x00"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected SSH/SCP argv: %q", got)
	}

	missing := practicalCommand(t, dir, practicalExample(t, "scp-copy.yesh"))
	missing.Dir = filepath.Join(dir, "missing-source")
	if err := os.Mkdir(missing.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	missing.Env = append(missing.Env, env...)
	out, err := missing.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 || string(out) != "source file is missing\n" {
		t.Fatalf("missing source: %v %s", err, out)
	}
	if got := string(readTestFile(t, trace)); strings.Count(got, "scp\x00") != 1 {
		t.Fatalf("scp ran for missing source: %q", got)
	}
}

func TestPOSIXPracticalServiceFailureStopsScript(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(bin, "systemctl"), []byte(quickstartSystemctlFixture), 0700)
	script := filepath.Join(dir, "check.yesh")
	source := readTestFile(t, practicalExample(t, "service-check.yesh"))
	writeTestFile(t, script, append(source, []byte("\ntouch after\n")...), 0600)
	trace := filepath.Join(dir, "trace")
	cmd := practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TRACE_FILE="+trace, "CHECK_MODE=inactive")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("service failure: %v %s", err, out)
	}
	for _, want := range []string{"nginx is active", "sshd active check failed"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if got := string(readTestFile(t, trace)); got != "show --property=Version\nis-active --quiet -- nginx\nis-active --quiet -- sshd\n" {
		t.Fatalf("service calls: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "after")); !os.IsNotExist(err) {
		t.Fatalf("continued after failed service: %v", err)
	}
}

func TestPOSIXPracticalRsyncPreviewAndCopy(t *testing.T) {
	practicalDependencies(t, "rsync", "findmnt", "df")
	dir := t.TempDir()
	source, target := filepath.Join(dir, "source"), filepath.Join(dir, "target")
	for _, name := range []string{source, target} {
		if err := os.Mkdir(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(source, "file with spaces"), []byte("new payload\n"), 0600)
	writeTestFile(t, filepath.Join(target, "keep-extra"), []byte("do not delete\n"), 0600)
	out, err := practicalCommand(t, dir, practicalExample(t, "rsync-preview.yesh")).CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("file with spaces")) {
		t.Fatalf("preview: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(target, "file with spaces")); !os.IsNotExist(err) {
		t.Fatalf("preview copied data: %v", err)
	}
	if out, err := practicalCommand(t, dir, practicalExample(t, "rsync-copy.yesh")).CombinedOutput(); err != nil {
		t.Fatalf("copy: %v %s", err, out)
	}
	if got := string(readTestFile(t, filepath.Join(target, "file with spaces"))); got != "new payload\n" {
		t.Fatalf("copied data: %q", got)
	}
	if got := string(readTestFile(t, filepath.Join(target, "keep-extra"))); got != "do not delete\n" {
		t.Fatal("extra destination data deleted")
	}
	if got := string(readTestFile(t, filepath.Join(source, "file with spaces"))); got != "new payload\n" {
		t.Fatal("source changed")
	}
}

func TestPOSIXPracticalStorageCheck(t *testing.T) {
	practicalDependencies(t, "findmnt", "df", "du")
	for _, tc := range []struct {
		name, mountpoint string
		wantCode         int
		wantOutput       []string
	}{
		{"mounted directory", t.TempDir(), 0, []string{"Filesystem"}},
		{"missing directory", filepath.Join(t.TempDir(), "missing"), 2, []string{"backup mount is missing"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := readTestFile(t, practicalExample(t, "storage-check.yesh"))
			source = bytes.Replace(source, []byte("mountpoint = /backup"), []byte("mountpoint = "+tc.mountpoint), 1)
			script := filepath.Join(t.TempDir(), "storage-check.yesh")
			writeTestFile(t, script, source, 0600)
			cmd := practicalCommand(t, filepath.Dir(script), script)
			out, err := cmd.CombinedOutput()
			code := 0
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tc.wantCode {
				t.Fatalf("code=%d want=%d: %s", code, tc.wantCode, out)
			}
			for _, want := range tc.wantOutput {
				if !bytes.Contains(out, []byte(want)) {
					t.Fatalf("output %q does not contain %q", out, want)
				}
			}
		})
	}
}

func TestPOSIXPracticalStorageDeviceCheck(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "trace")
	for _, name := range []string{"findmnt", "lsblk", "blkid", "stat"} {
		body := "#!/bin/sh\nprintf '%s\\000' " + name + " \"$@\" >> \"$TRACE_FILE\"\n"
		if name == "findmnt" {
			body += "exit \"${FINDMNT_STATUS:-0}\"\n"
		}
		writeTestFile(t, filepath.Join(bin, name), []byte(body), 0700)
	}
	script := filepath.Join(dir, "device-check.yesh")
	source := readTestFile(t, practicalExample(t, "storage-device-check.yesh"))
	source = bytes.Replace(source, []byte("mountpoint = /backup"), []byte("mountpoint = "+dir), 1)
	writeTestFile(t, script, source, 0600)

	cmd := practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TRACE_FILE="+trace)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("device report: %v %s", err, out)
	}
	want := []string{
		"findmnt", "--target", dir,
		"findmnt", "--target", dir,
		"findmnt", "-no", "SOURCE,FSTYPE,OPTIONS", "--target", dir,
		"lsblk", "--fs", "--output", "NAME,TYPE,FSTYPE,LABEL,UUID,PARTUUID,MOUNTPOINTS",
		"blkid", "--output", "full",
		"stat", "-c", "type=%F mode=%a owner=%U:%G", "--", dir,
	}
	got := strings.Split(strings.TrimSuffix(string(readTestFile(t, trace)), "\x00"), "\x00")
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("device report argv: %q want %q", got, want)
	}

	writeTestFile(t, trace, nil, 0600)
	cmd = practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TRACE_FILE="+trace, "FINDMNT_STATUS=1")
	if out, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(out, []byte("path is not on a mounted filesystem")) {
		t.Fatalf("missing mount: %v %s", err, out)
	}
	traceData := readTestFile(t, trace)
	for _, name := range []string{"lsblk", "blkid", "stat"} {
		if bytes.Contains(traceData, []byte(name+"\x00")) {
			t.Fatalf("%s ran after failed mount: %q", name, traceData)
		}
	}
}

func TestPOSIXPracticalStorageIdentityCheck(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	findmnt := "#!/bin/sh\nprintf '%s\\000' findmnt \"$@\" >> \"$TRACE_FILE\"\nexit \"${FINDMNT_STATUS:-0}\"\n"
	writeTestFile(t, filepath.Join(bin, "findmnt"), []byte(findmnt), 0700)
	script := filepath.Join(dir, "identity-check.yesh")
	source := readTestFile(t, practicalExample(t, "storage-identity-check.yesh"))
	source = bytes.Replace(source, []byte("mountpoint = /backup"), []byte("mountpoint = "+dir), 1)
	source = bytes.Replace(source, []byte("UUID=replace-with-the-real-uuid"), []byte("UUID=11111111-2222-3333-4444-555555555555"), 1)
	writeTestFile(t, script, source, 0600)

	run := func(status string) (int, []byte) {
		writeTestFile(t, filepath.Join(dir, "trace"), nil, 0600)
		cmd := practicalCommand(t, dir, script)
		cmd.Env = append(cmd.Env,
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"TRACE_FILE="+filepath.Join(dir, "trace"),
			"FINDMNT_STATUS="+status,
		)
		out, err := cmd.CombinedOutput()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return code, out
	}

	if code, out := run("0"); code != 0 || !bytes.Contains(out, []byte("identity is correct")) {
		t.Fatalf("matching identity: code=%d output=%s", code, out)
	}
	want := []string{"findmnt", "--source", "UUID=11111111-2222-3333-4444-555555555555", "--target", dir, "findmnt", "--source", "UUID=11111111-2222-3333-4444-555555555555", "--target", dir}
	got := strings.Split(strings.TrimSuffix(string(readTestFile(t, filepath.Join(dir, "trace"))), "\x00"), "\x00")
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("identity argv: %q want %q", got, want)
	}
	if code, out := run("1"); code != 3 || !bytes.Contains(out, []byte("identity does not match")) {
		t.Fatalf("mismatching identity: code=%d output=%s", code, out)
	}
}

func TestPOSIXPracticalRsyncSafeCopyChecksIdentity(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "trace")
	for _, name := range []string{"findmnt", "df", "rsync"} {
		body := "#!/bin/sh\nprintf '%s\\000' " + name + " \"$@\" >> \"$TRACE_FILE\"\n"
		if name == "findmnt" {
			body += "exit \"${FINDMNT_STATUS:-0}\"\n"
		}
		writeTestFile(t, filepath.Join(bin, name), []byte(body), 0700)
	}
	if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "target"), 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "safe-copy.yesh")
	source := readTestFile(t, practicalExample(t, "rsync-safe-copy.yesh"))
	writeTestFile(t, script, source, 0600)

	run := func(status string) (int, []byte) {
		writeTestFile(t, trace, nil, 0600)
		cmd := practicalCommand(t, dir, script)
		cmd.Env = append(cmd.Env,
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"TRACE_FILE="+trace,
			"YESH_BACKUP_SOURCE=UUID=11111111-2222-3333-4444-555555555555",
			"FINDMNT_STATUS="+status,
		)
		out, err := cmd.CombinedOutput()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return code, out
	}

	if code, out := run("0"); code != 0 || len(out) != 0 {
		t.Fatalf("matching safe copy: code=%d output=%s", code, out)
	}
	traceData := readTestFile(t, trace)
	for _, name := range []string{"findmnt\x00", "df\x00", "rsync\x00"} {
		if !bytes.Contains(traceData, []byte(name)) {
			t.Fatalf("matching safe copy did not run %s: %q", name, traceData)
		}
	}
	if code, out := run("1"); code != 3 || !bytes.Contains(out, []byte("identity does not match")) {
		t.Fatalf("mismatching safe copy: code=%d output=%s", code, out)
	}
	if bytes.Contains(readTestFile(t, trace), []byte("rsync\x00")) {
		t.Fatal("rsync ran after identity mismatch")
	}
}

func TestPOSIXPracticalPVPreservesData(t *testing.T) {
	practicalDependencies(t, "pv", "cat")
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input"), filepath.Join(dir, "output")
	data := bytes.Repeat([]byte("binary\x00payload\n"), 10000)
	writeTestFile(t, input, data, 0600)
	script := filepath.Join(dir, "progress.yesh")
	writeTestFile(t, script, []byte("cat < input | pv --numeric --bytes > output\n"), 0600)
	cmd := practicalCommand(t, dir, script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("pv: %v %s", err, stderr.String())
	}
	if !bytes.Equal(readTestFile(t, output), data) {
		t.Fatal("progress data leaked into payload or bytes lost")
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("progress streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestPOSIXPracticalRsnapshotRotation(t *testing.T) {
	practicalDependencies(t, "rsnapshot", "rsync", "cp", "rm")
	dir := t.TempDir()
	source, root := filepath.Join(dir, "source"), filepath.Join(dir, "snapshots")
	for _, name := range []string{source, root} {
		if err := os.Mkdir(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	// rsnapshot's native format uses tabs and is not sourced as shell code.
	rsync, _ := exec.LookPath("rsync")
	cp, _ := exec.LookPath("cp")
	rm, _ := exec.LookPath("rm")
	config := "config_version\t1.2\nsnapshot_root\t" + root + "/\nno_create_root\t1\n" +
		"cmd_rsync\t" + rsync + "\ncmd_cp\t" + cp + "\ncmd_rm\t" + rm + "\n" +
		"retain\tdaily\t2\nverbose\t5\nloglevel\t2\nlockfile\t" + filepath.Join(dir, "rsnapshot.pid") + "\n" +
		// The /./ pivot limits --relative to the owned source subtree,
		// excluding unrelated /tmp ancestor ownership from the snapshot.
		"backup\t" + source + "/./\tlocal/\n"
	writeTestFile(t, filepath.Join(dir, "rsnapshot.conf"), []byte(config), 0600)
	payload := filepath.Join(source, "payload")
	writeTestFile(t, payload, []byte("version one\n"), 0600)
	writeTestFile(t, filepath.Join(source, "unchanged"), []byte("shared snapshot data\n"), 0600)
	example := practicalExample(t, "rsnapshot-local.yesh")
	if out, err := practicalCommand(t, dir, example).CombinedOutput(); err != nil {
		t.Fatalf("snapshot one: %v %s", err, out)
	}
	// Atomic replacement avoids modifying any existing hard-linked snapshot inode.
	writeTestFile(t, filepath.Join(source, "next"), []byte("version two, changed size\n"), 0600)
	if err := os.Rename(filepath.Join(source, "next"), payload); err != nil {
		t.Fatal(err)
	}
	if out, err := practicalCommand(t, dir, example).CombinedOutput(); err != nil {
		t.Fatalf("snapshot two: %v %s", err, out)
	}
	for _, tc := range []struct{ level, want string }{{"daily.0", "version two, changed size\n"}, {"daily.1", "version one\n"}} {
		got := string(readTestFile(t, filepath.Join(root, tc.level, "local", "payload")))
		if got != tc.want {
			t.Fatalf("%s restored=%q want=%q", tc.level, got, tc.want)
		}
	}
	first, err := os.Stat(filepath.Join(root, "daily.0", "local", "unchanged"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(filepath.Join(root, "daily.1", "local", "unchanged"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(first, second) {
		t.Fatal("unchanged snapshot data was not reused through a hard link")
	}
	if _, err := os.Stat(filepath.Join(dir, "rsnapshot.pid")); !os.IsNotExist(err) {
		t.Fatalf("snapshot lock remains: %v", err)
	}
}

func TestPOSIXPracticalCancelledJobDoesNotCommit(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "pid")
	script := filepath.Join(dir, "job.yesh")
	fixture := posixFixture(t, `printf 'unfinished\n'; echo $$ > "$1"; exec sleep 30`)
	source := fixture + " " + pidfile + " > work.partial\nmv -- work.partial ready\nprintf '%s' '{\"status\":\"complete\"}' > state.json\n"
	writeTestFile(t, script, []byte(source), 0600)
	writeTestFile(t, filepath.Join(dir, "state.json"), []byte("previous state"), 0600)
	cmd := practicalCommand(t, dir, script)
	var output bytes.Buffer
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	var pid int
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		data, _ := os.ReadFile(pidfile)
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err == nil && pid > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("job did not become ready")
	}
	defer syscall.Kill(-pid, syscall.SIGKILL)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("cancelled job succeeded")
	}
	if err := syscall.Kill(-pid, 0); err != syscall.ESRCH {
		t.Fatalf("cancelled job group survived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ready")); !os.IsNotExist(err) {
		t.Fatalf("partial result was published: %v", err)
	}
	if got := string(readTestFile(t, filepath.Join(dir, "state.json"))); got != "previous state" {
		t.Fatalf("cancellation published success: %q", got)
	}
	if got := string(readTestFile(t, filepath.Join(dir, "work.partial"))); got != "unfinished\n" {
		t.Fatalf("partial result lost: %q", got)
	}
}
