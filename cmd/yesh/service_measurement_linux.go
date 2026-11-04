//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

func (r *runtime) runServiceMeasurement(manager, unit string, duration, interval, keepFor time.Duration, stateDir string) error {
	journalPath := serviceResourceJournalPath(stateDir)
	stateDirectory := serviceResourceStateDirectory(stateDir)
	if err := os.MkdirAll(stateDirectory, 0700); err != nil {
		return fmt.Errorf("create resource state directory: %w", err)
	}
	first, err := r.queryServiceResources(manager, unit)
	if err != nil {
		return err
	}
	if first.cgroup == "" {
		return fmt.Errorf("service %q has no observable cgroup", unit)
	}
	started := time.Now()
	deadline := started.Add(duration)
	samples := []serviceResourceStateSample{serviceResourceStateSampleFromSnapshot(first)}
	complete := true
	reasons := []string{}
	appendServiceMeasurementCompleteness(&complete, &reasons, first)
	for index := 1; ; index++ {
		target := started.Add(time.Duration(index) * interval)
		if target.After(deadline) {
			target = deadline
		}
		if wait := time.Until(target); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-r.context().Done():
				timer.Stop()
				return r.context().Err()
			}
		}
		next, err := r.queryServiceResources(manager, unit)
		if err != nil {
			return err
		}
		samples = append(samples, serviceResourceStateSampleFromSnapshot(next))
		appendServiceMeasurementCompleteness(&complete, &reasons, next)
		if next.cgroup != first.cgroup || next.properties["InvocationID"] != first.properties["InvocationID"] || !sameServiceResourceEpoch(next.epoch, first.epoch) {
			complete = false
			appendServiceMeasurementReason(&reasons, "identity_changed")
			break
		}
		if !target.Before(deadline) {
			break
		}
	}
	finished := time.Now().UTC()
	state := serviceResourceState{
		Schema: serviceResourceStateSchema, Kind: "service-resource-measurement", Manager: manager,
		Unit: unit, PlatformID: first.platform.id, Supervisor: first.supervisor,
		Cgroup: first.cgroup, InvocationID: first.properties["InvocationID"], Epoch: first.epoch,
		StartedAt: first.observedAt, FinishedAt: finished, Samples: samples,
		Complete: complete, IncompleteReasons: reasons,
	}
	if err := state.validate(); err != nil {
		return fmt.Errorf("resource measurement: %w", err)
	}
	path, err := writeServiceResourceState(r, journalPath, state)
	if err != nil {
		return err
	}
	removed := 0
	if keepFor > 0 {
		removed, err = pruneServiceResourceHistory(r, journalPath, manager, unit, keepFor)
		if err != nil {
			return err
		}
	}
	keepText := "none"
	if keepFor > 0 {
		keepText = keepFor.String()
	}
	fmt.Fprintf(r.outputWriter(), "SERVICE_MEASURE unit=%s state=%q samples=%d complete=%t history_keep=%s history_removed=%d\n", unit, path, len(samples), complete, keepText, removed)
	return nil
}

func appendServiceMeasurementCompleteness(complete *bool, reasons *[]string, snapshot serviceResourceSnapshot) {
	if snapshot.metrics["processes.incomplete"] == "yes" {
		*complete = false
		appendServiceMeasurementReason(reasons, "process_disappeared")
	}
}

func appendServiceMeasurementReason(reasons *[]string, reason string) {
	if reason == "" || reason == "none" {
		return
	}
	for _, existing := range *reasons {
		if existing == reason {
			return
		}
	}
	*reasons = append(*reasons, reason)
}

func (r *runtime) context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

func serviceResourceStateSampleFromSnapshot(snapshot serviceResourceSnapshot) serviceResourceStateSample {
	identities := make([]serviceResourceProcessIdentity, 0, len(snapshot.processes))
	for _, process := range snapshot.processes {
		identities = append(identities, serviceResourceProcessIdentity{
			PID: process.pid, StartTime: process.startTime, Name: process.name,
			RSSBytes: process.rssBytes,
		})
	}
	return serviceResourceStateSample{ObservedAt: snapshot.observedAt, Metrics: snapshot.metrics, ProcessCount: len(snapshot.processes), RSSBytes: snapshot.rssBytes, ProcessesIncomplete: snapshot.metrics["processes.incomplete"] == "yes", Processes: identities}
}
