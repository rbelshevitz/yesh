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
		}
	}
	return nil
}
