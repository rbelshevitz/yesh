//go:build !windows

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPOSIXRealSSHAndSCP is deliberately opt-in. It exercises the yesh
// runtime with a real host, while leaving authentication, agent selection,
// known_hosts, and private keys to OpenSSH. The remote directory must already
// exist; the test only creates a uniquely named file below it.
func TestPOSIXRealSSHAndSCP(t *testing.T) {
	if os.Getenv("YESH_SSH_INTEGRATION") != "1" {
		t.Skip("set YESH_SSH_INTEGRATION=1 to run against an explicit test host")
	}
	host := os.Getenv("YESH_SSH_TEST_HOST")
	remoteDir := os.Getenv("YESH_SSH_TEST_DIR")
	if host == "" || remoteDir == "" {
		t.Fatal("YESH_SSH_TEST_HOST and YESH_SSH_TEST_DIR are required")
	}
	if strings.ContainsAny(host, "\r\n") || strings.ContainsAny(remoteDir, "\r\n") {
		t.Fatal("SSH host and remote directory must not contain newlines")
	}
	if !filepath.IsAbs(remoteDir) {
		t.Fatal("YESH_SSH_TEST_DIR must be an absolute remote path")
	}

	localDir := t.TempDir()
	source := filepath.Join(localDir, "payload.txt")
	payload := []byte("yesh real SSH/SCP integration\n")
	writeTestFile(t, source, payload, 0600)
	remote := filepath.Join(remoteDir, fmt.Sprintf("yesh-integration-%d.txt", time.Now().UnixNano()))

	sshOptions := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes"}
	if key := os.Getenv("YESH_SSH_IDENTITY_FILE"); key != "" {
		sshOptions = append([]string{"-i", key, "-o", "IdentitiesOnly=yes"}, sshOptions...)
	}
	quoted := func(s string) string { return strconv.Quote(s) }
	list := func(items []string) string {
		parts := make([]string, len(items))
		for i, item := range items {
			parts[i] = quoted(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	sshArgs := list(sshOptions)
	scpOptions := append([]string{"-P", "22"}, sshOptions...)
	// The remote checksum is printed by ssh and compared by the Go test; no
	// shell is introduced on the remote side beyond the explicitly requested
	// sha256sum command and its two arguments.
	script := fmt.Sprintf("ssh_options = %s\nscp_options = %s\nsource = %s\nhost = %s\nremote = %s\nscp $scp_options -- $source $remote\nssh $ssh_options $host sha256sum -- $remote\n", sshArgs, list(scpOptions), quoted(source), quoted(host), quoted(host+":"+remote))
	// The final ssh destination is separate from the SCP destination.
	script = strings.Replace(script, "ssh $ssh_options $host sha256sum -- $remote", "ssh $ssh_options $host sha256sum -- "+quoted(remote), 1)
	scriptPath := filepath.Join(localDir, "integration.yesh")
	writeTestFile(t, scriptPath, []byte(script), 0600)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := posixCLI(t, ctx, scriptPath)
	cmd.Dir = localDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real SSH/SCP integration failed: %v\n%s", err, out)
	}
	checksum := regexp.MustCompile(`(?m)^([0-9a-f]{64})\s+`).FindSubmatch(out)
	if len(checksum) != 2 {
		t.Fatalf("remote sha256sum missing from output: %q", out)
	}
	local := exec.Command("sha256sum", source)
	localOut, err := local.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(checksum[1], bytes.Fields(localOut)[0]) {
		t.Fatalf("checksum mismatch: remote=%s local=%s", checksum[1], bytes.Fields(localOut)[0])
	}
	t.Logf("copied %s to %s:%s", source, host, remote)
}

// TestPOSIXRealRsyncPullAndIncremental is deliberately opt-in. It verifies
// the actual backup topology: yesh runs locally, rsync pulls from a remote
// source, and snapshots plus publication stay on the local filesystem.
func TestPOSIXRealRsyncPullAndIncremental(t *testing.T) {
	if os.Getenv("YESH_RSYNC_SSH_INTEGRATION") != "1" {
		t.Skip("set YESH_RSYNC_SSH_INTEGRATION=1 to run against an explicit test host")
	}
	host := os.Getenv("YESH_SSH_TEST_HOST")
	base := os.Getenv("YESH_SSH_TEST_DIR")
	if host == "" || base == "" {
		t.Fatal("YESH_SSH_TEST_HOST and YESH_SSH_TEST_DIR are required")
	}
	if strings.ContainsAny(host, "\r\n") || strings.ContainsAny(base, "\r\n") {
		t.Fatal("SSH host and remote directory must not contain newlines")
	}
	if !filepath.IsAbs(base) {
		t.Fatal("YESH_SSH_TEST_DIR must be an absolute remote path")
	}

	sshOptions := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes"}
	if key := os.Getenv("YESH_SSH_IDENTITY_FILE"); key != "" {
		sshOptions = append([]string{"-i", key, "-o", "IdentitiesOnly=yes"}, sshOptions...)
	}
	ssh := func(args ...string) ([]byte, error) {
		cmd := exec.Command("ssh", append(append([]string{}, sshOptions...), append([]string{host}, args...)...)...)
		return cmd.CombinedOutput()
	}
	if out, err := ssh("rsync", "--version"); err != nil || !strings.Contains(string(out), "rsync") {
		t.Fatalf("remote rsync unavailable: %v\n%s", err, out)
	}
	out, err := ssh("mktemp", "-d", "--", filepath.Join(base, "yesh-rsync-XXXXXX"))
	if err != nil {
		t.Fatalf("create remote fixture: %v\n%s", err, out)
	}
	remoteRoot := strings.TrimSpace(string(out))
	if !filepath.IsAbs(remoteRoot) || !strings.HasPrefix(remoteRoot, strings.TrimRight(base, "/")+"/") {
		t.Fatalf("unexpected remote fixture path %q", remoteRoot)
	}
	t.Cleanup(func() {
		if out, err := ssh("rm", "-rf", "--", remoteRoot); err != nil {
			t.Errorf("remove remote fixture: %v\n%s", err, out)
		}
	})
	if out, err := ssh("mkdir", "-m", "0700", "--", filepath.Join(remoteRoot, "data")); err != nil {
		t.Fatalf("create remote source: %v\n%s", err, out)
	}

	localDir := t.TempDir()
	localSource := filepath.Join(localDir, "source")
	if err := os.Mkdir(localSource, 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(localSource, "hello.txt"), []byte("remote version one\n"), 0600)
	writeTestFile(t, filepath.Join(localSource, "unchanged.txt"), []byte("remote unchanged\n"), 0600)
	copyRemote := func(name string) {
		args := append(append([]string{}, sshOptions...), filepath.Join(localSource, name), host+":"+filepath.Join(remoteRoot, "data", name))
		if out, err := exec.Command("scp", args...).CombinedOutput(); err != nil {
			t.Fatalf("copy remote source file %s: %v\n%s", name, err, out)
		}
	}
	copyRemote("hello.txt")
	copyRemote("unchanged.txt")

	root := filepath.Join(localDir, "snapshots")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	rsyncShell := "ssh -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=yes"
	if key := os.Getenv("YESH_SSH_IDENTITY_FILE"); key != "" {
		rsyncShell = "ssh -i " + key + " -o IdentitiesOnly=yes " + rsyncShell
	}
	run := func(previous, current, temporary string) {
		cmd := practicalCommand(t, localDir, practicalExample(t, "rsync-incremental.yesh"))
		cmd.Env = append(os.Environ(),
			"YESH_TEST_CLI=1",
			"RSYNC_RSH="+rsyncShell,
			"YESH_RSYNC_SOURCE="+host+":"+filepath.Join(remoteRoot, "data")+string(os.PathSeparator),
			"YESH_RSYNC_ROOT="+root,
			"YESH_RSYNC_PREVIOUS="+previous,
			"YESH_RSYNC_CURRENT="+current,
			"YESH_RSYNC_TEMPORARY="+temporary,
			"YESH_RSYNC_LOCK="+filepath.Join(root, ".rsync.lock"),
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("remote rsync run %s: %v\n%s", filepath.Base(current), err, out)
		}
	}
	first := filepath.Join(root, "snapshot.0")
	run(filepath.Join(root, "missing-previous"), first, filepath.Join(root, "snapshot.0.tmp"))
	if got := string(readTestFile(t, filepath.Join(first, "hello.txt"))); got != "remote version one\n" {
		t.Fatalf("first remote snapshot: %q", got)
	}
	writeTestFile(t, filepath.Join(localSource, "hello.txt"), []byte("remote version two\n"), 0600)
	writeTestFile(t, filepath.Join(localSource, "new.txt"), []byte("remote new\n"), 0600)
	copyRemote("hello.txt")
	copyRemote("new.txt")
	second := filepath.Join(root, "snapshot.1")
	run(first, second, filepath.Join(root, "snapshot.1.tmp"))
	if got := string(readTestFile(t, filepath.Join(second, "hello.txt"))); got != "remote version two\n" {
		t.Fatalf("changed remote file: %q", got)
	}
	if got := string(readTestFile(t, filepath.Join(second, "new.txt"))); got != "remote new\n" {
		t.Fatalf("new remote file: %q", got)
	}
	old, err := os.Stat(filepath.Join(first, "unchanged.txt"))
	if err != nil {
		t.Fatal(err)
	}
	next, err := os.Stat(filepath.Join(second, "unchanged.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(old, next) {
		t.Fatal("unchanged remote file was not reused through --link-dest")
	}
	if _, err := os.Stat(filepath.Join(root, "snapshot.1.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temporary snapshot remains: %v", err)
	}
}

// TestPOSIXRealRsyncRemoteFailureDoesNotPublish is deliberately opt-in.
// It checks that a real remote read error crosses the SSH/rsync boundary as a
// failed yesh operation and cannot publish a new snapshot.
func TestPOSIXRealRsyncRemoteFailureDoesNotPublish(t *testing.T) {
	if os.Getenv("YESH_RSYNC_SSH_INTEGRATION") != "1" {
		t.Skip("set YESH_RSYNC_SSH_INTEGRATION=1 to run against an explicit test host")
	}
	host := os.Getenv("YESH_SSH_TEST_HOST")
	base := os.Getenv("YESH_SSH_TEST_DIR")
	if host == "" || base == "" {
		t.Fatal("YESH_SSH_TEST_HOST and YESH_SSH_TEST_DIR are required")
	}
	if strings.ContainsAny(host, "\r\n") || strings.ContainsAny(base, "\r\n") {
		t.Fatal("SSH host and remote directory must not contain newlines")
	}
	if !filepath.IsAbs(base) {
		t.Fatal("YESH_SSH_TEST_DIR must be an absolute remote path")
	}

	sshOptions := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes"}
	if key := os.Getenv("YESH_SSH_IDENTITY_FILE"); key != "" {
		sshOptions = append([]string{"-i", key, "-o", "IdentitiesOnly=yes"}, sshOptions...)
	}
	ssh := func(args ...string) ([]byte, error) {
		cmd := exec.Command("ssh", append(append([]string{}, sshOptions...), append([]string{host}, args...)...)...)
		return cmd.CombinedOutput()
	}
	if out, err := ssh("rsync", "--version"); err != nil || !strings.Contains(string(out), "rsync") {
		t.Fatalf("remote rsync unavailable: %v\n%s", err, out)
	}
	out, err := ssh("mktemp", "-d", "--", filepath.Join(base, "yesh-rsync-failure-XXXXXX"))
	if err != nil {
		t.Fatalf("create remote fixture: %v\n%s", err, out)
	}
	remoteRoot := strings.TrimSpace(string(out))
	if !filepath.IsAbs(remoteRoot) || !strings.HasPrefix(remoteRoot, strings.TrimRight(base, "/")+"/") {
		t.Fatalf("unexpected remote fixture path %q", remoteRoot)
	}
	t.Cleanup(func() {
		if out, err := ssh("rm", "-rf", "--", remoteRoot); err != nil {
			t.Errorf("remove remote fixture: %v\n%s", err, out)
		}
	})
	remoteData := filepath.Join(remoteRoot, "data")
	if out, err := ssh("mkdir", "-m", "0700", "--", remoteData); err != nil {
		t.Fatalf("create remote source: %v\n%s", err, out)
	}

	localDir := t.TempDir()
	localSource := filepath.Join(localDir, "source")
	if err := os.Mkdir(localSource, 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(localSource, "good.txt"), []byte("good\n"), 0600)
	writeTestFile(t, filepath.Join(localSource, "blocked.txt"), []byte("blocked\n"), 0600)
	copyRemote := func(name string) {
		args := append(append([]string{}, sshOptions...), filepath.Join(localSource, name), host+":"+filepath.Join(remoteData, name))
		if out, err := exec.Command("scp", args...).CombinedOutput(); err != nil {
			t.Fatalf("copy remote source file %s: %v\n%s", name, err, out)
		}
	}
	copyRemote("good.txt")
	copyRemote("blocked.txt")
	if out, err := ssh("chmod", "000", "--", filepath.Join(remoteData, "blocked.txt")); err != nil {
		t.Fatalf("make remote source unreadable: %v\n%s", err, out)
	}

	root := filepath.Join(localDir, "snapshots")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	previous := filepath.Join(root, "snapshot.0")
	if err := os.Mkdir(previous, 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(previous, "old.txt"), []byte("must remain\n"), 0600)
	rsyncShell := "ssh -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=yes"
	if key := os.Getenv("YESH_SSH_IDENTITY_FILE"); key != "" {
		rsyncShell = "ssh -i " + key + " -o IdentitiesOnly=yes " + rsyncShell
	}
	run := func(current, temporary string) ([]byte, error) {
		cmd := practicalCommand(t, localDir, practicalExample(t, "rsync-incremental.yesh"))
		cmd.Env = append(os.Environ(),
			"YESH_TEST_CLI=1",
			"RSYNC_RSH="+rsyncShell,
			"YESH_RSYNC_SOURCE="+host+":"+remoteData+string(os.PathSeparator),
			"YESH_RSYNC_ROOT="+root,
			"YESH_RSYNC_PREVIOUS="+previous,
			"YESH_RSYNC_CURRENT="+current,
			"YESH_RSYNC_TEMPORARY="+temporary,
			"YESH_RSYNC_LOCK="+filepath.Join(root, ".rsync.lock"),
		)
		return cmd.CombinedOutput()
	}
	failed := filepath.Join(root, "snapshot.1")
	partial := filepath.Join(root, "snapshot.1.tmp")
	out, err = run(failed, partial)
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() == 0 {
		t.Fatalf("remote read failure exit=%v output=%q", err, out)
	}
	if _, err := os.Stat(failed); !os.IsNotExist(err) {
		t.Fatalf("remote read failure published snapshot: %v", err)
	}
	if got := string(readTestFile(t, filepath.Join(previous, "old.txt"))); got != "must remain\n" {
		t.Fatalf("remote read failure changed previous snapshot: %q", got)
	}
	if _, err := os.Stat(partial); err != nil {
		t.Fatalf("expected failed transfer staging for diagnosis: %v", err)
	}

	if out, err := ssh("chmod", "0600", "--", filepath.Join(remoteData, "blocked.txt")); err != nil {
		t.Fatalf("restore remote source permissions: %v\n%s", err, out)
	}
	retry := filepath.Join(root, "snapshot.2")
	if out, err := run(retry, filepath.Join(root, "snapshot.2.tmp")); err != nil {
		t.Fatalf("retry after remote read failure: %v\n%s", err, out)
	}
	if got := string(readTestFile(t, filepath.Join(retry, "blocked.txt"))); got != "blocked\n" {
		t.Fatalf("retry content: %q", got)
	}

	if out, err := ssh("rm", "-rf", "--", remoteData); err != nil {
		t.Fatalf("remove remote source for missing-source case: %v\n%s", err, out)
	}
	missing := filepath.Join(root, "snapshot.3")
	out, err = run(missing, filepath.Join(root, "snapshot.3.tmp"))
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() == 0 {
		t.Fatalf("missing remote source exit=%v output=%q", err, out)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing remote source published snapshot: %v", err)
	}
	if got := string(readTestFile(t, filepath.Join(retry, "blocked.txt"))); got != "blocked\n" {
		t.Fatalf("missing remote source changed previous snapshot: %q", got)
	}
}
