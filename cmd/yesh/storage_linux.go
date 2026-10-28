//go:build linux

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maxStorageOutput = 4 << 20

// A capped writer drains excess output without allocating it; overflow is an error.
type storageOutput struct {
	bytes.Buffer
	exceeded bool
}

func (b *storageOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxStorageOutput - b.Len()
	if len(p) > remaining {
		b.exceeded = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func (r *runtime) storageQuery(name string, args ...string) ([]byte, error) {
	return r.storageQueryPolicy(false, name, args...)
}
func (r *runtime) storageQueryPolicy(strict bool, name string, args ...string) ([]byte, error) {
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if cmd.Err != nil {
		return nil, fmt.Errorf("%s is required (util-linux package): %w", name, cmd.Err)
	}
	// Machine-readable output and no privileged fallback or interactive input.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "LC_ALL=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	group := &processGroup{}
	group.configure(cmd)
	cmd.Cancel = group.terminate
	cmd.WaitDelay = 2 * time.Second
	var out, diagnostic storageOutput
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	group.started(cmd)
	done := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		select {
		case <-ctx.Done():
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = group.kill()
			case <-done:
			}
		case <-done:
		}
	}()
	err := cmd.Wait()
	// Also release descendants retaining output descriptors after their leader exits.
	_ = group.kill()
	close(done)
	<-exited
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s: %w", name, ctx.Err())
	}
	if out.exceeded || diagnostic.exceeded {
		return nil, fmt.Errorf("%s output exceeds %d bytes", name, maxStorageOutput)
	}
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(diagnostic.String()))
	}
	if strict && diagnostic.Len() != 0 {
		return nil, fmt.Errorf("%s reported diagnostics; refusing partial table: %s", name, strings.TrimSpace(diagnostic.String()))
	}
	return out.Bytes(), nil
}
