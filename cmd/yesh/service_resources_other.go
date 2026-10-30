//go:build !linux

package main

import "fmt"

type serviceResourceSnapshot struct{}

func (r *runtime) queryServiceResources(manager, unit string) (serviceResourceSnapshot, error) {
	return serviceResourceSnapshot{}, fmt.Errorf("service resources are supported only on Linux")
}
