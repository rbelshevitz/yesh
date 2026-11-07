package yeshcore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const serviceResourceProperties = "Id,LoadState,ActiveState,SubState,MainPID,ControlGroup,NRestarts,InvocationID"

var serviceResourceMetricKeys = []string{
	"cgroup.type", "cgroup.controllers", "processes.incomplete",
	"io.stat.available",
	"memory.current", "memory.max", "memory.swap.current", "memory.swap.max",
	"memory.stat.anon", "memory.stat.file", "memory.stat.kernel",
	"memory.pressure.some.avg10", "memory.pressure.some.avg60", "memory.pressure.some.avg300", "memory.pressure.full.avg10",
	"cpu.pressure.some.avg10", "cpu.pressure.some.avg60", "cpu.pressure.some.avg300",
	"io.pressure.some.avg10", "io.pressure.some.avg60", "io.pressure.some.avg300",
	"host.memory.pressure.some.avg10", "host.memory.pressure.some.avg60", "host.memory.pressure.some.avg300",
	"host.cpu.pressure.some.avg10", "host.cpu.pressure.some.avg60", "host.cpu.pressure.some.avg300",
	"host.io.pressure.some.avg10", "host.io.pressure.some.avg60", "host.io.pressure.some.avg300",
	"pids.current", "pids.max", "pids.events.max",
	"cpu.stat.usage_usec", "cpu.stat.user_usec", "cpu.stat.system_usec", "cpu.stat.nr_periods", "cpu.stat.nr_throttled", "cpu.stat.throttled_usec",
	"cpu.stat.local.throttled_usec",
	"cgroup.events.populated", "cgroup.events.frozen",
	"memory.events.low", "memory.events.high", "memory.events.max", "memory.events.oom", "memory.events.oom_kill", "memory.events.oom_group_kill", "memory.events.sock_throttled",
	"memory.events.local.high", "memory.events.local.max", "memory.events.local.oom", "memory.events.local.oom_kill", "memory.events.local.oom_group_kill",
	"memory.swap.events.high", "memory.swap.events.max", "memory.swap.events.fail",
}

func unavailableServiceResourceMetrics() map[string]string {
	metrics := make(map[string]string, len(serviceResourceMetricKeys))
	for _, key := range serviceResourceMetricKeys {
		metrics[key] = "unavailable"
	}
	return metrics
}

func defaultServiceResourceStateDir(unit string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for resource state: %w", err)
	}
	return filepath.Join(home, ".local", "yesh", unit+".jsonl"), nil
}

func serviceResourceObservation(snapshot serviceResourceSnapshot) (string, string) {
	if snapshot.properties["LoadState"] == "not-found" {
		return "unavailable", "unit_not_found"
	}
	if snapshot.cgroup == "" {
		return "unavailable", "no_cgroup"
	}
	if snapshot.metrics["processes.incomplete"] == "yes" {
		return "partial", "process_disappeared"
	}
	for _, value := range snapshot.metrics {
		if value == "unavailable" {
			return "partial", "metric_unavailable"
		}
	}
	return "complete", "none"
}

func serviceResourceObservationMessage(state, reason string) string {
	switch reason {
	case "unit_not_found":
		return "systemd unit was not found; resource counters were not read"
	case "no_cgroup":
		return "unit has no observable cgroup; resource counters were not read"
	case "metric_unavailable":
		return "some resource files were unavailable; unavailable values are not zero"
	case "process_disappeared":
		return "a process disappeared during collection; the process table is incomplete"
	case "none":
		return "resource snapshot is complete"
	default:
		return "resource observation is incomplete"
	}
}

func (r *runtime) executeServiceResources(s serviceResourcesStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service resources expects one non-empty unit name")
	}
	if err := validateServiceUnit(s.manager, v.scalar); err != nil {
		return err
	}
	if s.manager == "auto" {
		if r.plan {
			fmt.Fprintf(r.outputWriter(), "PLAN detect systemd or OpenRC for service resources %q\n", v.scalar)
			return nil
		}
		manager, snapshot, err := r.queryAutoServiceResources(v.scalar)
		if err != nil {
			return fmt.Errorf("service resources %q: %w", v.scalar, err)
		}
		return r.writeServiceResourceSnapshot(manager, snapshot)
	}
	argv := systemctlArgv(s.manager, "show", "--no-pager", "--property="+serviceResourceProperties, v.scalar)
	if s.manager == "openrc" {
		if r.plan {
			fmt.Fprintf(r.outputWriter(), "PLAN %q\n", openRCStatusArgv())
			fmt.Fprintf(r.outputWriter(), "PLAN cgroup resources for OpenRC service %q (deferred)\n", v.scalar)
			return nil
		}
	}
	if r.plan {
		fmt.Fprintf(r.outputWriter(), "PLAN %q\n", argv)
		fmt.Fprintf(r.outputWriter(), "PLAN cgroup resources for %q (deferred)\n", v.scalar)
		return nil
	}
	snapshot, err := r.queryServiceResources(s.manager, v.scalar)
	if err != nil {
		return fmt.Errorf("service resources %q: %w", v.scalar, err)
	}
	return r.writeServiceResourceSnapshot(s.manager, snapshot)
}

