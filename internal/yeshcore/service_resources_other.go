//go:build !linux

package yeshcore

import (
	"fmt"
	"time"
)

type serviceResourceSnapshot struct {
	properties map[string]string
	platform   osReleaseInfo
	supervisor string
	epoch      serviceResourceEpoch
	cgroup     string
	observedAt time.Time
	metrics    map[string]string
	processes  []processInfo
	rssBytes   uint64
}

func (r *runtime) queryServiceResources(manager, unit string) (serviceResourceSnapshot, error) {
	return serviceResourceSnapshot{}, fmt.Errorf("service resources are supported only on Linux")
}

func (r *runtime) queryAutoServiceResources(unit string) (string, serviceResourceSnapshot, error) {
	return "", serviceResourceSnapshot{}, fmt.Errorf("service resources are supported only on Linux")
}

func (r *runtime) resolveAutoServiceManager(unit string) (string, error) {
	return "", fmt.Errorf("service resources are supported only on Linux")
}

func (r *runtime) runServiceMeasurement(manager, unit string, duration, interval, keepFor time.Duration, stateDir string) error {
	return fmt.Errorf("service measurement is supported only on Linux")
}

func (r *runtime) runServiceResourceSummary(manager, unit, stateDir string, since time.Duration) error {
	return fmt.Errorf("service resource summary is supported only on Linux")
}
