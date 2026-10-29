//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const serviceRestartProperties = "Id,LoadState,ActiveState,SubState,Result,ExecMainCode,ExecMainStatus,ActiveEnterTimestampMonotonic"

func (r *runtime) queryServiceStatus(manager, unit string) (serviceStatus, error) {
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	return queryServiceStatusContext(ctx, manager, unit)
}

func queryServiceStatusContext(ctx context.Context, manager, unit string) (serviceStatus, error) {
	argv := systemctlArgv(manager, "show", "--no-pager", "--property=Id,LoadState,ActiveState,SubState", unit)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, diagnostic storageOutput
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return serviceStatus{}, fmt.Errorf("%w", ctx.Err())
		}
		message := strings.TrimSpace(diagnostic.String())
		if message == "" {
			message = err.Error()
		}
		return serviceStatus{}, fmt.Errorf("systemctl failed: %s", message)
	}
	if out.exceeded || diagnostic.exceeded {
		return serviceStatus{}, fmt.Errorf("systemctl output exceeds %d bytes", maxStorageOutput)
	}
	return parseServiceStatus(out.Bytes())
}

func queryServiceRestartStatusContext(ctx context.Context, manager, unit string) (serviceStatus, error) {
	argv := systemctlArgv(manager, "show", "--no-pager", "--property="+serviceRestartProperties, unit)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, diagnostic storageOutput
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return serviceStatus{}, fmt.Errorf("%w", ctx.Err())
		}
		message := strings.TrimSpace(diagnostic.String())
		if message == "" {
			message = err.Error()
		}
		return serviceStatus{}, fmt.Errorf("systemctl failed: %s", message)
	}
	if out.exceeded || diagnostic.exceeded {
		return serviceStatus{}, fmt.Errorf("systemctl output exceeds %d bytes", maxStorageOutput)
	}
	return parseServiceStatus(out.Bytes())
}

func (r *runtime) restartService(manager, unit, expectedState string, timeout time.Duration) error {
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	before, err := queryServiceRestartStatusContext(ctx, manager, unit)
	if err != nil {
		return fmt.Errorf("service restart %q precondition: %w", unit, err)
	}
	argv := systemctlArgv(manager, "restart", unit)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var diagnostic storageOutput
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("service restart %q: %w", unit, ctx.Err())
		}
		message := strings.TrimSpace(diagnostic.String())
		if message == "" {
			message = err.Error()
		}
		if failed, snapshotErr := queryServiceRestartStatusContext(ctx, manager, unit); snapshotErr == nil && failed.ID == unit {
			return fmt.Errorf("service restart %q failed: %s; result=%s exec_main_code=%s exec_main_status=%s active_state=%s sub_state=%s", unit, message, failed.Result, failed.ExecMainCode, failed.ExecMainStatus, failed.ActiveState, failed.SubState)
		}
		return fmt.Errorf("service restart %q failed: %s", unit, message)
	}
	status, err := queryServiceRestartStatusContext(ctx, manager, unit)
	if err != nil {
		return fmt.Errorf("service restart %q postcondition: %w", unit, err)
	}
	status, err = waitForRestartedService(ctx, manager, unit, expectedState, status)
	if err != nil {
		return err
	}
	if status.ID != unit || status.ActiveState != expectedState {
		return fmt.Errorf("service restart %q postcondition failed: id=%s active_state=%s sub_state=%s result=%s exec_main_code=%s exec_main_status=%s active_enter_timestamp_monotonic=%s", unit, status.ID, status.ActiveState, status.SubState, status.Result, status.ExecMainCode, status.ExecMainStatus, status.ActiveEnterTimestampMonotonic)
	}
	if before.ActiveEnterTimestampMonotonic != "" && status.ActiveEnterTimestampMonotonic != "" && before.ActiveEnterTimestampMonotonic == status.ActiveEnterTimestampMonotonic {
		fmt.Fprintf(os.Stderr, "yesh: service restart %q warning: ActiveEnterTimestampMonotonic did not change (%s)\n", unit, status.ActiveEnterTimestampMonotonic)
	}
	fmt.Fprintf(os.Stdout, "SERVICE_RESTART id=%s active_state=%s sub_state=%s result=%s exec_main_code=%s exec_main_status=%s active_enter_timestamp_monotonic=%s\n", status.ID, status.ActiveState, status.SubState, status.Result, status.ExecMainCode, status.ExecMainStatus, status.ActiveEnterTimestampMonotonic)
	return nil
}

func waitForRestartedService(ctx context.Context, manager, unit, expectedState string, last serviceStatus) (serviceStatus, error) {
	if last.ID != unit {
		return serviceStatus{}, fmt.Errorf("service restart %q: systemctl returned canonical unit %q; aliases are not supported", unit, last.ID)
	}
	if err := waitStateError(unit, last); err != nil {
		return serviceStatus{}, fmt.Errorf("service restart %q postcondition failed: %w", unit, err)
	}
	if last.ID == unit && last.ActiveState == expectedState {
		return last, nil
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return serviceStatus{}, fmt.Errorf("service restart %q timed out: last active_state=%s sub_state=%s", unit, last.ActiveState, last.SubState)
		case <-ticker.C:
			var err error
			last, err = queryServiceRestartStatusContext(ctx, manager, unit)
			if err != nil {
				return serviceStatus{}, fmt.Errorf("service restart %q postcondition: %w", unit, err)
			}
			if err := waitStateError(unit, last); err != nil {
				return serviceStatus{}, fmt.Errorf("service restart %q postcondition failed: %w", unit, err)
			}
			if last.ID != unit {
				return serviceStatus{}, fmt.Errorf("service restart %q: systemctl returned canonical unit %q; aliases are not supported", unit, last.ID)
			}
			if last.ActiveState == expectedState {
				return last, nil
			}
		}
	}
}

func (r *runtime) waitForService(manager, unit, expectedState string, timeout time.Duration) (serviceStatus, error) {
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	check := func() (serviceStatus, error) {
		status, err := queryServiceStatusContext(ctx, manager, unit)
		if err != nil {
			return serviceStatus{}, fmt.Errorf("service wait %q: %w", unit, err)
		}
		if status.ID != unit {
			return serviceStatus{}, fmt.Errorf("service wait %q: systemctl returned canonical unit %q; aliases are not supported", unit, status.ID)
		}
		return status, nil
	}

	last, err := check()
	if err != nil {
		return serviceStatus{}, err
	}
	if err := waitStateError(unit, last); err != nil {
		return serviceStatus{}, err
	}
	if last.ActiveState == expectedState {
		return last, nil
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return serviceStatus{}, fmt.Errorf("service wait %q timed out: last active_state=%s sub_state=%s", unit, last.ActiveState, last.SubState)
		case <-ticker.C:
			last, err = check()
			if err != nil {
				return serviceStatus{}, err
			}
			if err := waitStateError(unit, last); err != nil {
				return serviceStatus{}, err
			}
			if last.ActiveState == expectedState {
				return last, nil
			}
		}
	}
}

func waitStateError(unit string, status serviceStatus) error {
	if status.ActiveState == "failed" || status.LoadState == "not-found" || status.LoadState == "masked" {
		return fmt.Errorf("service wait %q cannot reach active: load_state=%s active_state=%s sub_state=%s", unit, status.LoadState, status.ActiveState, status.SubState)
	}
	return nil
}
