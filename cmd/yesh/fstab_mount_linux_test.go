//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxFstabMountPreflight(t *testing.T) {
	dir := t.TempDir()
	bin := t.TempDir()
	mount := filepath.Join(dir, "backup disk")
	if err := os.Mkdir(mount, 0700); err != nil {
		t.Fatal(err)
	}
	devicesFile := filepath.Join(dir, "devices.json")
	mountFile := filepath.Join(dir, "mounted.json")
	log := filepath.Join(dir, "log")
	t.Setenv("PATH", bin)
	t.Setenv("DEVICES", devicesFile)
	t.Setenv("MOUNTED", mountFile)
	t.Setenv("ARGV_LOG", log)
	for _, name := range []string{"lsblk", "findmnt"} {
		variable := "DEVICES"
		if name == "findmnt" {
			variable = "MOUNTED"
		}
		script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$0\" \"$@\" >> \"$ARGV_LOG\"\nwhile IFS= read -r line; do printf '%%s\\n' \"$line\"; done < \"$%s\"\n", variable)
		writeTestFile(t, filepath.Join(bin, name), []byte(script), 0700)
	}
	writeTestFile(t, filepath.Join(bin, "echo"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0700)
	device := storageDevice{Path: "/dev/test", Number: "8:1", UUID: "backup-id", PartUUID: "partition-id", FSType: "ext4"}
	base := fstabEntry{Source: "UUID=backup-id", Target: mount, FSType: "ext4", Options: "noauto,nofail,x-systemd.automount", Line: 4}
	for _, tc := range []struct {
		name                                           string
		source, fs, number, root                       string
		duplicate, missingEntry, missingDevice, autofs bool
		ok                                             bool
	}{
		{name: "uuid success", ok: true},
		{name: "partuuid success", source: `PARTUUID="partition-id"`, ok: true},
		{name: "wrong uuid", source: "UUID=wrong-id"},
		{name: "wrong filesystem", fs: "xfs"},
		{name: "wrong active device", number: "8:2"},
		{name: "filesystem subtree", root: "/data"},
		{name: "duplicate entry", duplicate: true},
		{name: "missing entry", missingEntry: true},
		{name: "missing device", missingDevice: true},
		{name: "autofs only", autofs: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			if tc.source != "" {
				e.Source = tc.source
			}
			if tc.fs != "" {
				e.FSType = tc.fs
			}
			entries := []fstabEntry{e}
			if tc.duplicate {
				entries = append(entries, e)
			}
			if tc.missingEntry {
				entries = nil
			}
			devices := []storageDevice{device}
			if tc.missingDevice {
				devices = nil
			}
			writeTestFile(t, devicesFile, append(storageFixture(devices...), '\n'), 0600)
			number := device.Number
			if tc.number != "" {
				number = tc.number
			}
			root := "/"
			if tc.root != "" {
				root = tc.root
			}
			fs := device.FSType
			if tc.autofs {
				fs = "autofs"
			}
			data, _ := json.Marshal(map[string]any{"filesystems": []any{map[string]string{"target": mount, "source": device.Path, "fstype": fs, "maj:min": number, "fsroot": root}}})
			writeTestFile(t, mountFile, append(data, '\n'), 0600)
			output := filepath.Join(t.TempDir(), "ready")
			r := &runtime{filename: "mount.yesh", values: map[string]value{"mounts": {fstab: &fstabSnapshot{Entries: entries}}}}
			items, err := parse("require mounted "+strconvQuote(mount)+" from $mounts\necho ready > "+strconvQuote(output), r.filename)
			if err != nil {
				t.Fatal(err)
			}
			err = r.execute(items)
			if (err == nil) != tc.ok {
				t.Fatalf("execute %v", err)
			}
			bytes, readErr := os.ReadFile(output)
			if tc.ok {
				if readErr != nil || string(bytes) != "ready\n" {
					t.Fatalf("ready %q %v", bytes, readErr)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatal("published success after failed check")
			}
		})
	}
	args, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--nocanonicalize\n") || !strings.Contains(string(args), "--kernel\n") || !strings.Contains(string(args), "--mountpoint\n"+mount+"\n") {
		t.Fatalf("unsafe query arguments: %s", args)
	}
	// A missing path is intentional: mismatched autofs must fail BEFORE any stat.
	missing := filepath.Join(dir, "not-traversed")
	e := base
	e.Target = missing
	data, _ := json.Marshal(map[string]any{"filesystems": []any{map[string]string{"target": missing, "source": "systemd-1", "fstype": "autofs", "maj:min": "0:42", "fsroot": "/"}}})
	writeTestFile(t, devicesFile, append(storageFixture(device), '\n'), 0600)
	writeTestFile(t, mountFile, append(data, '\n'), 0600)
	r := &runtime{values: map[string]value{"mounts": {fstab: &fstabSnapshot{Entries: []fstabEntry{e}}}}}
	items, _ := parse("require mounted "+strconvQuote(missing)+" from $mounts", "autofs.yesh")
	if err := r.execute(items); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("path accessed before autofs rejection: %v", err)
	}
}

func TestLinuxFstabRealMountBinding(t *testing.T) {
	for _, name := range []string{"lsblk", "findmnt"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("missing %s (util-linux package)", name)
		}
	}
	r := &runtime{filename: "real-fstab.yesh", values: map[string]value{}}
	data, err := r.storageQuery("lsblk", "--json", "--list", "--paths", "--output", "PATH,MAJ:MIN,UUID,PARTUUID,FSTYPE,LABEL,RO")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Devices []storageDevice `json:"blockdevices"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	root, err := r.storageQuery("findmnt", "--kernel", "--json", "--list", "--nocanonicalize", "--mountpoint", "/", "--output", "TARGET,SOURCE,FSTYPE,MAJ:MIN,FSROOT")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Devices {
		if d.UUID == "" || d.FSType == "" || checkStorageMount(root, "/", d, d.FSType) != nil {
			continue
		}
		table := filepath.Join(t.TempDir(), "fstab")
		// Synthetic config, real device and real active mount; no host config changes.
		writeTestFile(t, table, []byte(fmt.Sprintf("UUID=%s / %s defaults 0 1\n", d.UUID, d.FSType)), 0600)
		example, err := os.ReadFile(filepath.Join("..", "..", "examples", "fstab-mount-check.yesh"))
		if err != nil {
			t.Fatal(err)
		}
		source := strings.ReplaceAll(strings.Replace(string(example), `"/etc/fstab"`, strconvQuote(table), 1), "/mnt/backup", "/")
		items, err := parse(source, r.filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.execute(items); err != nil {
			t.Fatal(err)
		}
		// A configured-but-unmounted backup must not pass because its parent is mounted.
		path := t.TempDir()
		writeTestFile(t, table, []byte(fmt.Sprintf("UUID=%s %s %s nofail 0 2\n", d.UUID, path, d.FSType)), 0600)
		items, err = parse("mounts = fstab "+strconvQuote(table)+"\nrequire mounted "+strconvQuote(path)+" from $mounts", r.filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.execute(items); err == nil {
			t.Fatal("ordinary directory passed active mount check")
		}
		t.Logf("synthetic fstab matched real %s (%s); unmounted directory rejected", d.Path, d.FSType)
		return
	}
	t.Skip("root has no readable UUID-backed whole filesystem; real fstab/mount binding unproven")
}
