//go:build !linux

package main

import (
	"fmt"
	"time"
)

func (r *runtime) queryServiceStatus(manager, unit string) (serviceStatus, error) {
	return serviceStatus{}, fmt.Errorf("systemd service status is supported only on Linux")
}

func (r *runtime) transitionService(manager, action, unit, expectedState string, timeout time.Duration) error {
	return fmt.Errorf("systemd service %s is supported only on Linux", action)
}

func (r *runtime) waitForService(manager, unit, expectedState string, timeout time.Duration) (serviceStatus, error) {
	return serviceStatus{}, fmt.Errorf("systemd service wait is supported only on Linux")
}
