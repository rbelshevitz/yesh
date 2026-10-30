//go:build linux

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// processInfo is deliberately a value type. A PID alone is not an identity:
// Linux may reuse it after a process exits, so callers must retain startTime.
type processInfo struct {
	pid       int
	name      string
	state     byte
	ppid      int
	pgid      int
	sid       int
	tty       int64
	uid       uint64
	rssBytes  uint64
	startTime uint64
	cgroup    string
}

func readProcessInfo(pid int) (processInfo, error) {
	if pid <= 0 {
		return processInfo{}, fmt.Errorf("invalid PID %d", pid)
	}
	root := filepath.Join("/proc", strconv.Itoa(pid))
	stat, err := os.ReadFile(filepath.Join(root, "stat"))
	if err != nil {
		return processInfo{}, err
	}
	info, err := parseProcStat(string(stat))
	if err != nil {
		return processInfo{}, fmt.Errorf("%s/stat: %w", root, err)
	}
	info.pid = pid
	info.uid, err = readProcessUID(filepath.Join(root, "status"))
	if err != nil {
		return processInfo{}, fmt.Errorf("%s/status: %w", root, err)
	}
	info.cgroup, err = readProcessCgroup(filepath.Join(root, "cgroup"))
	if err != nil {
		return processInfo{}, fmt.Errorf("%s/cgroup: %w", root, err)
	}
	return info, nil
}

