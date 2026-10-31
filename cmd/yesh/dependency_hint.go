package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type osReleaseInfo struct {
	id         string
	idLike     []string
	prettyName string
}

type packageHint struct {
	manager     string
	packageName string
}

// These are dependencies of yesh's documented practical scenarios. This is
// deliberately not a general command-to-package database.
var packageHints = map[string]map[string]packageHint{
	"debian": {
		"arp-scan":    {manager: "apt-get", packageName: "arp-scan"},
		"rsync":       {manager: "apt-get", packageName: "rsync"},
		"pv":          {manager: "apt-get", packageName: "pv"},
		"rsnapshot":   {manager: "apt-get", packageName: "rsnapshot"},
		"lsinitramfs": {manager: "apt-get", packageName: "initramfs-tools-core"},
		"cpio":        {manager: "apt-get", packageName: "cpio"},
		"zstd":        {manager: "apt-get", packageName: "zstd"},
		"findmnt":     {manager: "apt-get", packageName: "util-linux"},
		"flock":       {manager: "apt-get", packageName: "util-linux"},
		"ip":          {manager: "apt-get", packageName: "iproute2"},
		"ss":          {manager: "apt-get", packageName: "iproute2"},
		"ssh":         {manager: "apt-get", packageName: "openssh-client"},
		"scp":         {manager: "apt-get", packageName: "openssh-client"},
	},
	"fedora": {
		"arp-scan": {manager: "dnf", packageName: "arp-scan"},
		"rsync":    {manager: "dnf", packageName: "rsync"},
		"pv":       {manager: "dnf", packageName: "pv"},
		"cpio":     {manager: "dnf", packageName: "cpio"},
		"zstd":     {manager: "dnf", packageName: "zstd"},
		"findmnt":  {manager: "dnf", packageName: "util-linux"},
		"flock":    {manager: "dnf", packageName: "util-linux"},
		"ip":       {manager: "dnf", packageName: "iproute"},
		"ss":       {manager: "dnf", packageName: "iproute"},
		"ssh":      {manager: "dnf", packageName: "openssh-clients"},
		"scp":      {manager: "dnf", packageName: "openssh-clients"},
	},
}

func missingCommandHint(command string) string {
	return missingCommandHintWith(command, os.ReadFile, exec.LookPath)
}

func missingCommandHintWith(command string, readFile func(string) ([]byte, error), lookPath func(string) (string, error)) string {
	if strings.ContainsAny(command, `/\\`) {
		return ""
	}
	info, err := readOSRelease(readFile)
	if err != nil {
		return ""
	}
	for _, family := range append([]string{info.id}, info.idLike...) {
		hint, ok := packageHints[family][command]
		if !ok {
			continue
		}
		if _, err := lookPath(hint.manager); err != nil {
			return ""
		}
		return fmt.Sprintf("hint: on %s the package is %s; install it with %s install %s", displayOS(info), hint.packageName, hint.manager, hint.packageName)
	}
	return ""
}

func displayOS(info osReleaseInfo) string {
	if info.prettyName != "" {
		return info.prettyName
	}
	if info.id != "" {
		return info.id
	}
	return "Linux"
}

func readOSRelease(readFile func(string) ([]byte, error)) (osReleaseInfo, error) {
	var data []byte
	var err error
	for _, path := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		data, err = readFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return osReleaseInfo{}, err
	}
	var info osReleaseInfo
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok || !validOSReleaseKey(key) {
			continue
		}
		value, err := parseOSReleaseValue(raw)
		if err != nil {
			return osReleaseInfo{}, fmt.Errorf("invalid %s: %w", key, err)
		}
		switch key {
		case "ID":
			info.id = value
		case "ID_LIKE":
			info.idLike = strings.Fields(value)
		case "PRETTY_NAME":
			info.prettyName = value
		}
	}
	if info.id == "" {
		return osReleaseInfo{}, errors.New("os-release has no ID")
	}
	return info, nil
}

func validOSReleaseKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func parseOSReleaseValue(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || (raw[0] != '\'' && raw[0] != '"') {
		return raw, nil
	}
	if raw[0] == '\'' {
		if len(raw) < 2 || raw[len(raw)-1] != '\'' {
			return "", errors.New("unterminated single-quoted value")
		}
		return raw[1 : len(raw)-1], nil
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", err
	}
	return value, nil
}
