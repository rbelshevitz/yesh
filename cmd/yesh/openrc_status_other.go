//go:build !linux

package main

import "fmt"

// Keep the OpenRC-facing shape available to the portable service code. The
// manager is Linux-specific; these definitions make that boundary explicit
// without pretending that OpenRC can run on another host.
type openRCServiceStatus struct {
	Runlevel   string
	Name       string
	State      string
	Uptime     string
	Respawns   int
	Supervised bool
}

func openRCStatusArgv() []string {
	return []string{"rc-status", "-C", "-f", "ini", "-a"}
}

func openRCActiveState(state string) string {
	switch state {
	case "started":
		return "active"
	case "starting", "stopping", "scheduled":
		return "activating"
	case "failed", "crashed":
		return "failed"
	default:
		return "inactive"
	}
}

func (r *runtime) queryOpenRCServiceStatus(unit string) (openRCServiceStatus, error) {
	return openRCServiceStatus{}, fmt.Errorf("OpenRC service status is supported only on Linux")
}
