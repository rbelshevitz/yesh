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

// The first licensed-corpus adaptations deliberately keep their safety
// properties visible in the source: preview before copy, an explicit mount
// check, and non-interactive SSH host verification. This test guards those
// reviewable contracts without requiring a live remote host or a real backup
// device.
func TestPOSIXCorpusAdaptationSafetyContracts(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		must   []string
		forbid []string
	}{
		{name: "rsync preview", file: "rsync-preview.yesh", must: []string{"rsync -a --dry-run", "--itemize-changes", "-- $source $target"}},
		{name: "incremental rsync", file: "rsync-incremental.yesh", must: []string{"with lock $lock:", "--delay-updates", "--link-dest $previous"}, forbid: []string{"--delete", "--delete-excluded", "--delete-missing-args", "--remove-source-files", "--ignore-errors"}},
		{name: "rsnapshot preflight", file: "rsnapshot-preflight.yesh", must: []string{"rsnapshot -c $config configtest", "rsnapshot -c $config -t $level"}},
		{name: "rsnapshot guarded apply", file: "rsnapshot-apply.yesh", must: []string{"findmnt --source $mount_source --target $root", "rsnapshot -c $config -t $level", "YESH_RSNAPSHOT_CONFIRM"}},
		{name: "mount preflight", file: "storage-check.yesh", must: []string{"findmnt --target $mountpoint", "df -P -h -- $mountpoint"}},
		{name: "ssh health", file: "ssh-health.yesh", must: []string{"BatchMode=yes", "StrictHostKeyChecking=yes", "ConnectTimeout=5"}, forbid: []string{"StrictHostKeyChecking=no", "UserKnownHostsFile=/dev/null"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := string(readTestFile(t, practicalExample(t, tc.file)))
			for _, want := range tc.must {
				if !strings.Contains(data, want) {
					t.Fatalf("%s missing safety contract %q", tc.file, want)
				}
			}
			for _, banned := range tc.forbid {
				if strings.Contains(data, banned) {
					t.Fatalf("%s contains unsafe contract %q", tc.file, banned)
				}
			}
		})
	}
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

func TestPOSIXPracticalSSHHealthFailureStopsLoop(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "trace")
	ssh := "#!/bin/sh\nprintf '%s\\000' ssh \"$@\" >> \"$TRACE_FILE\"\nexit 255\n"
	writeTestFile(t, filepath.Join(bin, "ssh"), []byte(ssh), 0700)
	script := practicalExample(t, "ssh-health.yesh")
	cmd := practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TRACE_FILE="+trace,
	)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 || !bytes.Contains(out, []byte("web-1 health check failed")) {
		t.Fatalf("SSH failure: code=%v output=%q", err, out)
	}
	want := []string{
		"ssh", "-p", "22", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "web-1", "hostname",
	}
	got := strings.Split(strings.TrimSuffix(string(readTestFile(t, trace)), "\x00"), "\x00")
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("SSH failure argv: %q want %q", got, want)
	}
	if bytes.Contains(readTestFile(t, trace), []byte("web-2")) {
		t.Fatal("health check continued to the next host after failure")
	}
}

