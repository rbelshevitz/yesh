//go:build !linux

package main

import "fmt"

func navigatorFindMeasurementHistory(string, string) (navigatorMeasurementHistory, error) {
	return navigatorMeasurementHistory{}, fmt.Errorf("measurement history is supported only on Linux")
}

func navigatorMeasurementHistoryText(navigatorMeasurementHistory) string { return "" }
