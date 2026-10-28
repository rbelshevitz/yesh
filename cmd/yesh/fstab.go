package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	goruntime "runtime"
	"strings"
	"unicode/utf8"
)

const maxFstabBytes = 4 << 20
const maxFstabEntries = 10000
const maxFstabLine = 64 << 10

type fstabEntry struct {
	Source  string `json:"source"`
	Target  string `json:"target"`
	FSType  string `json:"fstype"`
	Options string `json:"options"`
	Freq    int    `json:"freq"`
	PassNo  int    `json:"passno"`
	Line    int    `json:"line"`
}
type fstabDiagnostic struct {
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type fstabSnapshot struct {
	Entries     []fstabEntry
	Diagnostics []fstabDiagnostic
}

func (r *runtime) evalFstab(raw string, line int) (value, error) {
	t, err := words(raw)
	if err != nil || len(t) != 2 || !storageKeyword(t[0], "fstab") || t[1].operator {
		return value{}, fmt.Errorf("%s:%d: expected: fstab path", r.filename, line)
	}
	path, err := r.storageString(t[1], line)
	if err != nil {
		return value{}, fmt.Errorf("%s:%d: fstab: %w", r.filename, line, err)
	}
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN fstab %q (deferred)\n", path)
		return value{fstab: &fstabSnapshot{}}, nil
	}
	if goruntime.GOOS != "linux" {
		return value{}, fmt.Errorf("%s:%d: fstab requires Linux and util-linux", r.filename, line)
	}
	snapshot, err := r.readFstab(path)
	if err != nil {
		return value{}, fmt.Errorf("%s:%d: fstab %q: %w", r.filename, line, path, err)
	}
	return value{fstab: snapshot}, nil
}

// Count source records without decoding fields; all actual parsing is libmount's.
func fstabRecordLines(data []byte) ([]int, error) {
	if len(data) > maxFstabBytes {
		return nil, fmt.Errorf("input exceeds %d bytes", maxFstabBytes)
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return nil, fmt.Errorf("input must be UTF-8 without NUL")
	}
	var records []int
	for i, start := 0, 0; start < len(data); i++ {
		rest := data[start:]
		end := bytes.IndexByte(rest, '\n')
		if end < 0 {
			end = len(rest)
		}
		line := rest[:end]
		start += end
		if start < len(data) {
			start++
		}
		if len(line) > maxFstabLine {
			return nil, fmt.Errorf("line %d exceeds %d bytes", i+1, maxFstabLine)
		}
		text := bytes.Trim(line, " \t\r")
		if len(text) == 0 || text[0] == '#' {
			continue
		}
		records = append(records, i+1)
		if len(records) > maxFstabEntries {
			return nil, fmt.Errorf("more than %d entries", maxFstabEntries)
		}
	}
	return records, nil
}
func (r *runtime) readFstab(path string) (*fstabSnapshot, error) {
	if r.ctx != nil {
		if err := r.ctx.Err(); err != nil {
			return nil, err
		}
	}
	f, err := openRedirect(path, false, false)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("fstab requires a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFstabBytes+1))
	if err != nil {
		return nil, err
	}
	lines, err := fstabRecordLines(data)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return &fstabSnapshot{Entries: []fstabEntry{}, Diagnostics: []fstabDiagnostic{}}, nil
	}
	// libmount reads a private immutable snapshot, not a mutable original pathname.
	snapshot, err := os.CreateTemp("", "yesh-fstab-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(snapshot.Name())
	if _, err := snapshot.Write(data); err != nil {
		snapshot.Close()
		return nil, err
	}
	if err := snapshot.Close(); err != nil {
		return nil, err
	}
	output, err := r.storageQueryPolicy(true, "findmnt", "--fstab", "--tab-file", snapshot.Name(), "--json", "--list", "--nocanonicalize", "--output", "SOURCE,TARGET,FSTYPE,OPTIONS,FREQ,PASSNO")
	if err != nil {
		return nil, err
	}
	return decodeFstab(output, lines)
}
func decodeFstab(data []byte, lines []int) (*fstabSnapshot, error) {
	var report struct {
		Entries []fstabEntry `json:"filesystems"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("invalid findmnt fstab JSON: %w", err)
	}
	if len(report.Entries) != len(lines) {
		return nil, fmt.Errorf("libmount returned %d of %d source records; refusing partial table", len(report.Entries), len(lines))
	}
	result := &fstabSnapshot{Entries: report.Entries, Diagnostics: []fstabDiagnostic{}}
	targets := map[string]int{}
	for i := range result.Entries {
		e := &result.Entries[i]
		e.Line = lines[i]
		if e.Source == "" || e.Target == "" || e.FSType == "" {
			return nil, fmt.Errorf("incomplete libmount record at line %d", e.Line)
		}
		if e.Freq < 0 || e.PassNo < 0 {
			result.Diagnostics = append(result.Diagnostics, fstabDiagnostic{e.Line, "negative-check-field", "negative dump/fsck field requires review"})
		}
		if e.FSType == "swap" {
			continue
		}
		if previous, exists := targets[e.Target]; exists {
			result.Diagnostics = append(result.Diagnostics, fstabDiagnostic{e.Line, "duplicate-target", fmt.Sprintf("same target as line %d; review record order and intent", previous)})
		} else {
			targets[e.Target] = e.Line
		}
	}
	return result, nil
}

// Never print option values: credentials may occur under filesystem-specific names.
func fstabOptionNames(options string) []string {
	result := []string{}
	start := 0
	quoted := false
	escaped := false
	add := func(field string) {
		name, _, _ := strings.Cut(field, "=")
		if name == "" {
			return
		}
		for _, c := range name {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_.-", c)) {
				name = "[opaque]"
				break
			}
		}
		result = append(result, name)
	}
	for i, c := range options {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			quoted = !quoted
			continue
		}
		if c == ',' && !quoted {
			add(options[start:i])
			start = i + 1
		}
	}
	add(options[start:])
	return result
}
func writeFstabInspection(w io.Writer, s *fstabSnapshot) error {
	type visible struct {
		Line        int      `json:"line"`
		Source      string   `json:"source"`
		Target      string   `json:"target"`
		FSType      string   `json:"fstype"`
		OptionNames []string `json:"option_names"`
		Freq        int      `json:"freq"`
		PassNo      int      `json:"passno"`
	}
	entries := []visible{}
	for _, e := range s.Entries {
		source := "[redacted non-local source]"
		if strings.HasPrefix(e.Source, "/") || strings.HasPrefix(e.Source, "UUID=") || strings.HasPrefix(e.Source, "PARTUUID=") || strings.HasPrefix(e.Source, "LABEL=") || strings.HasPrefix(e.Source, "PARTLABEL=") || e.Source == "none" || e.Source == "tmpfs" || e.Source == "proc" {
			source = e.Source
		}
		// UNC sources can embed user/password before @; do not expose them.
		if strings.HasPrefix(e.Source, "//") {
			source = "[redacted non-local source]"
		}
		entries = append(entries, visible{e.Line, source, e.Target, e.FSType, fstabOptionNames(e.Options), e.Freq, e.PassNo})
	}
	return json.NewEncoder(w).Encode(struct {
		Entries     []visible         `json:"entries"`
		Diagnostics []fstabDiagnostic `json:"diagnostics"`
	}{entries, s.Diagnostics})
}