func TestPOSIXPracticalSCPFailureDoesNotPublish(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "trace")
	scp := "#!/bin/sh\nprintf '%s\\000' scp \"$@\" >> \"$TRACE_FILE\"\nexit 23\n"
	writeTestFile(t, filepath.Join(bin, "scp"), []byte(scp), 0700)
	writeTestFile(t, filepath.Join(dir, "backup.tar.gz"), []byte("fixture\n"), 0600)
	script := filepath.Join(dir, "copy.yesh")
	source := readTestFile(t, practicalExample(t, "scp-copy.yesh"))
	writeTestFile(t, script, append(source, []byte("\ntouch published\n")...), 0600)
	cmd := practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TRACE_FILE="+trace,
	)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("SCP failure: code=%v output=%q", err, out)
	}
	want := []string{
		"scp", "-P", "22", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "--", "backup.tar.gz", "user@backup-1:/srv/incoming/backup.tar.gz",
	}
	got := strings.Split(strings.TrimSuffix(string(readTestFile(t, trace)), "\x00"), "\x00")
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("SCP failure argv: %q want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "published")); !os.IsNotExist(err) {
		t.Fatalf("script continued after failed SCP: %v", err)
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

func TestPOSIXPracticalIncrementalRsyncRoundTrip(t *testing.T) {
	practicalDependencies(t, "rsync")
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	root := filepath.Join(dir, "snapshots")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(source, "changed.txt"), []byte("version one\n"), 0600)
	writeTestFile(t, filepath.Join(source, "unchanged.txt"), []byte("shared payload\n"), 0600)

	run := func(previous, current, temporary string) {
		env := append(os.Environ(),
			"YESH_TEST_CLI=1",
			"YESH_RSYNC_SOURCE="+source+string(os.PathSeparator),
			"YESH_RSYNC_ROOT="+root,
			"YESH_RSYNC_PREVIOUS="+previous,
			"YESH_RSYNC_CURRENT="+current,
			"YESH_RSYNC_TEMPORARY="+temporary,
			"YESH_RSYNC_LOCK="+filepath.Join(root, ".rsync.lock"),
		)
		cmd := practicalCommand(t, dir, practicalExample(t, "rsync-incremental.yesh"))
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("incremental rsync %s: %v\n%s", filepath.Base(current), err, out)
		}
		if _, err := os.Stat(current); err != nil {
			t.Fatalf("published snapshot %s: %v\n%s", current, err, out)
		}
		if _, err := os.Stat(temporary); !os.IsNotExist(err) {
			t.Fatalf("temporary snapshot remains: %v", err)
		}
	}

	first := filepath.Join(root, "snapshot.0")
	run(filepath.Join(root, "missing-previous"), first, filepath.Join(root, "snapshot.0.tmp"))
	if got := string(readTestFile(t, filepath.Join(first, "changed.txt"))); got != "version one\n" {
		t.Fatalf("first snapshot content: %q", got)
	}

	// Replace the source atomically so rsync never observes a partially written file.
	writeTestFile(t, filepath.Join(source, "changed.next"), []byte("version two, changed size\n"), 0600)
	if err := os.Rename(filepath.Join(source, "changed.next"), filepath.Join(source, "changed.txt")); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, "snapshot.1")
	run(first, second, filepath.Join(root, "snapshot.1.tmp"))
	if got := string(readTestFile(t, filepath.Join(second, "changed.txt"))); got != "version two, changed size\n" {
		t.Fatalf("second snapshot content: %q", got)
	}
	if got := string(readTestFile(t, filepath.Join(second, "unchanged.txt"))); got != "shared payload\n" {
		t.Fatalf("second snapshot unchanged content: %q", got)
	}
	one, err := os.Stat(filepath.Join(first, "unchanged.txt"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := os.Stat(filepath.Join(second, "unchanged.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(one, two) {
		t.Fatal("unchanged file was not reused through --link-dest")
	}
	// A rerun cannot merge into an already published destination.
	rerun := practicalCommand(t, dir, practicalExample(t, "rsync-incremental.yesh"))
	rerun.Env = append(os.Environ(),
		"YESH_TEST_CLI=1",
		"YESH_RSYNC_SOURCE="+source+string(os.PathSeparator),
		"YESH_RSYNC_ROOT="+root,
		"YESH_RSYNC_PREVIOUS="+first,
		"YESH_RSYNC_CURRENT="+second,
		"YESH_RSYNC_TEMPORARY="+filepath.Join(root, "snapshot.1.tmp"),
		"YESH_RSYNC_LOCK="+filepath.Join(root, ".rsync.lock"),
	)
	if out, err := rerun.CombinedOutput(); err == nil {
		t.Fatalf("rerun unexpectedly succeeded: %s", out)
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 3 {
		t.Fatalf("rerun exit=%v output=%q", err, out)
	}
	if got := string(readTestFile(t, filepath.Join(second, "changed.txt"))); got != "version two, changed size\n" {
		t.Fatalf("rerun changed published data: %q", got)
	}
	// A missing source must fail before taking the old published snapshot away.
	missing := practicalCommand(t, dir, practicalExample(t, "rsync-incremental.yesh"))
	missing.Env = append(os.Environ(),
		"YESH_TEST_CLI=1",
		"YESH_RSYNC_SOURCE="+filepath.Join(dir, "missing-source"),
		"YESH_RSYNC_ROOT="+root,
		"YESH_RSYNC_PREVIOUS="+second,
		"YESH_RSYNC_CURRENT="+filepath.Join(root, "snapshot.2"),
		"YESH_RSYNC_TEMPORARY="+filepath.Join(root, "snapshot.2.tmp"),
		"YESH_RSYNC_LOCK="+filepath.Join(root, ".rsync.lock"),
	)
	if out, err := missing.CombinedOutput(); err == nil {
		t.Fatalf("missing source unexpectedly succeeded: %s", out)
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
		t.Fatalf("missing source exit=%v output=%q", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "snapshot.2")); !os.IsNotExist(err) {
		t.Fatalf("failed run published a snapshot: %v", err)
	}

	// Restore is intentionally performed by the external rsync utility; yesh
	// owns the selection and publication boundary, not a second copy engine.
	restore := filepath.Join(dir, "restore")
	if err := os.Mkdir(restore, 0700); err != nil {
		t.Fatal(err)
	}
	copy := exec.Command("rsync", "-a", second+string(os.PathSeparator), restore+string(os.PathSeparator))
	if out, err := copy.CombinedOutput(); err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	for name, want := range map[string]string{
		"changed.txt":   "version two, changed size\n",
		"unchanged.txt": "shared payload\n",
	} {
		if got := string(readTestFile(t, filepath.Join(restore, name))); got != want {
			t.Fatalf("restored %s: %q", name, got)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".rsync.lock")); err != nil {
		t.Fatalf("rsync lock path disappeared: %v", err)
	}
}

func TestPOSIXPracticalRsyncPreviewIntegration(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "trace")
	// Keep the fixtures intentionally tiny: the test is about yesh's process
	// orchestration and argv preservation, not rsync/findmnt implementation.
	writeTestFile(t, filepath.Join(bin, "findmnt"), []byte("#!/bin/sh\nprintf '%s\\000' findmnt \"$@\" >> \"$TRACE_FILE\"\nexit \"${FINDMNT_STATUS:-0}\"\n"), 0700)
	writeTestFile(t, filepath.Join(bin, "df"), []byte("#!/bin/sh\nprintf '%s\\000' df \"$@\" >> \"$TRACE_FILE\"\nprintf '%s\\n' 'Filesystem 1024-blocks Used Available Capacity Mounted on'\n"), 0700)
	writeTestFile(t, filepath.Join(bin, "rsync"), []byte("#!/bin/sh\nprintf '%s\\000' rsync \"$@\" >> \"$TRACE_FILE\"\nexit \"${RSYNC_STATUS:-0}\"\n"), 0700)

	if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "target"), 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "preview.yesh")
	writeTestFile(t, script, readTestFile(t, practicalExample(t, "rsync-preview.yesh")), 0600)

	run := func(findmntStatus string) (int, []byte, string) {
		writeTestFile(t, trace, nil, 0600)
		cmd := practicalCommand(t, dir, script)
		cmd.Env = append(cmd.Env,
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"TRACE_FILE="+trace,
			"FINDMNT_STATUS="+findmntStatus,
		)
		out, err := cmd.CombinedOutput()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return code, out, string(readTestFile(t, trace))
	}

	if code, out, got := run("0"); code != 0 {
		t.Fatalf("preview success: code=%d output=%q trace=%q", code, out, got)
	} else {
		want := []string{
			"findmnt", "--target", "target/",
			"findmnt", "--target", "target/",
			"df", "-P", "-h", "--", "target/",
			"rsync", "-a", "--dry-run", "--itemize-changes", "--", "source/", "target/",
		}
		gotArgs := strings.Split(strings.TrimSuffix(got, "\x00"), "\x00")
		if strings.Join(gotArgs, "|") != strings.Join(want, "|") {
			t.Fatalf("preview argv: %q want %q", gotArgs, want)
		}
	}

	if code, out, got := run("1"); code != 3 || !bytes.Contains(out, []byte("target is not on a mounted filesystem")) {
		t.Fatalf("unmounted target: code=%d output=%q trace=%q", code, out, got)
	} else if strings.Contains(got, "rsync\x00") || strings.Contains(got, "df\x00") {
		t.Fatalf("preflight failure ran later commands: %q", got)
	}

	copyScript := filepath.Join(dir, "copy.yesh")
	copySource := readTestFile(t, practicalExample(t, "rsync-copy.yesh"))
	writeTestFile(t, copyScript, append(copySource, []byte("\ntouch published\n")...), 0600)
	writeTestFile(t, trace, nil, 0600)
	cmd := practicalCommand(t, dir, copyScript)
	cmd.Env = append(cmd.Env,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TRACE_FILE="+trace,
		"RSYNC_STATUS=23",
	)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("rsync failure: code=%v output=%q", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "published")); !os.IsNotExist(err) {
		t.Fatalf("script continued after failed rsync: %v", err)
	}

	if err := os.RemoveAll(filepath.Join(dir, "target")); err != nil {
		t.Fatal(err)
	}
	if code, out, got := run("0"); code != 2 || !bytes.Contains(out, []byte("target directory is missing")) {
		t.Fatalf("missing target: code=%d output=%q trace=%q", code, out, got)
	} else if got != "" {
		t.Fatalf("missing target invoked external commands: %q", got)
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

func TestPOSIXPracticalRsnapshotPreflightOrder(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "rsnapshot.trace")
	writeTestFile(t, filepath.Join(bin, "rsnapshot"), []byte("#!/bin/sh\nprintf '%s\\000' rsnapshot \"$@\" >> \"$TRACE_FILE\"\n"), 0700)
	config := filepath.Join(dir, "rsnapshot.conf")
	writeTestFile(t, config, []byte("config_version 1.2\n"), 0600)
	cmd := practicalCommand(t, dir, practicalExample(t, "rsnapshot-preflight.yesh"))
	cmd.Env = append(os.Environ(),
		"YESH_TEST_CLI=1",
		"YESH_RSNAPSHOT_CONFIG="+config,
		"YESH_RSNAPSHOT_LEVEL=daily",
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TRACE_FILE="+trace,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("rsnapshot preflight: %v\n%s", err, out)
	}
	got := strings.Split(strings.TrimSuffix(string(readTestFile(t, trace)), "\x00"), "\x00")
	want := []string{"rsnapshot", "-c", config, "configtest", "rsnapshot", "-c", config, "-t", "daily"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("rsnapshot preflight argv: %q want %q", got, want)
	}
}

