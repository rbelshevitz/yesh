//go:build !windows

package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

func commandContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
}

type processGroup struct {
	mu sync.RWMutex
	id int
}

func (g *processGroup) configure(cmd *exec.Cmd) {
	g.mu.RLock()
	pgid := g.id
	g.mu.RUnlock()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: pgid}
}

func (g *processGroup) started(cmd *exec.Cmd) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.id == 0 {
		g.id = cmd.Process.Pid
	}
}

func (g *processGroup) terminate() error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.id == 0 {
		return nil
	}
	return syscall.Kill(-g.id, syscall.SIGTERM)
}

func (g *processGroup) kill() error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.id == 0 {
		return nil
	}
	return syscall.Kill(-g.id, syscall.SIGKILL)
}
