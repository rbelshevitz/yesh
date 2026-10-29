package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type line struct {
	number, indent int
	text           string
}

type statement interface{ stmt() }

type commandStmt struct {
	line int
	text string
}

func (commandStmt) stmt() {}

type storageStmt struct {
	line  int
	terms []commandToken
}

func (storageStmt) stmt() {}

type assignStmt struct {
	line        int
	name, value string
}

func (assignStmt) stmt() {}

type showStmt struct {
	line   int
	name   string
	fields []string
}

func (showStmt) stmt() {}

type summarizeStmt struct {
	line   int
	name   string
	metric string
}

func (summarizeStmt) stmt() {}

type serviceStatusStmt struct {
	line    int
	manager string
	unit    commandToken
}

func (serviceStatusStmt) stmt() {}

type serviceRestartStmt struct {
	line          int
	manager       string
	unit          commandToken
	expectedState string
	timeout       time.Duration
	timeoutText   string
}

func (serviceRestartStmt) stmt() {}

type serviceWaitStmt struct {
	line          int
	manager       string
	unit          commandToken
	expectedState string
	timeout       time.Duration
	timeoutText   string
}

func (serviceWaitStmt) stmt() {}

type journalStmt struct {
	line      int
	manager   string
	unit      commandToken
	since     time.Duration
	sinceText string
	lines     int
}

func (journalStmt) stmt() {}

type exitStmt struct {
	line int
	code commandToken
}

func (exitStmt) stmt() {}

type ifStmt struct {
	line            int
	condition       conditionExpr
	body, otherwise []statement
}

func (ifStmt) stmt() {}

type forStmt struct {
	line         int
	name, values string
	body         []statement
}

func (forStmt) stmt() {}

type lockStmt struct {
	line int
	path string
	body []statement
}

func (lockStmt) stmt() {}

type publishStmt struct {
	line         int
	source, dest commandToken
}

func (publishStmt) stmt() {}

type parser struct {
	lines    []line
	filename string
}

func parse(source, filename string) ([]statement, error) {
	p := parser{filename: filename}
	for n, raw := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if strings.Contains(raw[:indent], "\t") {
			return nil, p.err(n+1, "use spaces for indentation")
		}
		p.lines = append(p.lines, line{n + 1, indent, trimmed})
	}
	items, _, err := p.block(0, 0)
	return items, err
}

func (p parser) err(n int, message string) error {
	return fmt.Errorf("%s:%d: %s", p.filename, n, message)
}

