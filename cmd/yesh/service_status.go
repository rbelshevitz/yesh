package main

import (
	"fmt"
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
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service status expects one non-empty unit name")
	}
	if err := validateServiceUnit(s.manager, v.scalar); err != nil {
		return err
	}
	if s.manager == "auto" {
		if r.plan {
			fmt.Fprintf(r.outputWriter(), "PLAN detect systemd or OpenRC for service %q\n", v.scalar)
			return nil
		}
		manager, status, err := r.queryAutoServiceStatus(v.scalar)
		if err != nil {
			return fmt.Errorf("service status %q: %w", v.scalar, err)
		}
		fmt.Fprintf(r.outputWriter(), "SERVICE_STATUS id=%s load_state=%s active_state=%s sub_state=%s supervisor=%s\n", status.ID, status.LoadState, status.ActiveState, status.SubState, manager)
		return nil
	}
	if s.manager == "openrc" {
		if r.plan {
			fmt.Fprintf(r.outputWriter(), "PLAN %q\n", openRCStatusArgv())
			fmt.Fprintf(r.outputWriter(), "PLAN verify OpenRC service %q\n", v.scalar)
			return nil
		}
		status, err := r.queryOpenRCServiceStatus(v.scalar)
		if err != nil {
			return fmt.Errorf("service status %q: %w", v.scalar, err)
		}
		fmt.Fprintf(r.outputWriter(), "SERVICE_STATUS id=%s load_state=loaded active_state=%s sub_state=%s supervisor=openrc runlevel=%s supervised=%t respawns=%d\n", v.scalar, openRCActiveState(status.State), status.State, status.Runlevel, status.Supervised, status.Respawns)
		return nil
	}
	argv := systemctlArgv(s.manager, "show", "--no-pager", "--property=Id,LoadState,ActiveState,SubState", v.scalar)
	if r.plan {
		fmt.Fprintf(r.outputWriter(), "PLAN %q\n", argv)
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
	fmt.Fprintf(r.outputWriter(), "SERVICE_STATUS id=%s load_state=%s active_state=%s sub_state=%s\n",
		status.ID, status.LoadState, status.ActiveState, status.SubState)
	return nil
}

func (r *runtime) executeServiceRestart(s serviceRestartStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service %s expects one non-empty unit name", s.action)
	}
	if err := validateServiceUnit(s.manager, v.scalar); err != nil {
		return err
	}
	if s.expectedState != "active" {
		return fmt.Errorf("service %s currently supports only postcondition active", s.action)
	}
	if s.manager == "openrc" {
		return fmt.Errorf("service %s is not implemented for OpenRC", s.action)
	}
	argv := systemctlArgv(s.manager, s.action, v.scalar)
	if r.plan {
		fmt.Fprintf(r.outputWriter(), "PLAN %q until %s within %s\n", argv, s.expectedState, s.timeoutText)
		return nil
	}
	return r.transitionService(s.manager, s.action, v.scalar, s.expectedState, s.timeout)
}

func (r *runtime) executeServiceWait(s serviceWaitStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("service wait expects one non-empty unit name")
	}
	if err := validateServiceUnit(s.manager, v.scalar); err != nil {
		return err
	}
	if s.expectedState != "active" {
		return fmt.Errorf("service wait currently supports only postcondition active")
	}
	if s.manager == "openrc" {
		return fmt.Errorf("service wait is not implemented for OpenRC")
	}
	argv := systemctlArgv(s.manager, "show", "--no-pager", "--property=Id,LoadState,ActiveState,SubState", v.scalar)
	if r.plan {
		fmt.Fprintf(r.outputWriter(), "PLAN %q until %s within %s\n", argv, s.expectedState, s.timeoutText)
		return nil
	}
	status, err := r.waitForService(s.manager, v.scalar, s.expectedState, s.timeout)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.outputWriter(), "SERVICE_WAIT id=%s active_state=%s sub_state=%s\n", status.ID, status.ActiveState, status.SubState)
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

func validateServiceUnit(manager, unit string) error {
	if manager != "openrc" && manager != "auto" {
		return validateUnitName(unit)
	}
	if unit == "" || strings.ContainsAny(unit, " \t\r\n\x00/|;&<>*?[]") || strings.HasPrefix(unit, "-") {
		return fmt.Errorf("OpenRC service operation expects one service name without path, glob, or shell operator")
	}
	return nil
}

func (r *runtime) queryAutoServiceStatus(unit string) (string, serviceStatus, error) {
	system, systemErr := r.queryServiceStatus("system", unit)
	if systemErr == nil && system.ID != unit {
		systemErr = fmt.Errorf("systemctl returned canonical unit %q; aliases are not supported", system.ID)
	}
	openrc, openRCErr := r.queryOpenRCServiceStatus(unit)
	if systemErr == nil && openRCErr == nil {
		return "", serviceStatus{}, fmt.Errorf("service %q is reported by both systemd and OpenRC", unit)
	}
	if systemErr == nil {
		return "systemd", system, nil
	}
	if openRCErr == nil {
		return "openrc", openRCStatusAsServiceStatus(unit, openrc), nil
	}
	return "", serviceStatus{}, fmt.Errorf("neither systemd nor OpenRC confirmed service %q (systemd: %v; OpenRC: %v)", unit, systemErr, openRCErr)
}

func openRCStatusAsServiceStatus(unit string, status openRCServiceStatus) serviceStatus {
	return serviceStatus{ID: unit, LoadState: "loaded", ActiveState: openRCActiveState(status.State), SubState: status.State}
}

func parseServiceStatus(data []byte) (serviceStatus, error) {
	var result serviceStatus
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
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
