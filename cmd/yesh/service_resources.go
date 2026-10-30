package main

import (
	"fmt"
	"os"
)

const serviceResourceProperties = "Id,LoadState,ActiveState,SubState,MainPID,ControlGroup,NRestarts"

func (r *runtime) executeServiceResources(s serviceResourcesStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service resources expects one non-empty unit name")
	}
	if err := validateUnitName(v.scalar); err != nil {
		return err
	}
	argv := systemctlArgv(s.manager, "show", "--no-pager", "--property="+serviceResourceProperties, v.scalar)
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN %q\n", argv)
		fmt.Fprintf(os.Stdout, "PLAN cgroup resources for %q (deferred)\n", v.scalar)
		return nil
	}
	snapshot, err := r.queryServiceResources(s.manager, v.scalar)
	if err != nil {
		return fmt.Errorf("service resources %q: %w", v.scalar, err)
	}
	fmt.Fprintf(os.Stdout, "SERVICE_RESOURCES id=%s load_state=%s active_state=%s sub_state=%s main_pid=%s cgroup=%q n_restarts=%s process_count=%d rss_bytes=%d memory_current=%s memory_max=%s pids_current=%s pids_max=%s cpu_usage_usec=%s oom=%s oom_kill=%s\n",
		snapshot.properties["Id"], snapshot.properties["LoadState"], snapshot.properties["ActiveState"], snapshot.properties["SubState"], snapshot.properties["MainPID"], snapshot.cgroup, snapshot.properties["NRestarts"], len(snapshot.processes), snapshot.rssBytes,
		snapshot.metrics["memory.current"], snapshot.metrics["memory.max"], snapshot.metrics["pids.current"], snapshot.metrics["pids.max"], snapshot.metrics["cpu.usage_usec"], snapshot.metrics["memory.events.oom"], snapshot.metrics["memory.events.oom_kill"])
	if len(snapshot.processes) > 0 {
		if err := writeProcessTable(os.Stdout, snapshot.processes, []string{"pid", "ppid", "uid", "state", "rss", "start_time", "cgroup"}); err != nil {
			return fmt.Errorf("service resources processes: %w", err)
		}
	}
	return nil
}
