//go:build !linux

package main

import (
	"fmt"
	"time"
)

type serviceResourceSnapshot struct {
	properties map[string]string
	cgroup     string
	observedAt time.Time
	metrics    map[string]string
	processes  []processInfo
	rssBytes   uint64
}

func (r *runtime) queryServiceResources(manager, unit string) (serviceResourceSnapshot, error) {
	return serviceResourceSnapshot{}, fmt.Errorf("service resources are supported only on Linux")
}

func (r *runtime) runServiceMeasurement(manager, unit string, duration, interval, keepFor time.Duration, stateDir string) error {
	return fmt.Errorf("service measurement is supported only on Linux")
}

func (r *runtime) runServiceResourceSummary(manager, unit, stateDir string, since time.Duration) error {
	return fmt.Errorf("service resource summary is supported only on Linux")
}
