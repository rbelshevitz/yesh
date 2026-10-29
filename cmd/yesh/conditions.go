package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type conditionKind int

const (
	commandCondition conditionKind = iota
	existsCondition
	matchesCondition
	freeSpaceCondition
	processMemoryCondition
)

type conditionExpr struct {
	kind  conditionKind
	text  string
	terms []commandToken
}

func parseCondition(text string) (conditionExpr, error) {
	text = strings.TrimSpace(text)
	expr := conditionExpr{text: text}
	if strings.HasPrefix(text, "exists(") {
		return expr, fmt.Errorf("use exists path, not exists(path)")
	}
	terms, err := words(text)
	if err != nil {
		return expr, err
	}
	if len(terms) == 0 {
		return expr, fmt.Errorf("if requires a condition")
	}
	expr.terms = terms
	keyword := func(i int, word string) bool {
		return i < len(terms) && !terms[i].literal && !terms[i].operator && terms[i].text == word
	}
	if keyword(0, "exists") {
		if len(terms) != 2 || terms[1].operator {
			return expr, fmt.Errorf("expected: exists path (one string value)")
		}
		expr.kind = existsCondition
	} else if keyword(1, "matches") {
		if len(terms) != 3 && !(len(terms) == 4 && keyword(2, "regex")) {
			return expr, fmt.Errorf("expected: text matches regex")
		}
		for _, term := range terms {
			if term.operator {
				return expr, fmt.Errorf("operators are not allowed inside matches")
			}
		}
		expr.kind = matchesCondition
	} else if keyword(0, "free_space") {
		if len(terms) != 4 || terms[1].operator || !keyword(2, "at_least") || terms[3].operator {
			return expr, fmt.Errorf("expected: free_space path at_least amount")
		}
		if _, err := parseByteSize(terms[3].text); err != nil {
			return expr, err
		}
		expr.kind = freeSpaceCondition
	} else if keyword(0, "memory") {
		if len(terms) != 5 || terms[1].operator || terms[2].operator || terms[3].operator ||
			terms[4].operator || !keyword(3, "at_most") || terms[2].text != "rss" {
			return expr, fmt.Errorf("expected: memory $process_set rss at_most amount")
		}
		if _, err := parseByteSize(terms[4].text); err != nil {
			return expr, err
		}
		expr.kind = processMemoryCondition
	}
	return expr, nil
}

func (r *runtime) conditionValue(term commandToken, line int) (value, error) {
	if term.literal {
		return value{scalar: term.text}, nil
	}
	return r.eval(term.text, line)
}

func (r *runtime) condition(text string, line int) (bool, error) {
	expr, err := parseCondition(text)
	if err != nil {
		return false, fmt.Errorf("%s:%d: %w", r.filename, line, err)
	}
	return r.evaluateCondition(expr, line)
}

func (r *runtime) evaluateCondition(expr conditionExpr, line int) (bool, error) {
	switch expr.kind {
	case existsCondition:
		v, err := r.conditionValue(expr.terms[1], line)
		if err != nil {
			return false, err
		}
		if v.fstab != nil || v.device != nil || v.isList || v.regex != nil || v.scalar == "" || strings.ContainsRune(v.scalar, 0) {
			return false, fmt.Errorf("%s:%d: exists expects one non-empty path string", r.filename, line)
		}
		ok, err := fileExists(v.scalar)
		if err != nil {
			return false, fmt.Errorf("%s:%d: exists: %w", r.filename, line, err)
		}
		return ok, nil
	case matchesCondition:
		left, err := r.conditionValue(expr.terms[0], line)
		if err != nil {
			return false, err
		}
		var right value
		if len(expr.terms) == 4 {
			raw := expr.terms[3].text
			if expr.terms[3].literal {
				raw = strconv.Quote(raw)
			}
			right, err = r.eval("regex "+raw, line)
		} else {
			right, err = r.conditionValue(expr.terms[2], line)
		}
		if err != nil {
			return false, err
		}
		if left.fstab != nil || left.device != nil || left.isList || left.regex != nil || right.regex == nil {
			return false, fmt.Errorf("%s:%d: matches expects text matches regex", r.filename, line)
		}
		return right.regex.MatchString(left.scalar), nil
	case freeSpaceCondition:
		path, err := r.conditionValue(expr.terms[1], line)
		if err != nil {
			return false, err
		}
		if path.fstab != nil || path.device != nil || path.isList || path.regex != nil || path.scalar == "" || strings.ContainsRune(path.scalar, 0) {
			return false, fmt.Errorf("%s:%d: free_space expects one non-empty path string", r.filename, line)
		}
		need, err := parseByteSize(expr.terms[3].text)
		if err != nil {
			return false, fmt.Errorf("%s:%d: %w", r.filename, line, err)
		}
		free, err := freeBytes(path.scalar)
		if err != nil {
			return false, fmt.Errorf("%s:%d: free_space: %w", r.filename, line, err)
		}
		return free >= need, nil
	case processMemoryCondition:
		name, isReference := variableReference(expr.terms[1].text)
		if !isReference {
			return false, fmt.Errorf("%s:%d: memory expects a $process_set", r.filename, line)
		}
		set, ok := r.values[name]
		if !ok || !set.isProcesses {
			return false, fmt.Errorf("%s:%d: memory expects a process set", r.filename, line)
		}
		need, err := parseByteSize(expr.terms[4].text)
		if err != nil {
			return false, fmt.Errorf("%s:%d: %w", r.filename, line, err)
		}
		total, err := processRSSTotal(set.processes)
		if err != nil {
			return false, fmt.Errorf("%s:%d: memory: %w", r.filename, line, err)
		}
		return total <= need, nil
	}
	if r.plan {
		return false, fmt.Errorf("%s:%d: --plan cannot evaluate command condition: %s", r.filename, line, expr.text)
	}
	err := r.run(expr.text, line, false)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	var status statusError
	if (errors.As(err, &exitErr) && exitErr.ProcessState.Exited()) || errors.As(err, &status) {
		return false, nil
	}
	return false, err
}

func fileExists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
