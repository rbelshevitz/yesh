package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func storageFixture(devices ...storageDevice) []byte {
	data, _ := json.Marshal(map[string]any{"blockdevices": devices})
	return data
}
func TestStorageIdentityAndMount(t *testing.T) {
	d := storageDevice{Path: "/dev/test", Number: "8:1", UUID: "disk-id", PartUUID: "partition-id", FSType: "ext4"}
	other := d
	other.Path = "/dev/other"
	other.Number = "8:2"
	for _, kind := range []string{"uuid", "partuuid"} {
		id := d.UUID
		if kind == "partuuid" {
			id = d.PartUUID
		}
		for _, tc := range []struct {
			name string
			data []byte
			ok   bool
		}{
			{"unique", storageFixture(d), true}, {"same device repeated", storageFixture(d, d), true},
			{"ambiguous", storageFixture(d, other), false}, {"missing", storageFixture(), false},
			{"malformed", []byte(`{"blockdevices":`), false}, {"wrong schema", []byte(`{}`), false},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				got, err := selectStorageDevice(tc.data, kind, id)
				if (err == nil) != tc.ok {
					t.Fatalf("got %#v, %v", got, err)
				}
			})
		}
	}
	for _, tc := range []struct {
		name, raw, mode string
		ok              bool
	}{
		{"exact", `{"filesystems":[{"target":"/backup","source":"/dev/test","fstype":"ext4","maj:min":"8:1","fsroot":"/"}]}`, "", true},
		{"parent", `{"filesystems":[{"target":"/","source":"/dev/test","fstype":"ext4","maj:min":"8:1","fsroot":"/"}]}`, "", false},
		{"wrong device", `{"filesystems":[{"target":"/backup","source":"/dev/test","fstype":"ext4","maj:min":"8:2","fsroot":"/"}]}`, "", false},
		{"wrong fs", `{"filesystems":[{"target":"/backup","source":"/dev/test","fstype":"xfs","maj:min":"8:1","fsroot":"/"}]}`, "", false},
		{"subtree", `{"filesystems":[{"target":"/backup","source":"/dev/test[/data]","fstype":"ext4","maj:min":"8:1","fsroot":"/data"}]}`, "", false},
		{"readonly", `{"filesystems":[{"target":"/backup","source":"/dev/test","fstype":"ext4","maj:min":"8:1","fsroot":"/","vfs-options":"ro,relatime","fs-options":"ro,norecovery"}]}`, "readonly", true},
		{"readonly rejects writable", `{"filesystems":[{"target":"/backup","source":"/dev/test","fstype":"ext4","maj:min":"8:1","fsroot":"/","vfs-options":"rw,relatime","fs-options":"rw"}]}`, "readonly", false},
		{"writable", `{"filesystems":[{"target":"/backup","source":"/dev/test","fstype":"ext4","maj:min":"8:1","fsroot":"/","vfs-options":"rw,relatime","fs-options":"rw"}]}`, "writable", true},
		{"writable rejects readonly", `{"filesystems":[{"target":"/backup","source":"/dev/test","fstype":"ext4","maj:min":"8:1","fsroot":"/","vfs-options":"ro,relatime","fs-options":"ro,norecovery"}]}`, "writable", false},
		{"missing", `{"filesystems":[]}`, "", false}, {"invalid", `garbage`, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkStorageMount([]byte(tc.raw), "/backup", d, "ext4", tc.mode); (err == nil) != tc.ok {
				t.Fatal(err)
			}
		})
	}
}
func TestStorageSyntaxAndTypes(t *testing.T) {
	for _, source := range []string{"inspect $disk | cat", "inspect $disk > file", "require mounted /backup", "require mounted /backup from $disk filesystem ext4 | cat"} {
		if _, err := parse(source, "storage.yesh"); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
	for _, source := range []string{"require mounted /backup from $disk filesystem ext4 readonly", "require mounted /backup from $disk filesystem ext4 writable"} {
		if _, err := parse(source, "storage.yesh"); err != nil {
			t.Fatalf("rejected %s: %v", source, err)
		}
	}
	r := &runtime{values: map[string]value{"disk": {device: &storageDevice{}}}, filename: "storage.yesh", plan: true}
	for _, raw := range []string{"[$disk]", "regex $disk", "device by uuid $disk", "device by uuid [one,two]", "device by label x"} {
		if _, err := r.eval(raw, 3); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := r.expand("$disk", 3); err == nil || !strings.Contains(err.Error(), "device") {
		t.Fatal(err)
	}
	for _, raw := range []string{"exists $disk", "free_space $disk at_least 1B", "$disk matches regex x"} {
		if _, err := r.condition(raw, 3); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	// Quoted identities stay literal even if a variable has the same name.
	r.values["id"] = value{isList: true}
	if _, err := r.eval(`device by uuid "id"`, 3); err != nil {
		t.Fatal(err)
	}
}