func (p parser) block(index, indent int) ([]statement, int, error) {
	var result []statement
	for index < len(p.lines) {
		l := p.lines[index]
		if l.indent < indent {
			return result, index, nil
		}
		if l.indent > indent {
			return nil, index, p.err(l.number, "unexpected indentation")
		}
		text := l.text
		if text == "if:" {
			return nil, index, p.err(l.number, "if requires a condition")
		}
		if text == "for:" {
			return nil, index, p.err(l.number, "expected: for name in values:")
		}
		if strings.HasPrefix(text, "with lock ") {
			if !strings.HasSuffix(text, ":") {
				return nil, index, p.err(l.number, "expected ':' after with lock path")
			}
			path := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "with lock"), ":"))
			if path == "" {
				return nil, index, p.err(l.number, "with lock requires a path")
			}
			body, next, err := p.child(index, indent)
			if err != nil {
				return nil, index, err
			}
			result = append(result, lockStmt{line: l.number, path: path, body: body})
			index = next
			continue
		}
		if strings.HasPrefix(text, "publish ") {
			tokens, err := words(text)
			if err != nil {
				return nil, index, p.err(l.number, err.Error())
			}
			if len(tokens) != 4 || tokens[0].operator || tokens[0].text != "publish" ||
				tokens[1].operator || tokens[2].operator || tokens[2].text != "to" || tokens[3].operator {
				return nil, index, p.err(l.number, "expected: publish source to destination")
			}
			result = append(result, publishStmt{line: l.number, source: tokens[1], dest: tokens[3]})
			index++
			continue
		}
		if text == "if" || strings.HasPrefix(text, "if ") {
			if !strings.HasSuffix(text, ":") {
				return nil, index, p.err(l.number, "expected ':' after if condition")
			}
		}
		if text == "for" || strings.HasPrefix(text, "for ") {
			if !strings.HasSuffix(text, ":") {
				return nil, index, p.err(l.number, "expected ':' after for values")
			}
		}
		if text == "else:" || text == "else" || strings.HasPrefix(text, "else ") {
			return nil, index, p.err(l.number, "else: must follow an if block at the same indentation")
		}
		if strings.HasPrefix(text, "if ") && strings.HasSuffix(text, ":") {
			body, next, err := p.child(index, indent)
			if err != nil {
				return nil, index, err
			}
			condition, err := parseCondition(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "if "), ":")))
			if err != nil {
				return nil, index, p.err(l.number, err.Error())
			}
			var other []statement
			if next < len(p.lines) && p.lines[next].indent == indent && p.lines[next].text == "else:" {
				other, next, err = p.child(next, indent)
				if err != nil {
					return nil, index, err
				}
			}
			result = append(result, ifStmt{l.number, condition, body, other})
			index = next
			continue
		}
		if strings.HasPrefix(text, "for ") && strings.HasSuffix(text, ":") {
			body, next, err := p.child(index, indent)
			if err != nil {
				return nil, index, err
			}
			name, values, ok := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(text, "for "), ":"), " in ")
			name, values = strings.TrimSpace(name), strings.TrimSpace(values)
			if !ok || name == "" || strings.ContainsAny(name, " \t") || values == "" {
				return nil, index, p.err(l.number, "expected: for name in values:")
			}
			result = append(result, forStmt{l.number, name, values, body})
			index = next
			continue
		}
		if strings.HasPrefix(text, "while ") {
			return nil, index, p.err(l.number, "while is disabled: use a finite for loop")
		}
		if text == "exit" || strings.HasPrefix(text, "exit ") || strings.HasPrefix(text, "exit\t") {
			tokens, err := words(text)
			if err != nil {
				return nil, index, p.err(l.number, err.Error())
			}
			if len(tokens) != 2 || tokens[1].operator {
				return nil, index, p.err(l.number, "expected: exit code (0..255), without pipeline or redirection")
			}
			result = append(result, exitStmt{l.number, tokens[1]})
			index++
			continue
		}
		if text == "inspect" || strings.HasPrefix(text, "inspect ") || text == "require" || strings.HasPrefix(text, "require ") {
			terms, err := words(text)
			if err != nil {
				return nil, index, p.err(l.number, err.Error())
			}
			if err := validateStorageStatement(terms); err != nil {
				return nil, index, p.err(l.number, err.Error())
			}
			result = append(result, storageStmt{l.number, terms})
			index++
			continue
		}
		if strings.HasPrefix(text, "show ") {
			tokens, err := words(text)
			name, ok := "", false
			if len(tokens) > 1 {
				name, ok = variableReference(tokens[1].text)
			}
			if err != nil || len(tokens) < 2 || len(tokens) == 3 || tokens[1].operator || !ok {
				return nil, index, p.err(l.number, "expected: show $process_set [fields field ...]")
			}
			show := showStmt{line: l.number, name: name}
			if len(tokens) > 2 {
				if tokens[2].operator || tokens[2].text != "fields" {
					return nil, index, p.err(l.number, "expected: show process_set fields field ...")
				}
				for _, token := range tokens[3:] {
					if token.operator || token.text == "" {
						return nil, index, p.err(l.number, "process fields must be names")
					}
					show.fields = append(show.fields, token.text)
				}
			}
			result = append(result, show)
			index++
			continue
		}
		if strings.HasPrefix(text, "summarize ") {
			tokens, err := words(text)
			name, ok := "", false
			if len(tokens) > 1 {
				name, ok = variableReference(tokens[1].text)
			}
			if err != nil || len(tokens) != 4 || tokens[1].operator || tokens[2].operator || tokens[3].operator ||
				!ok || tokens[2].text != "memory" || tokens[3].text != "rss" {
				return nil, index, p.err(l.number, "expected: summarize $process_set memory rss")
			}
			result = append(result, summarizeStmt{line: l.number, name: name, metric: tokens[3].text})
			index++
			continue
		}
		if text == "service" || strings.HasPrefix(text, "service ") {
			tokens, err := words(text)
			manager, offset, ok := systemdManager(tokens, 2)
			if err == nil && ok && len(tokens) == offset+5 && builtinKeyword(tokens[0], "service") &&
				builtinKeyword(tokens[1], "wait") && !tokens[offset].operator &&
				builtinKeyword(tokens[offset+1], "until") && !tokens[offset+2].operator &&
				builtinKeyword(tokens[offset+3], "within") && !tokens[offset+4].operator {
				duration, durationErr := time.ParseDuration(tokens[offset+4].text)
				if durationErr != nil || duration <= 0 {
					return nil, index, p.err(l.number, "service wait timeout must be a positive duration")
				}
				result = append(result, serviceWaitStmt{line: l.number, manager: manager, unit: tokens[offset], expectedState: tokens[offset+2].text,
					timeout: duration, timeoutText: tokens[offset+4].text})
				index++
				continue
			}
			if err == nil && ok && len(tokens) == offset+5 && builtinKeyword(tokens[0], "service") &&
				builtinKeyword(tokens[1], "restart") && !tokens[offset].operator &&
				builtinKeyword(tokens[offset+1], "until") && !tokens[offset+2].operator &&
				builtinKeyword(tokens[offset+3], "within") && !tokens[offset+4].operator {
				duration, durationErr := time.ParseDuration(tokens[offset+4].text)
				if durationErr != nil || duration <= 0 {
					return nil, index, p.err(l.number, "service restart timeout must be a positive duration")
				}
				result = append(result, serviceRestartStmt{line: l.number, manager: manager, unit: tokens[offset], expectedState: tokens[offset+2].text,
					timeout: duration, timeoutText: tokens[offset+4].text})
				index++
				continue
			}
			manager, offset, ok = systemdManager(tokens, 2)
			if err != nil || !ok || len(tokens) != offset+1 || !builtinKeyword(tokens[0], "service") ||
				!builtinKeyword(tokens[1], "status") || tokens[offset].operator {
				return nil, index, p.err(l.number, "expected: service status unit")
			}
			result = append(result, serviceStatusStmt{line: l.number, manager: manager, unit: tokens[offset]})
			index++
			continue
		}
		if text == "journal" || strings.HasPrefix(text, "journal ") {
			tokens, err := words(text)
			manager, offset, ok := systemdManager(tokens, 1)
			if err == nil && ok && len(tokens) == offset+5 && builtinKeyword(tokens[0], "journal") &&
				!tokens[offset].operator && builtinKeyword(tokens[offset+1], "since") && !tokens[offset+2].operator &&
				builtinKeyword(tokens[offset+3], "lines") && !tokens[offset+4].operator {
				since, sinceErr := time.ParseDuration(tokens[offset+2].text)
				lines, linesErr := strconv.Atoi(tokens[offset+4].text)
				if sinceErr != nil || since <= 0 || linesErr != nil || lines <= 0 || lines > 10000 {
					return nil, index, p.err(l.number, "journal requires a positive duration and 1..10000 lines")
				}
				result = append(result, journalStmt{line: l.number, manager: manager, unit: tokens[offset], since: since, sinceText: tokens[offset+2].text, lines: lines})
				index++
				continue
			}
			return nil, index, p.err(l.number, "expected: journal unit since duration lines count")
		}
		if name, value, ok := assignment(text); ok {
			result = append(result, assignStmt{l.number, name, value})
			index++
			continue
		}
		result = append(result, commandStmt{l.number, text})
		index++
	}
	return result, index, nil
}

// systemdManager accepts an optional profile before the operation's unit.
// Keeping the omitted form as "system" preserves existing scripts while the
// explicit form makes the manager choice visible in review and in --plan.
func systemdManager(tokens []commandToken, unitOffset int) (string, int, bool) {
	if len(tokens) > unitOffset && builtinKeyword(tokens[unitOffset], "system") {
		return "system", unitOffset + 1, true
	}
	if len(tokens) > unitOffset && builtinKeyword(tokens[unitOffset], "user") {
		return "user", unitOffset + 1, true
	}
	return "system", unitOffset, true
}

func builtinKeyword(t commandToken, word string) bool {
	return !t.literal && !t.operator && t.text == word
}

func (p parser) child(index, indent int) ([]statement, int, error) {
	if index+1 >= len(p.lines) || p.lines[index+1].indent <= indent {
		return nil, index, p.err(p.lines[index].number, "block requires an indented body")
	}
	return p.block(index+1, p.lines[index+1].indent)
}

func assignment(s string) (string, string, bool) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	name := strings.TrimSpace(parts[0])
	if name == "" || strings.ContainsAny(name, " \t") {
		return "", "", false
	}
	return name, strings.TrimSpace(parts[1]), true
}
