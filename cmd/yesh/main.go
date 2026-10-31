package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxScriptBytes = 4 << 20

func main() {
	os.Exit(runCLI())
}

func runCLI() int {
	flags := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	plan := flags.Bool("plan", false, "show commands without executing them")
	diagnostic := flags.String("diagnostic", "text", "error diagnostic format: text or json")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [--plan] SCRIPT.yesh\n", filepath.Base(os.Args[0]))
		return 2
	}
	if *diagnostic != "text" && *diagnostic != "json" {
		fmt.Fprintln(os.Stderr, "yesh: -diagnostic must be text or json")
		return 2
	}
	script := flags.Arg(0)
	file, err := os.Open(script)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := io.ReadAll(io.LimitReader(file, maxScriptBytes+1))
	closeErr := file.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if closeErr != nil {
		fmt.Fprintln(os.Stderr, closeErr)
		return 1
	}
	if len(data) > maxScriptBytes {
		fmt.Fprintf(os.Stderr, "yesh: script exceeds %d bytes\n", maxScriptBytes)
		return 1
	}
	items, err := parse(string(data), script)
	executionStarted := false
	if err == nil {
		ctx, stop := commandContext()
		defer stop()
		executionStarted = true
		err = (&runtime{ctx: ctx, values: map[string]value{}, filename: script, plan: *plan}).execute(items)
	}
	if err != nil {
		var exit scriptExit
		if errors.As(err, &exit) {
			return exit.code
		}
		if *diagnostic == "json" {
			writeJSONDiagnostic(os.Stderr, err, executionStarted && !*plan)
		} else {
			if executionStarted && !*plan {
				fmt.Fprintln(os.Stderr, "yesh: execution stopped; earlier actions may have completed, the current action may be partial, and no rollback was performed")
			}
			fmt.Fprintln(os.Stderr, "yesh:", err)
		}
		return 1
	}
	return 0
}

func writeJSONDiagnostic(w io.Writer, err error, executionStopped bool) {
	report := struct {
		Message          string   `json:"message"`
		ExecutionStopped bool     `json:"execution_stopped"`
		Stage            int      `json:"stage,omitempty"`
		Stages           int      `json:"stages,omitempty"`
		Argv             []string `json:"argv,omitempty"`
		ExitCode         *int     `json:"exit_code,omitempty"`
		Signal           string   `json:"signal,omitempty"`
		TimedOut         bool     `json:"timed_out,omitempty"`
		Stderr           string   `json:"stderr,omitempty"`
		StderrTruncated  bool     `json:"stderr_truncated,omitempty"`
		DurationMS       int64    `json:"duration_ms,omitempty"`
	}{Message: err.Error(), ExecutionStopped: executionStopped}
	var diagnostic *commandDiagnostic
	if errors.As(err, &diagnostic) {
		report.Stage = diagnostic.Stage
		report.Stages = diagnostic.Stages
		report.Argv = diagnostic.Argv
		code := diagnostic.ExitCode
		if code >= 0 {
			report.ExitCode = &code
		}
		report.Signal = diagnostic.Signal
		report.TimedOut = diagnostic.TimedOut
		report.Stderr = diagnostic.Stderr
		report.StderrTruncated = diagnostic.StderrTruncated
		report.DurationMS = diagnostic.Duration.Milliseconds()
	}
	_ = json.NewEncoder(w).Encode(report)
}