// listProcessInfo takes a bounded, best-effort snapshot. A process that exits
// between ReadDir and opening /proc/<pid> is omitted; other read failures are
// returned instead of being silently presented as an incomplete report.
func listProcessInfo(limit int) ([]processInfo, error) {
	if limit <= 0 || limit > maxProcessSnapshot {
		return nil, fmt.Errorf("process snapshot limit must be between 1 and %d", maxProcessSnapshot)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	result := make([]processInfo, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if len(result) == limit {
			break
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		info, err := readProcessInfo(pid)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}

// findProcessInfo uses an exact kernel-reported name. Regex and substring
// matching stay out of this primitive so a later signal operation cannot
// accidentally broaden its target set.
func findProcessInfo(name string, limit int) ([]processInfo, error) {
	if name == "" {
		return nil, errors.New("process name must not be empty")
	}
	if limit <= 0 || limit > maxProcessSnapshot {
		return nil, fmt.Errorf("process snapshot limit must be between 1 and %d", maxProcessSnapshot)
	}
	all, err := listProcessInfo(maxProcessSnapshot)
	if err != nil {
		return nil, err
	}
	result := make([]processInfo, 0, min(limit, len(all)))
	for _, process := range all {
		if process.name != name {
			continue
		}
		if len(result) == limit {
			break
		}
		result = append(result, process)
	}
	return result, nil
}

var processFields = map[string]bool{
	"pid": true, "uid": true, "state": true, "rss": true,
	"ppid": true, "pgid": true, "sid": true, "name": true, "cgroup": true,
	"start_time": true,
}

func writeProcessTable(out io.Writer, processes []processInfo, fields []string) error {
	if len(fields) == 0 {
		fields = []string{"pid", "uid", "state", "rss", "ppid", "pgid", "sid", "name", "start_time", "cgroup"}
	}
	for _, field := range fields {
		if !processFields[field] {
			return fmt.Errorf("unknown process field %q", field)
		}
	}
	if _, err := fmt.Fprintln(out, strings.ToUpper(strings.Join(fields, "\t"))); err != nil {
		return err
	}
	for _, process := range processes {
		values := make([]string, 0, len(fields))
		for _, field := range fields {
			switch field {
			case "pid":
				values = append(values, fmt.Sprintf("%d", process.pid))
			case "uid":
				values = append(values, fmt.Sprintf("%d", process.uid))
			case "state":
				values = append(values, string(process.state))
			case "rss":
				values = append(values, fmt.Sprintf("%d", process.rssBytes))
			case "ppid":
				values = append(values, fmt.Sprintf("%d", process.ppid))
			case "pgid":
				values = append(values, fmt.Sprintf("%d", process.pgid))
			case "sid":
				values = append(values, fmt.Sprintf("%d", process.sid))
			case "name":
				values = append(values, process.name)
			case "cgroup":
				values = append(values, process.cgroup)
			case "start_time":
				values = append(values, fmt.Sprintf("%d", process.startTime))
			}
		}
		if _, err := fmt.Fprintln(out, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func writeProcessSummary(out io.Writer, processes []processInfo, metric string) error {
	if metric != "rss" {
		return fmt.Errorf("unknown memory metric %q", metric)
	}
	total, err := processRSSTotal(processes)
	if err != nil {
		return err
	}
	var maximum uint64
	for _, process := range processes {
		if process.rssBytes > maximum {
			maximum = process.rssBytes
		}
	}
	average := uint64(0)
	if len(processes) > 0 {
		average = total / uint64(len(processes))
	}
	if _, err := fmt.Fprintf(out, "PROCESS_COUNT\t%d\nRSS_TOTAL\t%s\nRSS_MAX\t%s\nRSS_AVERAGE\t%s\n",
		len(processes), formatBytes(total), formatBytes(maximum), formatBytes(average)); err != nil {
		return err
	}
	return nil
}

func processRSSTotal(processes []processInfo) (uint64, error) {
	var total uint64
	for _, process := range processes {
		if total > ^uint64(0)-process.rssBytes {
			return 0, fmt.Errorf("RSS total overflows")
		}
		total += process.rssBytes
	}
	return total, nil
}

func formatBytes(bytes uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%dB (%d bytes)", bytes, bytes)
	}
	return fmt.Sprintf("%.2f%s (%d bytes)", value, units[unit], bytes)
}

// parseProcStat handles names containing spaces and parentheses by locating
// the final ')' before the numeric fields, rather than using strings.Fields
// on the whole line.
func parseProcStat(line string) (processInfo, error) {
	open := strings.IndexByte(line, '(')
	close := strings.LastIndexByte(line, ')')
	if open <= 0 || close <= open+1 || close+2 >= len(line) {
		return processInfo{}, errors.New("malformed stat record")
	}
	name := line[open+1 : close]
	fields := strings.Fields(line[close+2:])
	if len(fields) <= 21 {
		return processInfo{}, errors.New("short stat record")
	}
	parseInt := func(index int, label string) (int64, error) {
		value, err := strconv.ParseInt(fields[index], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", label, err)
		}
		return value, nil
	}
	state := fields[0]
	if len(state) != 1 {
		return processInfo{}, errors.New("invalid process state")
	}
	ppid, err := parseInt(1, "ppid")
	if err != nil {
		return processInfo{}, err
	}
	pgid, err := parseInt(2, "pgid")
	if err != nil {
		return processInfo{}, err
	}
	sid, err := parseInt(3, "sid")
	if err != nil {
		return processInfo{}, err
	}
	tty, err := parseInt(4, "tty")
	if err != nil {
		return processInfo{}, err
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return processInfo{}, fmt.Errorf("start time: %w", err)
	}
	rssPages, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil || rssPages < 0 {
		return processInfo{}, errors.New("invalid resident set size")
	}
	pageSize := uint64(os.Getpagesize())
	if pageSize == 0 || uint64(rssPages) > ^uint64(0)/pageSize {
		return processInfo{}, errors.New("resident set size overflows")
	}
	return processInfo{
		name:      name,
		state:     state[0],
		ppid:      int(ppid),
		pgid:      int(pgid),
		sid:       int(sid),
		tty:       tty,
		rssBytes:  uint64(rssPages) * pageSize,
		startTime: start,
	}, nil
}

func readProcessUID(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "Uid:" {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("Uid field is missing")
}

func readProcessCgroup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", errors.New("empty cgroup record")
	}
	// cgroup v2 has a single hierarchy line: 0::<path>. Keep the path only;
	// mixed v1/v2 details can be added when the public query is designed.
	parts := strings.SplitN(lines[0], ":", 3)
	if len(parts) != 3 {
		return "", errors.New("malformed cgroup record")
	}
	return parts[2], nil
}
