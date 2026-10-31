package main

import (
	"errors"
	"strings"
	"testing"
)

func TestReadOSReleaseUsesIDLikeAndQuotedValues(t *testing.T) {
	data := map[string][]byte{
		"/etc/os-release": []byte("ID=ubuntu\nID_LIKE=debian\nPRETTY_NAME=\"Ubuntu 24.04 LTS\"\n"),
	}
	info, err := readOSRelease(func(path string) ([]byte, error) {
		value, ok := data[path]
		if !ok {
			return nil, errors.New("missing")
		}
		return value, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.id != "ubuntu" || len(info.idLike) != 1 || info.idLike[0] != "debian" || info.prettyName != "Ubuntu 24.04 LTS" {
		t.Fatalf("unexpected os-release: %#v", info)
	}
}

func TestReadOSReleaseRejectsMissingID(t *testing.T) {
	_, err := readOSRelease(func(string) ([]byte, error) {
		return []byte("NAME=unknown\n"), nil
	})
	if err == nil || !strings.Contains(err.Error(), "no ID") {
		t.Fatalf("expected missing ID error, got %v", err)
	}
}

func TestPackageHintsStayScopedToKnownFamilies(t *testing.T) {
	if hint := packageHints["debian"]["arp-scan"]; hint.packageName != "arp-scan" || hint.manager != "apt-get" {
		t.Fatalf("unexpected Debian hint: %#v", hint)
	}
	if _, ok := packageHints["debian"]["tar"]; ok {
		t.Fatal("base tar must not have a scenario package hint")
	}
	if _, ok := packageHints["fedora"]["lsinitramfs"]; ok {
		t.Fatal("unconfirmed Fedora lsinitramfs mapping must not be suggested")
	}
}

func TestMissingCommandHintUsesOnlyMatchingFamily(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		if path == "/etc/os-release" {
			return []byte("ID=ubuntu\nID_LIKE=debian\nPRETTY_NAME=\"Ubuntu\"\n"), nil
		}
		return nil, errors.New("missing")
	}
	lookPath := func(name string) (string, error) {
		if name == "apt-get" {
			return "/usr/bin/apt-get", nil
		}
		return "", errors.New("not found")
	}
	hint := missingCommandHintWith("rsync", readFile, lookPath)
	if !strings.Contains(hint, "apt-get install rsync") || strings.Contains(hint, "dnf") {
		t.Fatalf("unexpected Ubuntu hint: %q", hint)
	}
}

func TestMissingCommandHintStaysQuietWithoutManagerOrForPath(t *testing.T) {
	readFile := func(string) ([]byte, error) {
		return []byte("ID=ubuntu\nID_LIKE=debian\n"), nil
	}
	noManager := func(string) (string, error) { return "", errors.New("not found") }
	if hint := missingCommandHintWith("rsync", readFile, noManager); hint != "" {
		t.Fatalf("unexpected hint without manager: %q", hint)
	}
	if hint := missingCommandHintWith("/opt/rsync", readFile, noManager); hint != "" {
		t.Fatalf("unexpected hint for explicit path: %q", hint)
	}
}
