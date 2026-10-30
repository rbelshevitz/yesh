package main

import (
	"fmt"
	"os"
)

func (r *runtime) executeJournal(s journalStmt) error {
	v, err := r.conditionValue(s.unit, s.line)
	if err != nil {
		return err
	}
	if v.fstab != nil || v.device != nil || v.ip != nil || v.cidr != nil || v.isList || v.regex != nil || v.isProcesses || v.secret || v.scalar == "" {
		return fmt.Errorf("journal expects one non-empty unit name")
	}
	if err := validateUnitName(v.scalar); err != nil {
		return err
	}
	argv := journalctlArgv(s.manager, "--no-pager", "--output=json", "--unit", v.scalar,
		"--since", "-"+s.sinceText, "--lines", fmt.Sprintf("%d", s.lines))
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN %q\n", argv)
		return nil
	}
	data, err := r.queryJournal(s.manager, v.scalar, s.sinceText, s.lines)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func journalctlArgv(manager string, args ...string) []string {
	argv := []string{"journalctl"}
	if manager == "user" {
		argv = append(argv, "--user")
	}
	return append(argv, args...)
}