func TestPOSIXPracticalRsnapshotGuardedApply(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "snapshots")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "rsnapshot.conf")
	confirm := filepath.Join(dir, "approved")
	trace := filepath.Join(dir, "trace")
	writeTestFile(t, config, []byte("config_version 1.2\n"), 0600)
	writeTestFile(t, filepath.Join(bin, "findmnt"), []byte("#!/bin/sh\nexit 0\n"), 0700)
	writeTestFile(t, filepath.Join(bin, "rsnapshot"), []byte("#!/bin/sh\nprintf '%s\\000' rsnapshot \"$@\" >> \"$TRACE_FILE\"\n"), 0700)
	run := func() (int, string) {
		cmd := practicalCommand(t, dir, practicalExample(t, "rsnapshot-apply.yesh"))
		cmd.Env = append(os.Environ(),
			"YESH_TEST_CLI=1",
			"YESH_RSNAPSHOT_CONFIG="+config,
			"YESH_RSNAPSHOT_LEVEL=daily",
			"YESH_RSNAPSHOT_ROOT="+root,
			"YESH_RSNAPSHOT_SOURCE=UUID=test-fixture",
			"YESH_RSNAPSHOT_CONFIRM="+confirm,
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"TRACE_FILE="+trace,
		)
		out, err := cmd.CombinedOutput()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return code, string(out)
	}
	if code, out := run(); code != 4 || !strings.Contains(out, "confirmation") {
		t.Fatalf("without confirmation: code=%d output=%q", code, out)
	}
	got := strings.TrimSuffix(string(readTestFile(t, trace)), "\x00")
	args := strings.Split(got, "\x00")
	calls := 0
	for _, arg := range args {
		if arg == "rsnapshot" {
			calls++
		}
	}
	if calls != 2 {
		t.Fatalf("interval ran without confirmation: calls=%d trace=%q", calls, args)
	}
	writeTestFile(t, confirm, nil, 0600)
	if code, out := run(); code != 0 {
		t.Fatalf("with confirmation: code=%d output=%q", code, out)
	}
	got = strings.TrimSuffix(string(readTestFile(t, trace)), "\x00")
	args = strings.Split(got, "\x00")
	calls = 0
	for _, arg := range args {
		if arg == "rsnapshot" {
			calls++
		}
	}
	want := []string{"rsnapshot", "-c", config, "daily"}
	if calls != 5 || len(args) < len(want) || strings.Join(args[len(args)-len(want):], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("confirmed interval missing from trace: %q", args)
	}
}

