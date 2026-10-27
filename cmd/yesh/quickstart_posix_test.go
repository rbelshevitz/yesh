//go:build !windows

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const quickstartSystemctlFixture = `#!/bin/sh
printf '%s\n' "$*" >> "$TRACE_FILE"
if [ "$1" = show ]; then
    if [ "$CHECK_MODE" = manager-down ]; then
        printf 'systemd bus unavailable\n' >&2
        exit 1
    fi
    printf 'Version=fixture\n'
    exit 0
fi
case "$CHECK_MODE" in
    all-active) exit 0 ;;
    inactive) case "$4" in nginx) exit 0 ;; *) exit 3 ;; esac ;;
    bus-lost) printf 'systemd bus connection lost\n' >&2; exit 1 ;;
    signal) kill -TERM $$ ;;
esac
exit 99
`

func TestPOSIXQuickstartServiceOutcomes(t *testing.T) {
	source := quickstartBlocks(t, "quickstart.md")[0]
	for _, tc := range []struct {
		mode                      string
		code                      int
		output, diagnostic, trace string
	}{
		{"all-active", 0, "nginx is active\nsshd is active\n", "", "show --property=Version\nis-active --quiet -- nginx\nis-active --quiet -- sshd\n"},
		{"inactive", 1, "nginx is active\nsshd active check failed\n", "", "show --property=Version\nis-active --quiet -- nginx\nis-active --quiet -- sshd\n"},
		{"manager-down", 1, "", "systemd bus unavailable", "show --property=Version\n"},
		{"bus-lost", 1, "nginx active check failed\n", "systemd bus connection lost", "show --property=Version\nis-active --quiet -- nginx\n"},
		{"signal", 1, "", "signal:", "show --property=Version\nis-active --quiet -- nginx\n"},
		{"missing-command", 1, "", "resolve systemctl", ""},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			if tc.mode != "missing-command" {
				writeTestFile(t, filepath.Join(bin, "systemctl"), []byte(quickstartSystemctlFixture), 0700)
			}
			trace, script := filepath.Join(dir, "trace"), filepath.Join(dir, "check.yesh")
			writeTestFile(t, script, []byte(source+"\ntouch after\n"), 0600)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := posixCLI(t, ctx, script)
			cmd.Dir = dir
			path := bin
			if tc.mode != "missing-command" {
				path += string(os.PathListSeparator) + os.Getenv("PATH")
			}
			cmd.Env = append(cmd.Env, "PATH="+path, "TRACE_FILE="+trace, "CHECK_MODE="+tc.mode)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			code := 0
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tc.code || stdout.String() != tc.output || !strings.Contains(stderr.String(), tc.diagnostic) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if data, err := os.ReadFile(trace); tc.trace == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("missing command ran: %q %v", data, err)
				}
			} else if err != nil || string(data) != tc.trace {
				t.Fatalf("command order=%q err=%v", data, err)
			}
			_, err = os.Stat(filepath.Join(dir, "after"))
			if (err == nil) != (tc.code == 0) {
				t.Fatalf("unexpected continuation: %v", err)
			}
		})
	}
}

func TestPOSIXQuickstartValuesAndExists(t *testing.T) {
	blocks := quickstartBlocks(t, "quickstart.md")
	dir := t.TempDir()
	present := filepath.Join(dir, "present")
	writeTestFile(t, present, []byte("data"), 0600)
	for _, tc := range []struct {
		name, source, output string
		code                 int
	}{
		{"list", blocks[1], "nginx\nsshd\n", 0},
		{"present", strings.ReplaceAll(blocks[3], "/etc/my-service.conf", present), "configuration exists\n", 0},
		{"absent", strings.ReplaceAll(blocks[3], "/etc/my-service.conf", filepath.Join(dir, "absent")), "configuration is missing\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := filepath.Join(dir, tc.name+".yesh")
			writeTestFile(t, script, []byte(tc.source+"\n"), 0600)
			out, err := practicalCommand(t, dir, script).CombinedOutput()
			code := 0
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tc.code || string(out) != tc.output {
				t.Fatalf("exit=%d output=%q", code, out)
			}
		})
	}
}

func TestPOSIXQuickstartPipelineDrainsInput(t *testing.T) {
	practicalDependencies(t, "sort")
	source := quickstartBlocks(t, "quickstart.md")[2]
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	// Large process-list fixture exceeds a pipe buffer. Every row must be consumed.
	producer := "#!/bin/sh\ni=1\nwhile [ \"$i\" -le 10000 ]; do printf '1 0 user %s S process\\n' \"$i\"; i=$((i+1)); done\n"
	writeTestFile(t, filepath.Join(bin, "ps"), []byte(producer), 0700)
	script := filepath.Join(dir, "pipeline.yesh")
	writeTestFile(t, script, []byte(source+"\n"), 0600)
	cmd := practicalCommand(t, dir, script)
	cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pipeline: %v %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 10000 {
		t.Fatalf("lost process rows: %d", len(lines))
	}
	for i, line := range lines {
		if want := fmt.Sprintf("1 0 user %d S process", 10000-i); line != want {
			t.Fatalf("row %d=%q want=%q", i, line, want)
		}
	}
}

func TestPOSIXQuickstartPlanDoesNotQuerySystemd(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "check.yesh")
	writeTestFile(t, script, []byte(quickstartBlocks(t, "quickstart.md")[0]+"\n"), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := posixCLI(t, ctx, "--plan", script)
	cmd.Env = append(cmd.Env, "PATH="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plan needs external command: %v %s", err, out)
	}
	for _, want := range []string{`PLAN ["systemctl" "show" "--property=Version"] > "/dev/null"`, `PLAN IF ["systemctl" "is-active" "--quiet" "--" "nginx"]`, `PLAN ELSE`} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("missing %q in plan: %s", want, out)
		}
	}
}
