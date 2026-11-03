//go:build !linux

package main

import "fmt"

func navigatorListServices(string) ([]navigatorService, error) {
	return nil, fmt.Errorf("service discovery is supported only on Linux")
}
