package main

import (
	"fmt"
	"os"
	"strings"
)

func (r *runtime) execute(items []statement) error {
	for _, raw := range items {
		r.steps++
		if r.steps > maxSteps {
			return fmt.Errorf("%s: execution limit of %d steps exceeded", r.filename, maxSteps)
		}
		switch s := raw.(type) {
		case storageStmt:
			if err := r.executeStorage(s); err != nil {
				return fmt.Errorf("%s:%d: %w", r.filename, s.line, err)
			}
		case exitStmt:
			v, err := r.conditionValue(s.code, s.line)
			if err != nil {
				return err
			}
			code, err := exitCode(v)
			if err != nil {
				return fmt.Errorf("%s:%d: %w", r.filename, s.line, err)
			}
			if r.plan {
				fmt.Fprintf(os.Stdout, "PLAN exit %d\n", code)
				continue
			}
			return scriptExit{code: code}
		case assignStmt:
			v, err := r.eval(s.value, s.line)
			if err != nil {
				return err
			}
			r.values[s.name] = v
		case showStmt:
			v, ok := r.values[s.name]
			if !ok || !v.isProcesses {
				return fmt.Errorf("%s:%d: show expects a process set", r.filename, s.line)
			}
			if r.plan {
				if len(s.fields) == 0 {
					fmt.Fprintf(os.Stdout, "PLAN show %s\n", s.name)
				} else {
					fmt.Fprintf(os.Stdout, "PLAN show %s fields %s\n", s.name, strings.Join(s.fields, " "))
				}
				continue
			}
			if err := writeProcessTable(os.Stdout, v.processes, s.fields); err != nil {
				return fmt.Errorf("%s:%d: show: %w", r.filename, s.line, err)
			}
		case summarizeStmt:
			v, ok := r.values[s.name]
			if !ok || !v.isProcesses {
				return fmt.Errorf("%s:%d: summarize expects a process set", r.filename, s.line)
			}
			if r.plan {
				fmt.Fprintf(os.Stdout, "PLAN summarize %s memory %s\n", s.name, s.metric)
				continue
			}
			if err := writeProcessSummary(os.Stdout, v.processes, s.metric); err != nil {
				return fmt.Errorf("%s:%d: summarize: %w", r.filename, s.line, err)
			}
		case serviceStatusStmt:
			if err := r.executeServiceStatus(s); err != nil {
				return fmt.Errorf("%s:%d: %w", r.filename, s.line, err)
			}
		case serviceRestartStmt:
			if err := r.executeServiceRestart(s); err != nil {
				return fmt.Errorf("%s:%d: %w", r.filename, s.line, err)
			}
		case serviceWaitStmt:
			if err := r.executeServiceWait(s); err != nil {
				return fmt.Errorf("%s:%d: %w", r.filename, s.line, err)
			}
		case journalStmt:
			if err := r.executeJournal(s); err != nil {
				return fmt.Errorf("%s:%d: %w", r.filename, s.line, err)
			}
		case commandStmt:
			if err := r.run(s.text, s.line, true); err != nil {
				return err
			}
		case ifStmt:
			if r.plan && s.condition.kind == commandCondition {
				stages, err := r.parseCommand(s.condition.text, s.line)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "PLAN IF %s\n", r.formatPipeline(stages))
				if err := r.execute(s.body); err != nil {
					return err
				}
				if len(s.otherwise) > 0 {
					fmt.Fprintln(os.Stdout, "PLAN ELSE")
					if err := r.execute(s.otherwise); err != nil {
						return err
					}
				}
				continue
			}
			ok, err := r.evaluateCondition(s.condition, s.line)
			if err != nil {
				return err
			}
			if ok {
				if err := r.execute(s.body); err != nil {
					return err
				}
			} else if err := r.execute(s.otherwise); err != nil {
				return err
			}
		case forStmt:
			v, err := r.eval(s.values, s.line)
			if err != nil {
				return err
			}
			if !v.isList {
				return fmt.Errorf("%s:%d: for expects a list", r.filename, s.line)
			}
			if len(v.list) > maxLoopIterations {
				return fmt.Errorf("%s:%d: loop exceeds limit of %d iterations", r.filename, s.line, maxLoopIterations)
			}
			for _, item := range v.list {
				r.values[s.name] = value{scalar: item}
				if err := r.execute(s.body); err != nil {
					return err
				}
			}
		case lockStmt:
			v, err := r.eval(s.path, s.line)
			if err != nil {
				return err
			}
			if v.fstab != nil || v.device != nil || v.isList || v.regex != nil || v.isProcesses || v.scalar == "" {
				return fmt.Errorf("%s:%d: with lock expects one non-empty path", r.filename, s.line)
			}
			if r.plan {
				fmt.Fprintf(os.Stdout, "PLAN lock %q\n", v.scalar)
				if err := r.execute(s.body); err != nil {
					return err
				}
				continue
			}
			if err := r.withLock(v.scalar, func() error { return r.execute(s.body) }); err != nil {
				return fmt.Errorf("%s:%d: lock %q: %w", r.filename, s.line, v.scalar, err)
			}
		case publishStmt:
			source, err := r.expandWord(s.source, s.line)
			if err != nil {
				return err
			}
			dest, err := r.expandWord(s.dest, s.line)
			if err != nil {
				return err
			}
			if len(source) != 1 || len(dest) != 1 || source[0] == "" || dest[0] == "" {
				return fmt.Errorf("%s:%d: publish expects one non-empty source and destination", r.filename, s.line)
			}
			if r.plan {
				fmt.Fprintf(os.Stdout, "PLAN publish %q to %q\n", source[0], dest[0])
				continue
			}
			if source[0] == dest[0] {
				return fmt.Errorf("%s:%d: publish source and destination must differ", r.filename, s.line)
			}
			if err := os.Rename(source[0], dest[0]); err != nil {
				return fmt.Errorf("%s:%d: publish %q to %q: %w", r.filename, s.line, source[0], dest[0], err)
			}
		}
	}
	return nil
}
