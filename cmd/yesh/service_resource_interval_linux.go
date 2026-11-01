//go:build linux

package main

import (
	"fmt"
	"strconv"
	"time"
)

// serviceResourceInterval is deliberately an intermediate model. It describes
// what can be proven by comparing two finite snapshots; it is not a history
// store and does not infer a cause from the observed changes.
type serviceResourceInterval struct {
	startedAt        time.Time
	finishedAt       time.Time
	elapsed          time.Duration
	initial          map[string]uint64
	final            map[string]uint64
	deltas           map[string]uint64
	processesStarted int
	processesExited  int
	incomplete       bool
	reasons          []string
}

var serviceResourceCounterMetrics = []string{
	"cpu.stat.usage_usec",
	"cpu.stat.user_usec",
	"cpu.stat.system_usec",
	"cpu.stat.nr_periods",
	"cpu.stat.nr_throttled",
	"cpu.stat.throttled_usec",
	"memory.events.high",
	"memory.events.max",
	"memory.events.oom",
	"memory.events.oom_kill",
	"memory.events.oom_group_kill",
	"memory.events.local.high",
	"memory.events.local.max",
	"memory.events.local.oom",
	"memory.events.local.oom_kill",
	"memory.events.local.oom_group_kill",
	"memory.swap.events.high",
	"memory.swap.events.max",
	"memory.swap.events.fail",
	"pids.events.max",
}

func compareServiceResourceSnapshots(first, last serviceResourceSnapshot) (serviceResourceInterval, error) {
	if first.observedAt.IsZero() || last.observedAt.IsZero() {
		return serviceResourceInterval{}, fmt.Errorf("resource snapshots have no observation time")
	}
	if !last.observedAt.After(first.observedAt) {
		return serviceResourceInterval{}, fmt.Errorf("resource snapshots are not in chronological order")
	}
	if first.properties["Id"] != last.properties["Id"] || first.cgroup != last.cgroup {
		return serviceResourceInterval{}, fmt.Errorf("resource snapshots do not describe the same unit and cgroup")
	}
	interval := serviceResourceInterval{
		startedAt:  first.observedAt,
		finishedAt: last.observedAt,
		elapsed:    last.observedAt.Sub(first.observedAt),
		initial:    make(map[string]uint64),
		final:      make(map[string]uint64),
		deltas:     make(map[string]uint64),
	}
	if first.metrics["processes.incomplete"] == "yes" || last.metrics["processes.incomplete"] == "yes" {
		interval.incomplete = true
		interval.reasons = append(interval.reasons, "process snapshot incomplete")
	}
	if first.properties["NRestarts"] != last.properties["NRestarts"] ||
		(first.properties["InvocationID"] != "" && last.properties["InvocationID"] != "" && first.properties["InvocationID"] != last.properties["InvocationID"]) {
		interval.incomplete = true
		interval.reasons = append(interval.reasons, "unit restarted during interval")
	}
	for _, name := range serviceResourceCounterMetrics {
		before, beforeOK := parseServiceResourceUint(first.metrics[name])
		after, afterOK := parseServiceResourceUint(last.metrics[name])
		if !beforeOK || !afterOK {
			continue
		}
		if after < before {
			interval.incomplete = true
			interval.reasons = append(interval.reasons, "counter reset: "+name)
			continue
		}
		interval.initial[name], interval.final[name], interval.deltas[name] = before, after, after-before
	}
	return interval, nil
}

func parseServiceResourceUint(value string) (uint64, bool) {
	if value == "" || value == "unavailable" || value == "max" {
		return 0, false
	}
	n, err := strconv.ParseUint(value, 10, 64)
	return n, err == nil
}

func processIdentitySet(processes []processInfo) map[string]struct{} {
	result := make(map[string]struct{}, len(processes))
	for _, process := range processes {
		result[strconv.Itoa(process.pid)+":"+strconv.FormatUint(process.startTime, 10)] = struct{}{}
	}
	return result
}

func countProcessChurn(first, last []processInfo) (started, exited int) {
	before, after := processIdentitySet(first), processIdentitySet(last)
	for identity := range after {
		if _, ok := before[identity]; !ok {
			started++
		}
	}
	for identity := range before {
		if _, ok := after[identity]; !ok {
			exited++
		}
	}
	return started, exited
}
