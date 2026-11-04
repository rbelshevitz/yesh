//go:build navigator

package main

import "strings"

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
