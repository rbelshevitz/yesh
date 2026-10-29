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
