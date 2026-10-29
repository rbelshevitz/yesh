//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinuxStoragePreflight(t *testing.T) {
	bin := t.TempDir()
	dir := t.TempDir()
	mount := filepath.Join(dir, "backup disk")
	if err := os.Mkdir(mount, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	writeTestFile(t, filepath.Join(bin, "echo"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0700)
	d := storageDevice{Path: "/dev/test", Number: "8:1", UUID: "disk-id", PartUUID: "part-id", FSType: "ext4"}
	fixture := filepath.Join(dir, "devices.json")
	mountFixture := filepath.Join(dir, "mount.json")
	log := filepath.Join(dir, "argv")
	t.Setenv("STORAGE_DEVICES", fixture)
	t.Setenv("STORAGE_MOUNT", mountFixture)
	t.Setenv("STORAGE_LOG", log)
	for _, name := range []string{"lsblk", "findmnt"} {
		variable := "STORAGE_DEVICES"
		if name == "findmnt" {
			variable = "STORAGE_MOUNT"
		}
		script := "#!/bin/sh\nprintf '%s\\n' \"$0\" \"$@\" >> \"$STORAGE_LOG\"\nwhile IFS= read -r line; do printf '%s\\n' \"$line\"; done < \"$" + variable + "\"\n"
		writeTestFile(t, filepath.Join(bin, name), []byte(script), 0700)
	}
	mountJSON := func(number string) []byte {
		data, _ := json.Marshal(map[string]any{"filesystems": []any{map[string]string{"target": mount, "source": d.Path, "fstype": "ext4", "maj:min": number, "fsroot": "/"}}})
		return append(data, '\n')
	}
	source := "disk = device by uuid \"disk-id\"\ninspect $disk\nrequire mounted " + strconvQuote(mount) + " from $disk filesystem ext4\necho passed > " + strconvQuote(filepath.Join(dir, "published")) + "\n"
	for _, tc := range []struct {
		name           string
		devices, mount []byte
		ok             bool
	}{
		{"success", append(storageFixture(d), '\n'), mountJSON("8:1"), true},
		{"wrong mount", append(storageFixture(d), '\n'), mountJSON("8:2"), false},
		{"missing mount", append(storageFixture(d), '\n'), []byte("{\"filesystems\":[]}\n"), false},
		{"duplicate uuid", append(storageFixture(d, storageDevice{Path: "/dev/other", Number: "8:2", UUID: "disk-id"}), '\n'), mountJSON("8:1"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(dir, "published")
			_ = os.Remove(output)
			writeTestFile(t, fixture, tc.devices, 0600)
			writeTestFile(t, mountFixture, tc.mount, 0600)
			items, err := parse(source, "storage.yesh")
			if err != nil {
				t.Fatal(err)
			}
			r := &runtime{values: map[string]value{}, filename: "storage.yesh"}
			err = r.execute(items)
			if (err == nil) != tc.ok {
				t.Fatalf("execute: %v", err)
			}
			data, readErr := os.ReadFile(output)
			if tc.ok {
				if readErr != nil || string(data) != "passed\n" {
					t.Fatalf("publication: %q %v", data, readErr)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("published after failure: %q %v", data, readErr)
			}
		})
	}
	// Revalidation must fail when identity now belongs to a different device.
	writeTestFile(t, fixture, append(storageFixture(d), '\n'), 0600)
	r := &runtime{values: map[string]value{}, filename: "storage.yesh"}
	v, err := r.eval(`device by uuid "disk-id"`, 1)
	if err != nil {
		t.Fatal(err)
	}
	r.values["disk"] = v
	changed := d
	changed.Number = "8:2"
	changed.Path = "/dev/other"
	writeTestFile(t, fixture, append(storageFixture(changed), '\n'), 0600)
	check, _ := parse("require mounted "+strconvQuote(mount)+" from $disk filesystem ext4", "storage.yesh")
	if err := r.execute(check); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("device change: %v", err)
	}
	writeTestFile(t, fixture, append(storageFixture(d), '\n'), 0600)
	check, _ = parse("require mounted "+strconvQuote(mount)+" from $disk filesystem xfs", "storage.yesh")
	if err := r.execute(check); err == nil || !strings.Contains(err.Error(), "filesystem") {
		t.Fatalf("filesystem: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(mount, link); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, mountFixture, []byte(strings.ReplaceAll(string(mountJSON("8:1")), strconvQuote(mount), strconvQuote(link))), 0600)
	check, _ = parse("require mounted "+strconvQuote(link)+" from $disk filesystem ext4", "storage.yesh")
	if err := r.execute(check); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink: %v", err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "--mountpoint\n"+mount+"\n") {
		t.Fatalf("argv lost path boundary: %s", data)
	}
	// Plan must not need either utilities or a real mount directory.
	t.Setenv("PATH", t.TempDir())
	items, err := parse(strings.ReplaceAll(source, mount, "/nonexistent/storage-mount"), "storage.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&runtime{values: map[string]value{}, plan: true}).execute(items); err != nil {
		t.Fatal(err)
	}
}
func strconvQuote(s string) string { data, _ := json.Marshal(s); return string(data) }

func TestLinuxStorageQueryFailuresAndCancellation(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	r := &runtime{}
	if _, err := r.storageQuery("lsblk"); err == nil || !strings.Contains(err.Error(), "util-linux") {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(bin, "lsblk"), []byte("#!/bin/sh\necho denied >&2\nexit 2\n"), 0700)
	if _, err := r.storageQuery("lsblk"); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(bin, "lsblk"), []byte("#!/bin/sh\ntrap '' TERM\nwhile :; do :; done\n"), 0700)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	r.ctx = ctx
	start := time.Now()
	if _, err := r.storageQuery("lsblk"); err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatal(err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("query failed to stop")
	}
	var out storageOutput
	out.Write(make([]byte, maxStorageOutput+1))
	if !out.exceeded || out.Len() != maxStorageOutput {
		t.Fatal("output cap failed")
	}
}
func TestLinuxStorageRealReadOnly(t *testing.T) {
	for _, name := range []string{"lsblk", "findmnt"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("missing %s (util-linux package)", name)
		}
	}
	r := &runtime{}
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
	for _, d := range report.Devices {
		if d.UUID != "" {
			got, err := r.resolveStorage("uuid", d.UUID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Number != d.Number {
				t.Fatal("identity mismatch")
			}
			t.Logf("resolved %s %s", got.Path, got.UUID)
			return
		}
	}
	t.Skip("lsblk returned no readable UUID; real identity resolution unproven")
}

func TestLinuxStorageRealMountRequirement(t *testing.T) {
	for _, name := range []string{"lsblk", "findmnt"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("missing %s (util-linux package)", name)
		}
	}
	r := &runtime{values: map[string]value{}, filename: "real-storage.yesh"}
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
	root, err := r.storageQuery("findmnt", "--json", "--mountpoint", "/", "--output", "TARGET,SOURCE,FSTYPE,MAJ:MIN,FSROOT")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Devices {
		if d.UUID == "" || d.FSType == "" || checkStorageMount(root, "/", d, d.FSType) != nil {
			continue
		}
		items, err := parse("disk = device by uuid "+strconvQuote(d.UUID)+"\nrequire mounted / from $disk filesystem "+strconvQuote(d.FSType), r.filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.execute(items); err != nil {
			t.Fatal(err)
		}
		items, err = parse("require mounted "+strconvQuote(t.TempDir())+" from $disk filesystem "+strconvQuote(d.FSType), r.filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.execute(items); err == nil {
			t.Fatal("ordinary directory passed exact mount requirement")
		}
		t.Logf("verified real root mount %s (%s); ordinary directory rejected", d.Path, d.FSType)
		return
	}
	t.Skip("root mount has no accessible unique block-device UUID; real mount requirement unproven")
}
