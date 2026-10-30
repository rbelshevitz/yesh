package main

import (
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
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [--plan] SCRIPT.yesh\n", filepath.Base(os.Args[0]))
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
		if executionStarted && !*plan {
			fmt.Fprintln(os.Stderr, "yesh: execution stopped; earlier actions may have completed, the current action may be partial, and no rollback was performed")
		}
		fmt.Fprintln(os.Stderr, "yesh:", err)
		return 1
	}
	return 0
}
