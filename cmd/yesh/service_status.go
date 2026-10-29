package main

import (
	"fmt"
	"os"
	"strings"
)

type serviceStatus struct {
	ID                            string
	LoadState                     string
	ActiveState                   string
	SubState                      string
	Result                        string
	ExecMainCode                  string
	ExecMainStatus                string
	ActiveEnterTimestampMonotonic string
}

func (r *runtime) executeServiceStatus(s serviceStatusStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service status expects one non-empty unit name")
	}
	if err := validateUnitName(v.scalar); err != nil {
		return err
	}
	argv := systemctlArgv(s.manager, "show", "--no-pager", "--property=Id,LoadState,ActiveState,SubState", v.scalar)
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN %q\n", argv)
		return nil
	}
	requested := argv[len(argv)-1]
	status, err := r.queryServiceStatus(s.manager, requested)
	if err != nil {
		return fmt.Errorf("service status %q: %w", requested, err)
	}
	if status.ID != requested {
		return fmt.Errorf("systemctl returned canonical unit %q for requested %q; aliases are not supported", status.ID, requested)
	}
	fmt.Fprintf(os.Stdout, "SERVICE_STATUS id=%s load_state=%s active_state=%s sub_state=%s\n",
		status.ID, status.LoadState, status.ActiveState, status.SubState)
	return nil
}

func (r *runtime) executeServiceRestart(s serviceRestartStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service restart expects one non-empty unit name")
	}
	if err := validateUnitName(v.scalar); err != nil {
		return err
	}
	if s.expectedState != "active" {
		return fmt.Errorf("service restart currently supports only postcondition active")
	}
	argv := systemctlArgv(s.manager, "restart", v.scalar)
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN %q until %s within %s\n", argv, s.expectedState, s.timeoutText)
		return nil
	}
	return r.restartService(s.manager, v.scalar, s.expectedState, s.timeout)
}

func (r *runtime) executeServiceWait(s serviceWaitStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service wait expects one non-empty unit name")
	}
	if err := validateUnitName(v.scalar); err != nil {
		return err
	}
	if s.expectedState != "active" {
		return fmt.Errorf("service wait currently supports only postcondition active")
	}
	argv := systemctlArgv(s.manager, "show", "--no-pager", "--property=Id,LoadState,ActiveState,SubState", v.scalar)
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN %q until %s within %s\n", argv, s.expectedState, s.timeoutText)
		return nil
	}
	status, err := r.waitForService(s.manager, v.scalar, s.expectedState, s.timeout)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "SERVICE_WAIT id=%s active_state=%s sub_state=%s\n", status.ID, status.ActiveState, status.SubState)
	return nil
}

func systemctlArgv(manager string, args ...string) []string {
	argv := []string{"systemctl"}
	if manager == "user" {
		argv = append(argv, "--user")
	}
	return append(argv, args...)
}

func validateUnitName(unit string) error {
	if unit == "" || strings.ContainsAny(unit, " \t\r\n\x00/|;&<>*?[]") || strings.HasPrefix(unit, "-") || !strings.Contains(unit, ".") {
		return fmt.Errorf("service operation expects one full unit name with suffix, without path, glob, or shell operator")
	}
	if strings.Contains(unit, "@.") {
		return fmt.Errorf("service operation expects a concrete unit instance, not template %q", unit)
	}
	return nil
}

func parseServiceStatus(data []byte) (serviceStatus, error) {
	var result serviceStatus
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || value == "" {
			return serviceStatus{}, fmt.Errorf("invalid systemctl show field %q", line)
		}
		if seen[key] {
			return serviceStatus{}, fmt.Errorf("duplicate systemctl show field %q", key)
		}
		seen[key] = true
		switch key {
		case "Id":
			result.ID = value
		case "LoadState":
			result.LoadState = value
		case "ActiveState":
			result.ActiveState = value
		case "SubState":
			result.SubState = value
		case "Result":
			result.Result = value
		case "ExecMainCode":
			result.ExecMainCode = value
		case "ExecMainStatus":
			result.ExecMainStatus = value
		case "ActiveEnterTimestampMonotonic":
			result.ActiveEnterTimestampMonotonic = value
		default:
			return serviceStatus{}, fmt.Errorf("unexpected systemctl show field %q", key)
		}
	}
	if result.ID == "" || result.LoadState == "" || result.ActiveState == "" || result.SubState == "" {
		return serviceStatus{}, fmt.Errorf("systemctl show omitted a required field")
	}
	return result, nil
}
