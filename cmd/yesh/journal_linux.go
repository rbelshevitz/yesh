//go:build linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func (r *runtime) queryJournal(manager, unit, sinceText string, lines int) ([]byte, error) {
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	argv := journalctlArgv(manager, "--no-pager", "--output=json", "--unit", unit,
		"--since", "-"+sinceText, "--lines", fmt.Sprintf("%d", lines))
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, diagnostic storageOutput
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("journalctl: %w", ctx.Err())
		}
		message := strings.TrimSpace(diagnostic.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("journalctl failed: %s", message)
	}
	if out.exceeded || diagnostic.exceeded {
		return nil, fmt.Errorf("journal output exceeds %d bytes", maxStorageOutput)
	}
	if message := strings.TrimSpace(diagnostic.String()); message != "" {
		return nil, fmt.Errorf("journalctl diagnostic: %s", message)
	}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			return nil, fmt.Errorf("journalctl returned invalid JSON")
		}
	}
	return out.Bytes(), nil
}
