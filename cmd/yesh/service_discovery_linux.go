//go:build linux && navigator

package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func navigatorListServices(manager string) ([]navigatorService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := systemctlArgv(manager, "list-units", "--type=service", "--all", "--no-legend", "--no-pager")
	command := exec.CommandContext(ctx, args[0], args[1:]...)
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("could not read services: timed out")
		}
		return nil, fmt.Errorf("could not read services: %w", err)
	}
	services := parseNavigatorServices(string(output))
	if len(services) == 0 {
		return services, fmt.Errorf("no loaded services were found")
	}
	return services, nil
}
