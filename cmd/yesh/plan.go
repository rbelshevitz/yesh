package main

import (
	"bytes"
	"context"
)

// renderPlan executes an already parsed script in plan mode. Keeping the AST
// as the input makes the navigator use the same parser/runtime boundary as the
// command-line interface without starting a second yesh process.
func renderPlan(items []statement, filename string) (string, error) {
	var output bytes.Buffer
	r := &runtime{
		ctx:      context.Background(),
		values:   map[string]value{},
		filename: filename,
		plan:     true,
		output:   &output,
	}
	if err := r.execute(items); err != nil {
		return output.String(), err
	}
	return output.String(), nil
}
