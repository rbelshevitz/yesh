package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFstabBoundsAndInspection(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"size", make([]byte, maxFstabBytes+1)},
		{"line", []byte(strings.Repeat("x", maxFstabLine+1))},
		{"count", []byte(strings.Repeat("x\n", maxFstabEntries+1))},
		{"nul", []byte("x\x00")}, {"utf8", []byte{255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := fstabRecordLines(tc.data); err == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
	lines, err := fstabRecordLines([]byte("# comment\r\n \t\nUUID=x /backup ext4 defaults 0 2\n"))
	if err != nil || len(lines) != 1 || lines[0] != 3 {
		t.Fatalf("lines %v %v", lines, err)
	}
	snapshot, err := decodeFstab([]byte(`{"filesystems":[{"source":"UUID=a","target":"/backup","fstype":"ext4","options":"ro","freq":0,"passno":2},{"source":"UUID=b","target":"/backup","fstype":"ext4","options":"rw","freq":0,"passno":2},{"source":"/swap1","target":"none","fstype":"swap"},{"source":"/swap2","target":"none","fstype":"swap"}]}`), []int{1, 2, 3, 4})
	if err != nil || len(snapshot.Diagnostics) != 1 || snapshot.Diagnostics[0].Code != "duplicate-target" {
		t.Fatalf("snapshot %#v %v", snapshot, err)
	}
	if _, err := decodeFstab([]byte(`{"filesystems":[]}`), []int{1}); err == nil {
		t.Fatal("accepted partial table")
	}
	s := &fstabSnapshot{Entries: []fstabEntry{{Source: "//user:sourceSecret@host/share", Target: "/mnt/data", FSType: "cifs", Options: `username=alice,password=optionSecret,context="system_u:object_r:tmp_t:s0:c1,c2",ro,x-systemd.requires=a,x-systemd.requires=b`}}}
	var out bytes.Buffer
	if err := writeFstabInspection(&out, s); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sourceSecret", "optionSecret", "alice", "system_u", "c1", "c2"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("leaked %s: %s", secret, out.String())
		}
	}
	if got := fstabOptionNames(s.Entries[0].Options); len(got) != 6 || got[3] != "ro" || got[4] != got[5] {
		t.Fatalf("option names %q", got)
	}
}
func TestFstabTypedValues(t *testing.T) {
	r := &runtime{filename: "fstab.yesh", values: map[string]value{"mounts": {fstab: &fstabSnapshot{}}}, plan: true}
	for _, raw := range []string{"[$mounts]", "regex $mounts", "fstab $mounts", "device by uuid $mounts", "fstab", "fstab a b"} {
		if _, err := r.eval(raw, 1); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := r.expand("$mounts", 1); err == nil {
		t.Fatal("expanded structured table")
	}
	items, err := parse("mounts = fstab /nonexistent/fstab\ninspect $mounts", "fstab.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}
