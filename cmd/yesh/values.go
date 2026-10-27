package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type value struct {
	scalar      string
	list        []string
	isList      bool
	regex       *regexp.Regexp
	secret      bool
	processes   []processInfo
	isProcesses bool
}

type runtime struct {
	ctx      context.Context
	values   map[string]value
	filename string
	plan     bool
	steps    int
}

const maxLoopIterations = 10000
const maxSteps = 100000
const commandTimeout = 5 * time.Minute
const maxRegexPattern = 64 * 1024

type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("command exited with status %d", e.code) }

// Script completion is control flow, not a failed external command.
type scriptExit struct{ code int }

func (e scriptExit) Error() string { return fmt.Sprintf("script exited with status %d", e.code) }

func exitCode(v value) (int, error) {
	if v.isList || v.regex != nil || v.scalar == "" {
		return 0, fmt.Errorf("exit expects a decimal code from 0 to 255")
	}
	for _, c := range v.scalar {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("exit expects a decimal code from 0 to 255")
		}
	}
	code, err := strconv.Atoi(v.scalar)
	if err != nil || code > 255 {
		return 0, fmt.Errorf("exit expects a decimal code from 0 to 255")
	}
	return code, nil
}

func (r *runtime) eval(raw string, line int) (value, error) {
	raw = strings.TrimSpace(raw)
	if v, ok := r.values[raw]; ok {
		return v, nil
	}
	if strings.HasPrefix(raw, "processes ") {
		return r.evalProcessQuery(raw, line)
	}
	if strings.HasPrefix(raw, "regex ") {
		pattern, err := r.eval(strings.TrimSpace(strings.TrimPrefix(raw, "regex ")), line)
		if err != nil {
			return value{}, err
		}
		if pattern.isList || pattern.regex != nil {
			return value{}, fmt.Errorf("%s:%d: regex pattern must be one string", r.filename, line)
		}
		if len(pattern.scalar) > maxRegexPattern {
			return value{}, fmt.Errorf("%s:%d: regex pattern exceeds %d bytes", r.filename, line, maxRegexPattern)
		}
		compiled, err := regexp.Compile(pattern.scalar)
		if err != nil {
			return value{}, fmt.Errorf("%s:%d: invalid regex: %w", r.filename, line, err)
		}
		if err := validateRegexNames(compiled); err != nil {
			return value{}, fmt.Errorf("%s:%d: %w", r.filename, line, err)
		}
		return value{regex: compiled}, nil
	}
	if raw == "environment" || strings.HasPrefix(raw, "environment ") {
		name := strings.TrimSpace(strings.TrimPrefix(raw, "environment "))
		if !validEnvironmentName(name) {
			return value{}, fmt.Errorf("%s:%d: environment expects one variable name", r.filename, line)
		}
		v, ok := os.LookupEnv(name)
		if !ok || v == "" {
			return value{}, fmt.Errorf("%s:%d: environment %s is missing or empty", r.filename, line, name)
		}
		return value{scalar: v, secret: true}, nil
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		inside := strings.TrimSpace(raw[1 : len(raw)-1])
		if inside == "" {
			return value{isList: true}, nil
		}
		var result []string
		items, err := listElements(inside)
		if err != nil {
			return value{}, fmt.Errorf("%s:%d: %w", r.filename, line, err)
		}
		for _, item := range items {
			v, err := r.eval(strings.TrimSpace(item), line)
			if err != nil {
				return value{}, err
			}
			if v.isList || v.regex != nil {
				return value{}, fmt.Errorf("%s:%d: nested lists are not supported", r.filename, line)
			}
			result = append(result, v.scalar)
		}
		return value{list: result, isList: true}, nil
	}
	if strings.HasPrefix(raw, "[") {
		return value{}, fmt.Errorf("%s:%d: unterminated list", r.filename, line)
	}
	if strings.HasPrefix(raw, `"`) || strings.HasPrefix(raw, `'`) {
		parts, err := words(raw)
		if err != nil || len(parts) != 1 || parts[0].operator {
			return value{}, fmt.Errorf("%s:%d: invalid string: %s", r.filename, line, raw)
		}
		return value{scalar: parts[0].text}, nil
	}
	if raw == "" {
		return value{}, fmt.Errorf("%s:%d: empty value", r.filename, line)
	}
	return value{scalar: raw}, nil
}

func validEnvironmentName(name string) bool {
	if name == "" || (name[0] != '_' && (name[0] < 'A' || name[0] > 'Z') && (name[0] < 'a' || name[0] > 'z')) {
		return false
	}
	for _, c := range name[1:] {
		if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// Split only at commas outside strings and nested brackets. Nested lists
// are diagnosed by eval rather than accidentally flattening their text.
func listElements(text string) ([]string, error) {
	var elements []string
	start, depth := 0, 0
	quote := rune(0)
	escaped := false
	for i, c := range text {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if c == '\\' && quote == '"' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '[':
			depth++
		case ']':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unmatched list bracket")
			}
		case ',':
			if depth == 0 {
				elements = append(elements, text[start:i])
				start = i + 1
			}
		}
	}
	if quote != 0 || depth != 0 {
		return nil, fmt.Errorf("unterminated string or list")
	}
	return append(elements, text[start:]), nil
}

func (r *runtime) expand(token string, line int) ([]string, error) {
	if v, ok := r.values[token]; ok {
		if v.isList {
			return v.list, nil
		}
		if v.regex != nil {
			return nil, fmt.Errorf("%s:%d: regex cannot be used as a command argument", r.filename, line)
		}
		return []string{v.scalar}, nil
	}
	v, err := r.eval(token, line)
	if err != nil {
		return nil, err
	}
	if v.isList {
		return v.list, nil
	}
	if v.regex != nil {
		return nil, fmt.Errorf("%s:%d: regex cannot be used as a command argument", r.filename, line)
	}
	return []string{v.scalar}, nil
}

func validateRegexNames(re *regexp.Regexp) error {
	seen := make(map[string]struct{})
	for _, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate regex capture name %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}
