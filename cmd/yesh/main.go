package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

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
	data, err := os.ReadFile(script)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	items, err := parse(string(data), script)
	if err == nil {
		ctx, stop := commandContext()
		defer stop()
		err = (&runtime{ctx: ctx, values: map[string]value{}, filename: script, plan: *plan}).execute(items)
	}
	if err != nil {
		var exit scriptExit
		if errors.As(err, &exit) {
			return exit.code
		}
		fmt.Fprintln(os.Stderr, "yesh:", err)
		return 1
	}
	return 0
}
