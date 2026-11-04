//go:build navigator

package main

import (
	"fmt"
	"strings"
)

type navigatorBackupRecipe struct {
	sourceKind  int
	source      string
	destination string
	lockPath    string
	outputPath  string
}

func (r navigatorBackupRecipe) validate() error {
	if strings.TrimSpace(r.source) == "" {
		return fmt.Errorf("source is empty")
	}
	if strings.TrimSpace(r.destination) == "" {
		return fmt.Errorf("destination is empty")
	}
	if strings.TrimSpace(r.lockPath) == "" {
		return fmt.Errorf("lock path is empty")
	}
	if strings.TrimSpace(r.outputPath) == "" {
		return fmt.Errorf("output file is empty")
	}
	if r.sourceKind != 0 && r.sourceKind != 1 {
		return fmt.Errorf("unknown source type")
	}
	return nil
}

func (r navigatorBackupRecipe) plan() []string {
	steps := []string{}
	if r.sourceKind == 0 {
		steps = append(steps, "Check that the local source exists")
	} else {
		steps = append(steps, "Leave remote source availability to rsync")
	}
	steps = append(steps,
		"Check that the destination exists",
		"Acquire a non-blocking lock",
		"Run the visible rsync command",
		"Keep the previous destination unchanged if rsync fails",
	)
	return steps
}

func (r navigatorBackupRecipe) warnings() []string {
	warnings := []string{
		"No files will be deleted.",
		"The generated script will not run commands during review.",
		"The output file will not be overwritten.",
	}
	if r.sourceKind == 1 {
		warnings = append(warnings, "A remote source is not checked with local exists.")
	}
	return warnings
}

func (r navigatorBackupRecipe) script() string {
	return navigatorBackupScript(r.sourceKind, r.source, r.destination, r.lockPath)
}
