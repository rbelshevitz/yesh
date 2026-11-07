//go:build linux

package yeshcore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// openRCServiceStatus is the small, manager-specific part of an OpenRC
// observation. Process and cgroup evidence is collected separately.
type openRCServiceStatus struct {
	Runlevel   string
	Name       string
	State      string
	Uptime     string
	Respawns   int
	Supervised bool
}

type openRCServiceDaemon struct {
	Exec    string
	Argv    []string
	PIDFile string
}

func parseOpenRCServiceDaemon(data []byte) (openRCServiceDaemon, error) {
	values := make(map[string]string)
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" {
			return openRCServiceDaemon{}, fmt.Errorf("OpenRC daemon line %d is not a key=value assignment", lineNumber+1)
		}
		if _, exists := values[key]; exists {
			return openRCServiceDaemon{}, fmt.Errorf("duplicate OpenRC daemon field %q", key)
		}
		values[key] = value
	}
	result := openRCServiceDaemon{Exec: values["exec"], PIDFile: values["pidfile"]}
	for index := 0; ; index++ {
		key := "argv_" + strconv.Itoa(index)
		value, ok := values[key]
		if !ok {
			break
		}
		result.Argv = append(result.Argv, value)
	}
	if result.Exec == "" {
		return openRCServiceDaemon{}, errors.New("OpenRC daemon record omitted exec")
	}
	return result, nil
}

// OpenRC keeps volatile service values separately from the human-oriented
// status listing.  In particular, supervise-daemon records child_pid,
// start_time and start_count.  Keep this parser deliberately generic: the
// caller must still verify the PID through /proc and cgroup before treating it
// as evidence about the service.
func parseOpenRCServiceValues(data []byte) (map[string]string, error) {
	values := make(map[string]string)
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("OpenRC service value line %d is not a non-empty key=value assignment", lineNumber+1)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate OpenRC service value %q", key)
		}
		values[key] = value
	}
	return values, nil
}

func openRCServicePID(values map[string]string) (int, error) {
	text := strings.TrimSpace(values["child_pid"])
	if text == "" {
		return 0, nil
	}
	pid, err := strconv.Atoi(text)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid OpenRC child_pid %q", text)
	}
	return pid, nil
}

func (r *runtime) queryOpenRCServiceResources(unit string) (serviceResourceSnapshot, error) {
	status, err := r.queryOpenRCServiceStatus(unit)
	if err != nil {
		return serviceResourceSnapshot{}, err
	}
	daemon, err := readOpenRCServiceDaemon(unit)
	if err != nil {
		return serviceResourceSnapshot{}, err
	}
	values := r.readOpenRCServiceValues(unit)
	pid, err := openRCServicePID(values)
	if err != nil {
		return serviceResourceSnapshot{}, err
	}
	if pid == 0 && daemon.PIDFile != "" {
		pid, err = readOpenRCPIDFile(daemon.PIDFile)
		if err != nil {
			return serviceResourceSnapshot{}, err
		}
	}

	platform, err := readOSRelease(os.ReadFile)
	if err != nil {
		platform = osReleaseInfo{}
	}
	properties := map[string]string{
		"Id":           unit,
		"LoadState":    "loaded",
		"ActiveState":  openRCActiveState(status.State),
		"SubState":     status.State,
		"MainPID":      strconv.Itoa(pid),
		"ControlGroup": "",
		"NRestarts":    values["start_count"],
		"InvocationID": "",
	}
	snapshot := serviceResourceSnapshot{
		properties: properties, platform: platform, supervisor: "openrc",
		observedAt: time.Now().UTC(), metrics: unavailableServiceResourceMetrics(),
	}
	if pid == 0 {
		return snapshot, nil
	}
	snapshot.epoch = serviceResourceEpoch{PID: pid, SupervisorStartTime: values["start_time"]}
	supervisorProcess, _ := findOpenRCSupervisor(unit)
	process, err := readProcessInfo(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			snapshot.metrics["processes.incomplete"] = "yes"
			if supervisorProcess == nil {
				return snapshot, nil
			}
			snapshot.cgroup = supervisorProcess.cgroup
			snapshot.properties["ControlGroup"] = snapshot.cgroup
			return r.collectOpenRCServiceCgroup(snapshot)
		}
		return serviceResourceSnapshot{}, fmt.Errorf("read OpenRC service PID %d: %w", pid, err)
	}
	snapshot.epoch = serviceResourceEpoch{PID: pid, ProcessStartTime: process.startTime, SupervisorStartTime: values["start_time"]}
	snapshot.cgroup = process.cgroup
	if snapshot.cgroup == "" && supervisorProcess != nil {
		snapshot.cgroup = supervisorProcess.cgroup
	}
	snapshot.properties["ControlGroup"] = snapshot.cgroup
	if snapshot.cgroup == "" {
		return snapshot, nil
	}
	return r.collectOpenRCServiceCgroup(snapshot)
}

func (r *runtime) collectOpenRCServiceCgroup(snapshot serviceResourceSnapshot) (serviceResourceSnapshot, error) {
	root := r.cgroupRoot
	if root == "" {
		root = "/sys/fs/cgroup"
	}
	processes, metrics, rss, err := collectCgroupResources(root, snapshot.cgroup)
	if err != nil {
		return serviceResourceSnapshot{}, err
	}
	snapshot.processes, snapshot.metrics, snapshot.rssBytes = processes, metrics, rss
	return snapshot, nil
}

func findOpenRCSupervisor(unit string) (*processInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if len(args) < 2 || filepath.Base(args[0]) != "supervise-daemon" || args[1] != unit {
			continue
		}
		process, err := readProcessInfo(pid)
		if err == nil {
			return &process, nil
		}
	}
	return nil, nil
}

