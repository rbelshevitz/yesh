//go:build !linux

package main

import "fmt"

func navigatorListServices(string) ([]navigatorService, error) {
	return nil, fmt.Errorf("service discovery is supported only on Linux")
}

func parseNavigatorServices(string) []navigatorService {
	return nil
}

func navigatorFilteredServices(services []navigatorService, prefix string) []navigatorService {
	if prefix == "" {
		return services
	}
	return nil
}
