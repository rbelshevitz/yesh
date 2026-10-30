//go:build !linux

package main

import "fmt"

type serviceResourceSnapshot struct {
	properties map[string]string
	cgroup     string
	metrics    map[string]string
	processes  []processInfo
	rssBytes   uint64
}

func (r *runtime) queryServiceResources(manager, unit string) (serviceResourceSnapshot, error) {
	return serviceResourceSnapshot{}, fmt.Errorf("service resources are supported only on Linux")
}