func TestPOSIXPracticalRsnapshotConfigtestFailureStopsRotation(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "rsnapshot.trace")
	writeTestFile(t, filepath.Join(bin, "rsnapshot"), []byte("#!/bin/sh\nprintf '%s\\000' rsnapshot \"$@\" >> \"$TRACE_FILE\"\nlast=\"\"\nfor last do :; done\nif [ \"$last\" = configtest ]; then exit 2; fi\nexit 0\n"), 0700)
	script := filepath.Join(dir, "backup.yesh")
	source := string(readTestFile(t, practicalExample(t, "rsnapshot-local.yesh"))) + "\ntouch published\n"
	writeTestFile(t, script, []byte(source), 0600)
	cmd := practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TRACE_FILE="+trace)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("configtest failure unexpectedly succeeded")
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		code := -1
		if ok {
			code = exit.ExitCode()
		}
		t.Fatalf("configtest failure exit code=%d output=%q", code, out)
	}
	got := strings.Split(strings.TrimSuffix(string(readTestFile(t, trace)), "\x00"), "\x00")
	want := []string{"rsnapshot", "-c", "rsnapshot.conf", "configtest"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("rsnapshot continued after failed configtest: got %q want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "published")); !os.IsNotExist(err) {
		t.Fatalf("rotation marker exists after failed configtest: %v", err)
	}
}