func (r *runtime) writeServiceResourceSnapshot(manager string, snapshot serviceResourceSnapshot) error {
	observation, observationReason := serviceResourceObservation(snapshot)
	fmt.Fprintf(r.outputWriter(), "RUNTIME_PLATFORM id=%s pretty_name=%q supervisor=%s\n", snapshot.platform.id, snapshot.platform.prettyName, snapshot.supervisor)
	fmt.Fprintf(r.outputWriter(), "RESOURCE_OBSERVATION: %s — %s.\n", observation, serviceResourceObservationMessage(observation, observationReason))
	fmt.Fprintf(r.outputWriter(), "SERVICE_RESOURCES observed_at=%s id=%s invocation_id=%s load_state=%s active_state=%s sub_state=%s main_pid=%s observation=%s observation_reason=%s cgroup=%q cgroup_type=%s cgroup_controllers=%q io_stat_available=%s processes_incomplete=%s n_restarts=%s process_count=%d rss_bytes=%d memory_current=%s memory_max=%s memory_swap_current=%s memory_swap_max=%s memory_anon=%s memory_file=%s memory_kernel=%s memory_pressure_some_avg10=%s memory_pressure_some_avg60=%s memory_pressure_some_avg300=%s memory_pressure_full_avg10=%s cpu_pressure_some_avg10=%s cpu_pressure_some_avg60=%s cpu_pressure_some_avg300=%s io_pressure_some_avg10=%s io_pressure_some_avg60=%s io_pressure_some_avg300=%s host_memory_pressure_some_avg10=%s host_memory_pressure_some_avg60=%s host_memory_pressure_some_avg300=%s host_cpu_pressure_some_avg10=%s host_cpu_pressure_some_avg60=%s host_cpu_pressure_some_avg300=%s host_io_pressure_some_avg10=%s host_io_pressure_some_avg60=%s host_io_pressure_some_avg300=%s pids_current=%s pids_max=%s pids_events_max=%s cpu_usage_usec=%s cpu_user_usec=%s cpu_system_usec=%s cpu_nr_periods=%s cpu_nr_throttled=%s cpu_throttled_usec=%s cgroup_populated=%s cgroup_frozen=%s memory_high=%s memory_max_events=%s oom=%s oom_kill=%s oom_group_kill=%s memory_high_local=%s memory_max_events_local=%s oom_local=%s oom_kill_local=%s oom_group_kill_local=%s memory_swap_events_high=%s memory_swap_events_max=%s memory_swap_events_fail=%s\n",
		snapshot.observedAt.Format(time.RFC3339Nano), snapshot.properties["Id"], snapshot.properties["InvocationID"], snapshot.properties["LoadState"], snapshot.properties["ActiveState"], snapshot.properties["SubState"], snapshot.properties["MainPID"], observation, observationReason, snapshot.cgroup,
		snapshot.metrics["cgroup.type"], snapshot.metrics["cgroup.controllers"], snapshot.metrics["io.stat.available"], snapshot.metrics["processes.incomplete"], snapshot.properties["NRestarts"], len(snapshot.processes), snapshot.rssBytes,
		snapshot.metrics["memory.current"], snapshot.metrics["memory.max"], snapshot.metrics["memory.swap.current"], snapshot.metrics["memory.swap.max"], snapshot.metrics["memory.stat.anon"], snapshot.metrics["memory.stat.file"], snapshot.metrics["memory.stat.kernel"], snapshot.metrics["memory.pressure.some.avg10"], snapshot.metrics["memory.pressure.some.avg60"], snapshot.metrics["memory.pressure.some.avg300"], snapshot.metrics["memory.pressure.full.avg10"], snapshot.metrics["cpu.pressure.some.avg10"], snapshot.metrics["cpu.pressure.some.avg60"], snapshot.metrics["cpu.pressure.some.avg300"], snapshot.metrics["io.pressure.some.avg10"], snapshot.metrics["io.pressure.some.avg60"], snapshot.metrics["io.pressure.some.avg300"], snapshot.metrics["host.memory.pressure.some.avg10"], snapshot.metrics["host.memory.pressure.some.avg60"], snapshot.metrics["host.memory.pressure.some.avg300"], snapshot.metrics["host.cpu.pressure.some.avg10"], snapshot.metrics["host.cpu.pressure.some.avg60"], snapshot.metrics["host.cpu.pressure.some.avg300"], snapshot.metrics["host.io.pressure.some.avg10"], snapshot.metrics["host.io.pressure.some.avg60"], snapshot.metrics["host.io.pressure.some.avg300"], snapshot.metrics["pids.current"], snapshot.metrics["pids.max"], snapshot.metrics["pids.events.max"], snapshot.metrics["cpu.stat.usage_usec"], snapshot.metrics["cpu.stat.user_usec"], snapshot.metrics["cpu.stat.system_usec"], snapshot.metrics["cpu.stat.nr_periods"], snapshot.metrics["cpu.stat.nr_throttled"], snapshot.metrics["cpu.stat.throttled_usec"], snapshot.metrics["cgroup.events.populated"], snapshot.metrics["cgroup.events.frozen"], snapshot.metrics["memory.events.high"], snapshot.metrics["memory.events.max"], snapshot.metrics["memory.events.oom"], snapshot.metrics["memory.events.oom_kill"], snapshot.metrics["memory.events.oom_group_kill"], snapshot.metrics["memory.events.local.high"], snapshot.metrics["memory.events.local.max"], snapshot.metrics["memory.events.local.oom"], snapshot.metrics["memory.events.local.oom_kill"], snapshot.metrics["memory.events.local.oom_group_kill"], snapshot.metrics["memory.swap.events.high"], snapshot.metrics["memory.swap.events.max"], snapshot.metrics["memory.swap.events.fail"])
	if len(snapshot.processes) > 0 {
		if err := writeProcessTable(r.outputWriter(), snapshot.processes, []string{"pid", "ppid", "uid", "state", "rss", "start_time", "cgroup"}); err != nil {
			return fmt.Errorf("service resources processes: %w", err)
		}
	}
	return nil
}