func openRCActiveState(state string) string {
	switch state {
	case "started":
		return "active"
	case "starting", "stopping", "scheduled":
		return "activating"
	case "failed", "crashed":
		return "failed"
	default:
		return "inactive"
	}
}

func (r *runtime) queryOpenRCServiceStatus(unit string) (openRCServiceStatus, error) {
	data, err := runOpenRCCommand(r.context(), openRCStatusArgv())
	if err != nil {
		return openRCServiceStatus{}, err
	}
	statuses, err := parseOpenRCStatusINI(data)
	if err != nil {
		return openRCServiceStatus{}, err
	}
	return selectOpenRCServiceStatus(statuses, unit)
}

func openRCStatusArgv() []string {
	return []string{"rc-status", "-C", "-f", "ini", "-a"}
}

func selectOpenRCServiceStatus(statuses []openRCServiceStatus, unit string) (openRCServiceStatus, error) {
	var found *openRCServiceStatus
	var runlevels []string
	for _, status := range statuses {
		if status.Name != unit {
			continue
		}
		if found == nil {
			copy := status
			found = &copy
		} else if found.State != status.State || found.Uptime != status.Uptime || found.Respawns != status.Respawns || found.Supervised != status.Supervised {
			return openRCServiceStatus{}, fmt.Errorf("OpenRC service %q has conflicting state in multiple runlevels", unit)
		}
		runlevels = append(runlevels, status.Runlevel)
	}
	if found == nil {
		return openRCServiceStatus{}, fmt.Errorf("OpenRC service %q was not found in rc-status output", unit)
	}
	found.Runlevel = strings.Join(runlevels, ",")
	return *found, nil
}

func readOpenRCServiceDaemon(unit string) (openRCServiceDaemon, error) {
	path := filepath.Join("/run/openrc/daemons", unit, "001")
	data, err := os.ReadFile(path)
	if err != nil {
		return openRCServiceDaemon{}, fmt.Errorf("read OpenRC daemon record %q: %w", path, err)
	}
	return parseOpenRCServiceDaemon(data)
}

func (r *runtime) readOpenRCServiceValues(unit string) map[string]string {
	result := make(map[string]string)
	path := "/usr/libexec/rc/bin/service_get_value"
	if _, err := os.Stat(path); err != nil {
		return result
	}
	for _, key := range []string{"child_pid", "start_time", "start_count"} {
		data, err := runOpenRCCommandWithEnv(r.context(), []string{"RC_SVCNAME=" + unit}, []string{path, key})
		if err == nil {
			if value := strings.TrimSpace(string(data)); value != "" {
				result[key] = value
			}
		}
	}
	return result
}

func readOpenRCPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read OpenRC PID file %q: %w", path, err)
	}
	fields := strings.Fields(string(data))
	if len(fields) != 1 {
		return 0, fmt.Errorf("OpenRC PID file %q does not contain one PID", path)
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("OpenRC PID file %q contains invalid PID %q", path, fields[0])
	}
	return pid, nil
}

func runOpenRCCommand(ctx context.Context, argv []string) ([]byte, error) {
	return runOpenRCCommandWithEnv(ctx, nil, argv)
}

func runOpenRCCommandWithEnv(ctx context.Context, env []string, argv []string) ([]byte, error) {
	if len(argv) == 0 {
		return nil, errors.New("empty OpenRC command")
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Env = append(os.Environ(), env...)
	var out, diagnostic storageOutput
	command.Stdout = &out
	command.Stderr = &diagnostic
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(diagnostic.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s failed: %s", argv[0], message)
	}
	if out.exceeded || diagnostic.exceeded {
		return nil, fmt.Errorf("%s output exceeds %d bytes", argv[0], maxStorageOutput)
	}
	return out.Bytes(), nil
}

var openRCServiceStates = map[string]bool{
	"stopped":    true,
	"started":    true,
	"stopping":   true,
	"starting":   true,
	"inactive":   true,
	"hotplugged": true,
	"failed":     true,
	"scheduled":  true,
	"crashed":    true,
}

func parseOpenRCStatusINI(data []byte) ([]openRCServiceStatus, error) {
	var result []openRCServiceStatus
	section := ""
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if section == "" {
				return nil, fmt.Errorf("rc-status line %d has an empty section", lineNumber+1)
			}
			continue
		}
		if section == "" {
			return nil, fmt.Errorf("rc-status line %d appears before a section", lineNumber+1)
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("rc-status line %d is not an assignment", lineNumber+1)
		}
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if name == "" || value == "" {
			return nil, fmt.Errorf("rc-status line %d has an empty service or state", lineNumber+1)
		}
		fields := strings.Fields(value)
		if len(fields) == 0 || !openRCServiceStates[fields[0]] {
			return nil, fmt.Errorf("rc-status line %d has unknown state %q", lineNumber+1, fields[0])
		}
		status := openRCServiceStatus{Runlevel: section, Name: name, State: fields[0]}
		if len(fields) > 1 {
			status.Uptime = fields[1]
		}
		if len(fields) > 2 {
			last := fields[len(fields)-1]
			if !strings.HasPrefix(last, "(") || !strings.HasSuffix(last, ")") {
				return nil, fmt.Errorf("rc-status line %d has invalid supervision count %q", lineNumber+1, last)
			}
			count, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(last, "("), ")"))
			if err != nil || count < 0 {
				return nil, fmt.Errorf("rc-status line %d has invalid supervision count %q", lineNumber+1, last)
			}
			status.Respawns, status.Supervised = count, true
		}
		result = append(result, status)
	}
	return result, nil
}