func TestPOSIXPracticalRsnapshotLockConflictStopsPublication(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "rsnapshot.trace")
	lock := filepath.Join(dir, "rsnapshot.pid")
	writeTestFile(t, lock, []byte("held by another backup\n"), 0600)
	writeTestFile(t, filepath.Join(dir, "rsnapshot.conf"), []byte("lockfile\t"+lock+"\n"), 0600)
	writeTestFile(t, filepath.Join(bin, "rsnapshot"), []byte("#!/bin/sh\nprintf '%s\\000' rsnapshot \"$@\" >> \"$TRACE_FILE\"\nlast=\"\"\nfor last do :; done\nif [ \"$last\" = daily ] && [ -e \"$LOCK_FILE\" ]; then exit 75; fi\nexit 0\n"), 0700)
	script := filepath.Join(dir, "backup.yesh")
	source := string(readTestFile(t, practicalExample(t, "rsnapshot-local.yesh"))) + "\ntouch published\n"
	writeTestFile(t, script, []byte(source), 0600)
	cmd := practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TRACE_FILE="+trace,
		"LOCK_FILE="+lock,
	)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		code := -1
		if ok {
			code = exit.ExitCode()
		}
		t.Fatalf("lock conflict exit code=%d output=%q", code, out)
	}
	got := strings.Split(strings.TrimSuffix(string(readTestFile(t, trace)), "\x00"), "\x00")
	want := []string{"rsnapshot", "-c", "rsnapshot.conf", "configtest", "rsnapshot", "-c", "rsnapshot.conf", "daily"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lock conflict argv: got %q want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "published")); !os.IsNotExist(err) {
		t.Fatalf("rotation marker exists after lock conflict: %v", err)
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
