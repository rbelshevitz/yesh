//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLinuxFstabLibmount(t *testing.T) {
	if _, err := exec.LookPath("findmnt"); err != nil {
		t.Skip("findmnt missing: util-linux package required")
	}
	temp := t.TempDir()
	snapshotDir := t.TempDir()
	t.Setenv("TMPDIR", snapshotDir)
	input := filepath.Join(temp, "fstab")
	cases := []struct {
		name, text string
		count      int
		wantErr    bool
	}{
		{"empty", "# comment only\n", 0, false},
		{"escapes and optional fields", `LABEL="backup\040disk" /backup\040disk ext4 noauto,nofail,x-systemd.automount 0 2
PARTUUID=part-id /mnt/part xfs
/swap1 none swap defaults 0 0
/swap2 none swap defaults 0 0
/data /bind none bind 0 0
server:/data /net nfs _netdev,x-systemd.requires=one,x-systemd.requires=two 0 0
tmpfs /tmp tmpfs mode=1777 0 0
UUID=tree /tree btrfs subvol=/snapshots,ro 0 0
UUID=escaped /tab\011back\134slash ext4 context="a,b",opaque=preserved 0 2
`, 9, false},
		{"mixed partial", "broken\nUUID=x /backup ext4 defaults 0 2\n", 0, true},
		{"bad number", "UUID=x /backup ext4 defaults nope 2\n", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeTestFile(t, input, []byte(tc.text), 0600)
			r := &runtime{values: map[string]value{}, filename: "fstab.yesh"}
			example, readErr := os.ReadFile(filepath.Join("..", "..", "examples", "fstab-inspect.yesh"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			source := strings.Replace(string(example), `"/etc/fstab"`, strconvQuote(input), 1)
			items, err := parse(source, "fstab.yesh")
			if err != nil {
				t.Fatal(err)
			}
			err = r.execute(items)
			if (err != nil) != tc.wantErr {
				t.Fatalf("execute %v", err)
			}
			if !tc.wantErr {
				s := r.values["mounts"].fstab
				if len(s.Entries) != tc.count {
					t.Fatalf("entries %d", len(s.Entries))
				}
				if tc.count > 0 {
					if s.Entries[0].Target != "/backup disk" || s.Entries[1].PassNo != 0 || len(s.Diagnostics) != 0 {
						t.Fatalf("snapshot %#v", s)
					}
					if s.Entries[8].Target != "/tab\tback\\slash" || s.Entries[8].Options != `context="a,b",opaque=preserved` {
						t.Fatalf("escapes/options changed: %#v", s.Entries[8])
					}
					if !strings.Contains(s.Entries[5].Options, "x-systemd.requires=one,x-systemd.requires=two") {
						t.Fatal("option order lost")
					}
				}
			} else if _, ok := r.values["mounts"]; ok {
				t.Fatal("partial table assigned")
			}
			after, _ := os.ReadFile(input)
			info, _ := os.Stat(input)
			if !bytes.Equal(after, []byte(tc.text)) || info.Mode().Perm() != 0600 {
				t.Fatal("input changed")
			}
			leftovers, err := os.ReadDir(snapshotDir)
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("snapshot leaked: %v %v", leftovers, err)
			}
		})
	}
}
func TestLinuxFstabNoBlockingOrFalseSuccess(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	r := &runtime{values: map[string]value{}, filename: "fstab.yesh"}
	start := time.Now()
	if _, err := r.readFstab(fifo); err == nil {
		t.Fatal("accepted fifo")
	}
	if time.Since(start) > time.Second {
		t.Fatal("blocked opening fifo")
	}
	input := filepath.Join(dir, "fstab")
	writeTestFile(t, input, []byte("UUID=x /backup ext4 defaults 0 2\n"), 0600)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if _, err := r.readFstab(input); err == nil || !strings.Contains(err.Error(), "util-linux") {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(bin, "findmnt"), []byte("#!/bin/sh\necho '{\"filesystems\":[]}'\necho 'parse error at line 1 -- ignored' >&2\nexit 0\n"), 0700)
	if _, err := r.readFstab(input); err == nil || !strings.Contains(err.Error(), "refusing partial") {
		t.Fatal(err)
	}
	// A plan never opens input, creates a snapshot, or needs findmnt.
	t.Setenv("TMPDIR", filepath.Join(dir, "nonexistent"))
	t.Setenv("PATH", t.TempDir())
	r.plan = true
	v, err := r.eval("fstab "+strconvQuote(fifo), 1)
	if err != nil || v.fstab == nil {
		t.Fatalf("plan %v", err)
	}
	r.plan = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.ctx = ctx
	if _, err := r.readFstab(input); err == nil {
		t.Fatal("ignored cancellation")
	}
}
