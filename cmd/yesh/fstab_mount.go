package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Match lexically: resolving paths could activate an automount while selecting config.
func selectFstabMount(snapshot *fstabSnapshot, path string) (fstabEntry, string, string, error) {
	var entries []fstabEntry
	for _, e := range snapshot.Entries {
		if filepath.IsAbs(e.Target) && filepath.Clean(e.Target) == path {
			entries = append(entries, e)
		}
	}
	if len(entries) != 1 {
		return fstabEntry{}, "", "", fmt.Errorf("fstab has %d entries for mountpoint %q; expected exactly one", len(entries), path)
	}
	e := entries[0]
	fail := func(reason string) (fstabEntry, string, string, error) {
		return fstabEntry{}, "", "", fmt.Errorf("fstab line %d: %s", e.Line, reason)
	}
	if e.Target != path {
		return fail("target must be a clean absolute directory path")
	}
	if e.FSType == "" || e.FSType == "auto" || e.FSType == "none" || e.FSType == "swap" || e.FSType == "autofs" || strings.ContainsAny(e.FSType, ", \t\r\n\x00") {
		return fail("require mounted needs one explicit local filesystem type; this type is unsupported")
	}
	for _, name := range fstabOptionNames(e.Options) {
		switch name {
		case "bind", "rbind", "move", "remount", "loop", "offset", "sizelimit", "subvol", "subvolid", "[opaque]":
			return fail("bind, loop, subvolume or opaque mount semantics are unsupported")
		}
	}
	tag, id, ok := strings.Cut(e.Source, "=")
	if !ok || (tag != "UUID" && tag != "PARTUUID") {
		return fail("require mounted from fstab currently needs a UUID= or PARTUUID= source")
	}
	if len(id) >= 2 && (id[0] == '"' && id[len(id)-1] == '"' || id[0] == '\'' && id[len(id)-1] == '\'') {
		id = id[1 : len(id)-1]
	}
	if id == "" || strings.ContainsAny(id, "\x00\r\n\t \"'") {
		return fail("invalid UUID/PARTUUID identity")
	}
	return e, strings.ToLower(tag), id, nil
}