func (r *runtime) executeServiceMeasure(s serviceMeasureStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("measure service expects one non-empty unit name")
	}
	if err := validateServiceUnit(s.manager, v.scalar); err != nil {
		return err
	}
	if s.manager == "auto" && !r.plan {
		manager, err := r.resolveAutoServiceManager(v.scalar)
		if err != nil {
			return err
		}
		s.manager = manager
	}
	statePath := ""
	if s.statePath.text != "" {
		state, err := r.conditionValue(s.statePath, s.line)
		if err != nil {
			return err
		}
		if state.fstab != nil || state.device != nil || state.ip != nil || state.cidr != nil || state.isList || state.regex != nil || state.isProcesses || state.secret || state.scalar == "" {
			return fmt.Errorf("measure service save path expects one non-empty path")
		}
		statePath = state.scalar
	} else {
		statePath, err = defaultServiceResourceStateDir(v.scalar)
		if err != nil {
			return err
		}
	}
	if s.duration < s.interval {
		return fmt.Errorf("measure duration must be at least one sampling interval")
	}
	count := int(s.duration/s.interval) + 1
	if count > maxServiceResourceStateSamples {
		return fmt.Errorf("measure would collect %d samples; limit is %d", count, maxServiceResourceStateSamples)
	}
	if r.plan {
		fmt.Fprintf(r.outputWriter(), "PLAN measure service %s %s for %s every %s save to %q keep history for %s\n", s.manager, v.scalar, s.durationText, s.intervalText, statePath, s.keepForText)
		return nil
	}
	return r.runServiceMeasurement(s.manager, v.scalar, s.duration, s.interval, s.keepFor, statePath)
}

func (r *runtime) executeServiceSummarize(s serviceSummarizeStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("summarize service expects one non-empty unit name")
	}
	if s.manager == "openrc" {
		if err := validateServiceUnit(s.manager, v.scalar); err != nil {
			return err
		}
	} else if err := validateUnitName(v.scalar); err != nil {
		return err
	}
	statePath := ""
	if s.statePath.text != "" {
		state, err := r.conditionValue(s.statePath, s.line)
		if err != nil {
			return err
		}
		if state.fstab != nil || state.device != nil || state.ip != nil || state.cidr != nil || state.isList || state.regex != nil || state.isProcesses || state.secret || state.scalar == "" {
			return fmt.Errorf("summarize service state path expects one non-empty path")
		}
		statePath = state.scalar
	} else {
		statePath, err = defaultServiceResourceStateDir(v.scalar)
		if err != nil {
			return err
		}
	}
	if r.plan {
		fmt.Fprintf(r.outputWriter(), "PLAN summarize service %s %s from %q since %s\n", s.manager, v.scalar, statePath, s.sinceText)
		return nil
	}
	return r.runServiceResourceSummary(s.manager, v.scalar, statePath, s.since)
}
