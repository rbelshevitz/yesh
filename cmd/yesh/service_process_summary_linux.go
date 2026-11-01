//go:build linux

package main

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

type serviceProcessGrowth struct {
	PID       int
	StartTime uint64
	Name      string
	FirstRSS  uint64
	LastRSS   uint64
	Delta     int64
}

type serviceProcessSummary struct {
	TopDelta    serviceProcessGrowth
	HasTop      bool
	Peak        serviceProcessGrowth
	HasPeak     bool
	Started     int
	Exited      int
	Comparisons int
	Incomplete  bool
	Available   bool
}

type serviceProcessSampleRef struct {
	InvocationID string
	Cgroup       string
	RecordID     time.Time
	Sample       serviceResourceStateSample
}

func summarizeServiceProcessSamples(result *serviceProcessSummary, states []serviceResourceState) {
	refs := make([]serviceProcessSampleRef, 0)
	for _, state := range states {
		for _, sample := range state.Samples {
			refs = append(refs, serviceProcessSampleRef{InvocationID: state.InvocationID, Cgroup: state.Cgroup, RecordID: state.StartedAt, Sample: sample})
		}
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Sample.ObservedAt.Before(refs[j].Sample.ObservedAt) })
	var previous *serviceProcessSampleRef
	for index := range refs {
		current := &refs[index]
		if current.Sample.ProcessesIncomplete {
			result.Incomplete = true
			previous = nil
			continue
		}
		if previous == nil || previous.InvocationID != current.InvocationID || previous.Cgroup != current.Cgroup || (current.InvocationID == "" && !previous.RecordID.Equal(current.RecordID)) {
			previous = current
			continue
		}
		result.Comparisons++
		compareServiceProcessSamples(result, previous.Sample.Processes, current.Sample.Processes)
		previous = current
	}
}

func compareServiceProcessSamples(result *serviceProcessSummary, before, after []serviceResourceProcessIdentity) {
	firstByID := serviceProcessMap(before)
	lastByID := serviceProcessMap(after)
	for _, process := range before {
		updateServiceProcessPeak(result, process)
	}
	for id, process := range lastByID {
		if _, ok := firstByID[id]; !ok {
			result.Started++
		}
		updateServiceProcessPeak(result, process)
	}
	for id := range firstByID {
		if _, ok := lastByID[id]; !ok {
			result.Exited++
		}
	}
	for id, beforeProcess := range firstByID {
		afterProcess, ok := lastByID[id]
		if !ok || beforeProcess.Name == "" || afterProcess.Name == "" {
			continue
		}
		growth := serviceProcessGrowth{PID: afterProcess.PID, StartTime: afterProcess.StartTime, Name: afterProcess.Name, FirstRSS: beforeProcess.RSSBytes, LastRSS: afterProcess.RSSBytes}
		if afterProcess.RSSBytes >= beforeProcess.RSSBytes {
			if afterProcess.RSSBytes-beforeProcess.RSSBytes > uint64(^uint64(0)>>1) {
				continue
			}
			growth.Delta = int64(afterProcess.RSSBytes - beforeProcess.RSSBytes)
		} else {
			if beforeProcess.RSSBytes-afterProcess.RSSBytes > uint64(^uint64(0)>>1) {
				continue
			}
			growth.Delta = -int64(beforeProcess.RSSBytes - afterProcess.RSSBytes)
		}
		result.Available = true
		if !result.HasTop || growth.Delta > result.TopDelta.Delta {
			result.TopDelta, result.HasTop = growth, true
		}
	}
}

func updateServiceProcessPeak(result *serviceProcessSummary, process serviceResourceProcessIdentity) {
	if process.Name == "" {
		return
	}
	result.Available = true
	peak := serviceProcessGrowth{PID: process.PID, StartTime: process.StartTime, Name: process.Name, LastRSS: process.RSSBytes}
	if !result.HasPeak || peak.LastRSS > result.Peak.LastRSS {
		result.Peak, result.HasPeak = peak, true
	}
}

func serviceProcessMap(processes []serviceResourceProcessIdentity) map[string]serviceResourceProcessIdentity {
	result := make(map[string]serviceResourceProcessIdentity, len(processes))
	for _, process := range processes {
		key := strconv.Itoa(process.PID) + ":" + strconv.FormatUint(process.StartTime, 10)
		result[key] = process
	}
	return result
}

func formatServiceBytes(value uint64) string {
	const unit = 1024
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	amount := float64(value)
	index := 0
	for amount >= unit && index < len(units)-1 {
		amount /= unit
		index++
	}
	if amount >= 100 || amount == float64(int64(amount)) {
		return fmt.Sprintf("%.0f %s", amount, units[index])
	}
	return fmt.Sprintf("%.1f %s", amount, units[index])
}

func formatServiceSignedBytes(value int64) string {
	if value >= 0 {
		return "+" + formatServiceBytes(uint64(value))
	}
	return "-" + formatServiceBytes(uint64(-(value + 1)))
}

func writeServiceProcessSummary(out interface{ Write([]byte) (int, error) }, unit string, summary serviceResourceSummary) {
	if !summary.processAvailable {
		if summary.processIncomplete {
			fmt.Fprintf(out, "Process RSS for %s: incomplete; a process disappeared or a process list was incomplete.\n", unit)
			return
		}
		fmt.Fprintf(out, "Process RSS for %s: not available in the saved measurements.\n", unit)
		return
	}
	if summary.processHasTop {
		fmt.Fprintf(out, "Process RSS for %s: %s (PID %d) changed by %s; peak %s; %d appeared and %d exited across %d adjacent comparisons.", unit, summary.processTop.Name, summary.processTop.PID, formatServiceSignedBytes(summary.processTop.Delta), formatServiceBytes(summary.processPeak.LastRSS), summary.processStarted, summary.processExited, summary.processComparisons)
	} else {
		fmt.Fprintf(out, "Process RSS for %s: no change was attributed to a continuing process; %d appeared and %d exited across %d adjacent comparisons.", unit, summary.processStarted, summary.processExited, summary.processComparisons)
	}
	if summary.processIncomplete {
		fmt.Fprint(out, " Some process comparisons were unavailable because a process list was incomplete.")
	}
	fmt.Fprintln(out)
}
