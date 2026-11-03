//go:build linux

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
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

func parseNavigatorServices(output string) []navigatorService {
	services := []navigatorService{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}
		services = append(services, navigatorService{unit: fields[0], active: fields[2], sub: fields[3], description: description})
	}
	return services
}

func navigatorFilteredServices(services []navigatorService, prefix string) []navigatorService {
	if prefix == "" {
		return services
	}
	filtered := make([]navigatorService, 0, len(services))
	for _, service := range services {
		if strings.HasPrefix(service.unit, prefix) {
			filtered = append(filtered, service)
		}
	}
	return filtered
}
