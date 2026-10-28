package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type storageDevice struct {
	Path           string `json:"path"`
	Number         string `json:"maj:min"`
	UUID           string `json:"uuid"`
	PartUUID       string `json:"partuuid"`
	FSType         string `json:"fstype"`
	Label          string `json:"label"`
	ReadOnly       bool   `json:"ro"`
	kind, identity string
}

func storageKeyword(t commandToken, word string) bool {
	return !t.literal && !t.operator && t.text == word
}
func validateStorageStatement(t []commandToken) error {
	if len(t) == 2 && storageKeyword(t[0], "inspect") && !t[1].literal && !t[1].operator {
		return nil
	}
	if len(t) == 5 && storageKeyword(t[0], "require") && storageKeyword(t[1], "mounted") && storageKeyword(t[3], "from") && !t[2].operator && !t[4].operator && !t[4].literal {
		return nil
	}
	if len(t) == 7 && storageKeyword(t[0], "require") && storageKeyword(t[1], "mounted") && storageKeyword(t[3], "from") && storageKeyword(t[5], "filesystem") && !t[2].operator && !t[4].operator && !t[4].literal && !t[6].operator {
		return nil
	}
	return fmt.Errorf("expected: inspect value; require mounted path from fstab_snapshot; or require mounted path from device filesystem type (without pipeline or redirection)")
}
func (r *runtime) storageString(t commandToken, line int) (string, error) {
	v, err := r.conditionValue(t, line)
	if err != nil {
		return "", err
	}
	if v.fstab != nil || v.device != nil || v.isProcesses || v.isList || v.regex != nil || v.secret || v.scalar == "" || strings.ContainsRune(v.scalar, 0) {
		return "", fmt.Errorf("storage expects one non-empty, non-secret string")
	}
	return v.scalar, nil
}
func (r *runtime) evalDevice(raw string, line int) (value, error) {
	t, err := words(raw)
	if err != nil {
		return value{}, err
	}
	if len(t) != 4 || !storageKeyword(t[0], "device") || !storageKeyword(t[1], "by") || !(storageKeyword(t[2], "uuid") || storageKeyword(t[2], "partuuid")) || t[3].operator {
		return value{}, fmt.Errorf("%s:%d: expected: device by uuid|partuuid identity", r.filename, line)
	}
	id, err := r.storageString(t[3], line)
	if err != nil {
		return value{}, fmt.Errorf("%s:%d: %w", r.filename, line, err)
	}
	d := storageDevice{kind: t[2].text, identity: id}
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN device by %s %q (deferred)\n", d.kind, id)
	} else {
		d, err = r.resolveStorage(d.kind, id)
		if err != nil {
			return value{}, fmt.Errorf("%s:%d: device: %w", r.filename, line, err)
		}
	}
	return value{device: &d}, nil
}
func (r *runtime) resolveStorage(kind, id string) (storageDevice, error) {
	data, err := r.storageQuery("lsblk", "--json", "--list", "--paths", "--output", "PATH,MAJ:MIN,UUID,PARTUUID,FSTYPE,LABEL,RO")
	if err != nil {
		return storageDevice{}, err
	}
	return selectStorageDevice(data, kind, id)
}
func selectStorageDevice(data []byte, kind, id string) (storageDevice, error) {
	var report struct {
		Devices []storageDevice `json:"blockdevices"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return storageDevice{}, fmt.Errorf("invalid lsblk JSON: %w", err)
	}
	var found []storageDevice
	seen := map[string]bool{}
	for _, d := range report.Devices {
		key := d.UUID
		if kind == "partuuid" {
			key = d.PartUUID
		}
		if key != id {
			continue
		}
		if !filepath.IsAbs(d.Path) || !strings.HasPrefix(d.Path, "/dev/") || !validDeviceNumber(d.Number) {
			return storageDevice{}, fmt.Errorf("invalid device record")
		}
		if seen[d.Number] {
			continue
		}
		seen[d.Number] = true
		d.kind = kind
		d.identity = id
		found = append(found, d)
	}
	if len(found) != 1 {
		return storageDevice{}, fmt.Errorf("%s %q resolved to %d devices; expected exactly one (missing or inaccessible metadata is not a match)", kind, id, len(found))
	}
	return found[0], nil
}
func validDeviceNumber(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
func (r *runtime) executeStorage(s storageStmt) error {
	t := s.terms
	name := t[1].text
	if t[0].text == "require" {
		name = t[4].text
	}
	v, ok := r.values[name]
	if t[0].text == "require" && len(t) == 5 {
		if !ok || v.fstab == nil {
			return fmt.Errorf("require mounted without filesystem expects a fstab snapshot: %s", name)
		}
		path, err := r.storageString(t[2], s.line)
		if err != nil {
			return err
		}
		if err := validateMountPath(path); err != nil {
			return err
		}
		if r.plan {
			fmt.Fprintf(os.Stdout, "PLAN require mounted %q from %s (deferred fstab/device/mount checks)\n", path, name)
			return nil
		}
		entry, kind, id, err := selectFstabMount(v.fstab, path)
		if err != nil {
			return err
		}
		d, err := r.resolveStorage(kind, id)
		if err != nil {
			return fmt.Errorf("fstab line %d: %w", entry.Line, err)
		}
		if err := r.requireMountedDevice(path, d, entry.FSType); err != nil {
			return fmt.Errorf("fstab line %d: %w", entry.Line, err)
		}
		return nil
	}
	if ok && v.fstab != nil && t[0].text == "inspect" {
		if r.plan {
			fmt.Fprintf(os.Stdout, "PLAN inspect %s (deferred)\n", name)
			return nil
		}
		return writeFstabInspection(os.Stdout, v.fstab)
	}
	if !ok || v.device == nil {
		if t[0].text == "inspect" {
			return fmt.Errorf("inspect expects a device or fstab snapshot: %s", name)
		}
		return fmt.Errorf("storage expects a device value: %s", name)
	}
	if t[0].text == "inspect" {
		if r.plan {
			fmt.Fprintf(os.Stdout, "PLAN inspect %s (deferred)\n", name)
			return nil
		}
		return json.NewEncoder(os.Stdout).Encode(v.device)
	}
	path, err := r.storageString(t[2], s.line)
	if err != nil {
		return err
	}
	fs, err := r.storageString(t[6], s.line)
	if err != nil {
		return err
	}
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN require mounted %q from %s filesystem %q (deferred)\n", path, name, fs)
		return nil
	}
	return r.requireMountedDevice(path, *v.device, fs)
}

func validateMountPath(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("mountpoint must be a clean absolute directory path")
	}
	return nil
}
func (r *runtime) requireMountedDevice(path string, original storageDevice, fs string) error {
	if err := validateMountPath(path); err != nil {
		return err
	}
	d, err := r.resolveStorage(original.kind, original.identity)
	if err != nil {
		return err
	}
	if d.Number != original.Number || d.Path != original.Path {
		return fmt.Errorf("device changed since lookup")
	}
	if d.FSType != fs {
		return fmt.Errorf("device filesystem %q does not match %q", d.FSType, fs)
	}
	// Do not traverse an automount target merely to discover that its real FS is absent.
	data, err := r.storageQueryPolicy(true, "findmnt", "--kernel", "--json", "--list", "--nocanonicalize", "--mountpoint", path, "--output", "TARGET,SOURCE,FSTYPE,MAJ:MIN,FSROOT")
	if err != nil {
		return err
	}
	if err := checkStorageMount(data, path, d, fs); err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if real != path {
		return fmt.Errorf("symlink mountpoint is not allowed")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("mountpoint is not a directory")
	}
	return nil
}

func checkStorageMount(data []byte, path string, d storageDevice, fs string) error {
	var report struct {
		Filesystems []struct {
			Target string `json:"target"`
			Source string `json:"source"`
			FSType string `json:"fstype"`
			Number string `json:"maj:min"`
			Root   string `json:"fsroot"`
		} `json:"filesystems"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("invalid findmnt JSON: %w", err)
	}
	if len(report.Filesystems) != 1 {
		return fmt.Errorf("expected exactly one mount at %q", path)
	}
	m := report.Filesystems[0]
	if m.Target != path || m.Number != d.Number || m.FSType != fs || m.Root != "/" || m.Source != d.Path {
		return fmt.Errorf("mount at %q does not match device %q and filesystem %q, or is a filesystem subtree", path, d.Path, fs)
	}
	return nil
}
