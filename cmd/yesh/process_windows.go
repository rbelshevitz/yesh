//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"sync"
)

func processSignal(*os.ProcessState) string { return "" }

func commandContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

type processGroup struct {
	mu       sync.RWMutex
	commands []*exec.Cmd
}

func (g *processGroup) configure(cmd *exec.Cmd) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.commands = append(g.commands, cmd)
}

func (g *processGroup) started(cmd *exec.Cmd) {}

func (g *processGroup) terminate() error {
	g.mu.RLock()
	commands := append([]*exec.Cmd(nil), g.commands...)
	g.mu.RUnlock()
	for _, cmd := range commands {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	return nil
}

func (g *processGroup) kill() error { return g.terminate() }
