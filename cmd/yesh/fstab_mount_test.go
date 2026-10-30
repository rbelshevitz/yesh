package main

import (
	"strings"
	"testing"
)

func TestFstabMountSelection(t *testing.T) {
	base := fstabEntry{Source: "UUID=backup-id", Target: "/backup", FSType: "ext4", Options: "noauto,nofail,x-systemd.automount", Line: 3}
	for _, tc := range []struct {
		name string
		edit func(*fstabEntry)
		ok   bool
	}{
		{"uuid", func(e *fstabEntry) {}, true},
		{"quoted partuuid", func(e *fstabEntry) { e.Source = `PARTUUID="partition-id"` }, true},
		{"empty identity", func(e *fstabEntry) { e.Source = `UUID=""` }, false},
		{"malformed quotes", func(e *fstabEntry) { e.Source = `UUID="a` }, false},
		{"path source", func(e *fstabEntry) { e.Source = "/dev/sda" }, false},
		{"label source", func(e *fstabEntry) { e.Source = "LABEL=backup" }, false},
		{"network", func(e *fstabEntry) { e.Source = "server:/data"; e.FSType = "nfs" }, false},
		{"swap", func(e *fstabEntry) { e.FSType = "swap" }, false},
		{"auto", func(e *fstabEntry) { e.FSType = "auto" }, false},
		{"type list", func(e *fstabEntry) { e.FSType = "ext4,xfs" }, false},
		{"unclean target", func(e *fstabEntry) { e.Target = "/backup/" }, false},
		{"quoted option comma", func(e *fstabEntry) { e.Options = `context="a,b",noauto` }, true},
		{"bind", func(e *fstabEntry) { e.Options = "bind" }, false},
		{"rbind", func(e *fstabEntry) { e.Options = "rbind" }, false},
		{"remount", func(e *fstabEntry) { e.Options = "remount,ro" }, false},
		{"loop", func(e *fstabEntry) { e.Options = "loop=/dev/loop0" }, false},
		{"subvol", func(e *fstabEntry) { e.Options = "subvol=/backup"; e.FSType = "btrfs" }, false},
		{"subvolid", func(e *fstabEntry) { e.Options = "subvolid=256"; e.FSType = "btrfs" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			tc.edit(&e)
			got, kind, id, err := selectFstabMount(&fstabSnapshot{Entries: []fstabEntry{e}}, "/backup")
			if (err == nil) != tc.ok {
				t.Fatalf("entry %#v, %v", got, err)
			}
			if tc.ok && (kind == "" || id == "" || got.Line != 3) {
				t.Fatalf("identity %q %q entry %#v", kind, id, got)
			}
		})
	}
	for _, entries := range [][]fstabEntry{nil, {base, base}, {base, {Target: "/backup/.", Line: 4}}} {
		if _, _, _, err := selectFstabMount(&fstabSnapshot{Entries: entries}, "/backup"); err == nil {
			t.Fatal("ambiguous/missing target accepted")
		}
	}
}
func TestFstabMountSyntaxAndPlan(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "mount.yesh", plan: true}
	for _, source := range []string{
		"mounts = fstab /does-not-exist\nrequire mounted /backup from $mounts",
		"path = /backup\nmounts = fstab /does-not-exist\nrequire mounted $path from $mounts",
	} {
		items, err := parse(source, r.filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.execute(items); err != nil {
			t.Fatal(err)
		}
	}
	for _, source := range []string{
		"require mounted /backup from $mounts | cat",
		"require mounted /backup from $mounts > file",
		"require mounted /backup from \"mounts\"",
	} {
		if _, err := parse(source, r.filename); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
	items, err := parse("require mounted /backup from $unknown", r.filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil || !strings.Contains(err.Error(), "fstab snapshot") {
		t.Fatal(err)
	}
	r.values["disk"] = value{device: &storageDevice{}}
	items, err = parse("require mounted /backup from $disk", r.filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err == nil {
		t.Fatal("device accepted as fstab table")
	}
}

func FuzzFstabMountIdentity(f *testing.F) {
	f.Add("093ca441-e3f0-4214-b5d2-33d98b6e40d4", true, false)
	f.Add("A40D-85E7", false, true)
	f.Add("", true, true)
	f.Add("quoted\" value", false, false)
	f.Fuzz(func(t *testing.T, id string, quoted, partition bool) {
		if len(id) > maxFstabLine {
			return
		}
		// Quoting is supplied below. Keep generated quote characters INSIDE
		// the payload, so a value such as '0' cannot accidentally become an
		// enclosing pair of fstab quotes rather than an invalid UUID payload.
		if strings.ContainsAny(id, "\"'") {
			id = "prefix" + id + "suffix"
		}
		tag := "UUID"
		if partition {
			tag = "PARTUUID"
		}
		encoded := id
		if quoted {
			encoded = "\"" + id + "\""
		}
		snapshot := &fstabSnapshot{Entries: []fstabEntry{{Source: tag + "=" + encoded, Target: "/backup", FSType: "ext4", Line: 1}}}
		_, kind, got, err := selectFstabMount(snapshot, "/backup")
		valid := id != "" && !strings.ContainsAny(id, "\x00\r\n\t \"'")
		if valid {
			if err != nil || got != id || kind != strings.ToLower(tag) {
				t.Fatalf("identity changed: kind=%q got=%q err=%v", kind, got, err)
			}
		} else if err == nil {
			t.Fatal("invalid identity accepted")
		}
	})
}

// Regression for the minimized fuzz input: enclosing quotes are fstab syntax,
// while quotes embedded in an identity are invalid payload characters.
func TestFstabMountIdentityQuoting(t *testing.T) {
	for _, source := range []string{"UUID=0", `UUID="0"`, "UUID='0'"} {
		snapshot := &fstabSnapshot{Entries: []fstabEntry{{Source: source, Target: "/backup", FSType: "ext4"}}}
		_, kind, id, err := selectFstabMount(snapshot, "/backup")
		if err != nil || kind != "uuid" || id != "0" {
			t.Fatalf("source %q: kind %q id %q error %v", source, kind, id, err)
		}
	}
}
