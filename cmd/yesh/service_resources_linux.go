//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type serviceResourceSnapshot struct {
	properties map[string]string
	cgroup     string
	metrics    map[string]string
	processes  []processInfo
	rssBytes   uint64
}

func (r *runtime) queryServiceResources(manager, unit string) (serviceResourceSnapshot, error) {
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	argv := systemctlArgv(manager, "show", "--no-pager", "--property="+serviceResourceProperties, unit)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, diagnostic storageOutput
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return serviceResourceSnapshot{}, ctx.Err()
		}
		message := strings.TrimSpace(diagnostic.String())
		if message == "" {
			message = err.Error()
		}
		return serviceResourceSnapshot{}, fmt.Errorf("systemctl failed: %s", message)
	}
	if out.exceeded || diagnostic.exceeded {
		return serviceResourceSnapshot{}, fmt.Errorf("systemctl output exceeds %d bytes", maxStorageOutput)
	}
	properties, err := parseServiceResourceProperties(out.Bytes())
	if err != nil {
		return serviceResourceSnapshot{}, err
	}
	if properties["Id"] != unit {
		return serviceResourceSnapshot{}, fmt.Errorf("systemctl returned canonical unit %q; aliases are not supported", properties["Id"])
	}
	snapshot := serviceResourceSnapshot{properties: properties, cgroup: properties["ControlGroup"]}
	if snapshot.cgroup == "" {
		return snapshot, nil
	}
	processes, metrics, rss, err := readServiceCgroup("/sys/fs/cgroup", snapshot.cgroup)
	if err != nil {
		return serviceResourceSnapshot{}, err
	}
	snapshot.processes, snapshot.metrics, snapshot.rssBytes = processes, metrics, rss
	return snapshot, nil
}

func parseServiceResourceProperties(data []byte) (map[string]string, error) {
	want := map[string]bool{}
	for _, key := range strings.Split(serviceResourceProperties, ",") {
		want[key] = true
	}
	result := make(map[string]string, len(want))
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !want[key] {
			return nil, fmt.Errorf("unexpected systemctl show field %q", line)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate systemctl show field %q", key)
		}
		result[key] = value
	}
	for _, key := range []string{"Id", "LoadState", "ActiveState", "SubState", "ControlGroup"} {
		if _, ok := result[key]; !ok {
			return nil, fmt.Errorf("systemctl show omitted required field %q", key)
		}
	}
	return result, nil
}

func readServiceCgroup(root, controlGroup string) ([]processInfo, map[string]string, uint64, error) {
	if root == "" || !filepath.IsAbs(root) || controlGroup == "" || !strings.HasPrefix(controlGroup, "/") || strings.ContainsRune(controlGroup, '\x00') {
		return nil, nil, 0, errors.New("invalid systemd ControlGroup path")
	}
	path := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(controlGroup, "/")))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, nil, 0, errors.New("systemd ControlGroup escapes cgroup root")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("cgroup path is not a directory")
		}
		return nil, nil, 0, fmt.Errorf("read cgroup %q: %w", controlGroup, err)
	}

	pids := make(map[int]bool)
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() != "cgroup.procs" {
			return nil
		}
		data, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		for _, field := range strings.Fields(string(data)) {
			pid, err := strconv.Atoi(field)
			if err != nil || pid <= 0 {
				return fmt.Errorf("invalid PID %q in %s", field, current)
			}
			pids[pid] = true
			if len(pids) > maxProcessSnapshot {
				return fmt.Errorf("service cgroup exceeds %d processes", maxProcessSnapshot)
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, 0, err
	}
	processes := make([]processInfo, 0, len(pids))
	var rss uint64
	for pid := range pids {
		process, err := readProcessInfo(pid)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, nil, 0, fmt.Errorf("read process %d: %w", pid, err)
		}
		processes = append(processes, process)
		if ^uint64(0)-rss < process.rssBytes {
			return nil, nil, 0, errors.New("process RSS total overflow")
		}
		rss += process.rssBytes
	}
	sort.Slice(processes, func(i, j int) bool { return processes[i].pid < processes[j].pid })
	metrics := map[string]string{}
	for _, name := range []string{"memory.current", "memory.max", "pids.current", "pids.max"} {
		metrics[name] = readCgroupMetric(path, name)
	}
	metrics["cpu.usage_usec"] = readCgroupMetric(path, "cpu.stat", "usage_usec")
	metrics["memory.events.oom"] = readCgroupMetric(path, "memory.events", "oom")
	metrics["memory.events.oom_kill"] = readCgroupMetric(path, "memory.events", "oom_kill")
	return processes, metrics, rss, nil
}

func readCgroupMetric(path string, names ...string) string {
	data, err := os.ReadFile(filepath.Join(append([]string{path}, names[0])...))
	if err != nil {
		return "unavailable"
	}
	if len(names) == 1 {
		return strings.TrimSpace(string(data))
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == names[1] {
			return fields[1]
		}
	}
	return "unavailable"
}
