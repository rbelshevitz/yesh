package main

import (
	"fmt"
	"strings"
)

const maxProcessSnapshot = 10000

func (r *runtime) evalProcessQuery(raw string, line int) (value, error) {
	tokens, err := words(raw)
	if err != nil {
		return value{}, fmt.Errorf("%s:%d: %w", r.filename, line, err)
	}
	if len(tokens) != 5 || tokens[0].text != "processes" || tokens[1].text != "where" ||
		tokens[2].text != "name" || tokens[3].text != "is" || tokens[4].operator {
		return value{}, fmt.Errorf("%s:%d: expected: processes where name is process_name", r.filename, line)
	}
	name, err := r.conditionValue(tokens[4], line)
	if err != nil {
		return value{}, err
	}
	if name.isList || name.regex != nil || name.scalar == "" || strings.ContainsRune(name.scalar, 0) {
		return value{}, fmt.Errorf("%s:%d: process name must be one non-empty string", r.filename, line)
	}
	processes, err := findProcessInfo(name.scalar, maxProcessSnapshot)
	if err != nil {
		return value{}, fmt.Errorf("%s:%d: processes: %w", r.filename, line, err)
	}
	return value{processes: processes, isProcesses: true}, nil
}
