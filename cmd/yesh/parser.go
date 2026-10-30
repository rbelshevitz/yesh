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
	line       int
	kind       storageKind
	path       commandToken
	source     commandToken
	filesystem commandToken
	mode       string
}

type storageKind int

const (
	storageInspect storageKind = iota
	storageRequireFstab
	storageRequireDevice
)

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
	action        string
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

type retentionPlanStmt struct {
	line       int
	root, keep commandToken
}

func (retentionPlanStmt) stmt() {}

type retentionQuarantineStmt struct {
	line                   int
	root, keep, quarantine commandToken
}

func (retentionQuarantineStmt) stmt() {}

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
		if strings.HasPrefix(text, "rsync retention ") {
			tokens, err := words(text)
			if err != nil {
				return nil, index, p.err(l.number, err.Error())
			}
			if len(tokens) == 9 && tokens[0].text == "rsync" && tokens[1].text == "retention" &&
				tokens[2].text == "quarantine" && tokens[3].text == "in" &&
				tokens[5].text == "keep" && tokens[7].text == "to" &&
				!tokens[0].operator && !tokens[1].operator && !tokens[2].operator &&
				!tokens[3].operator && !tokens[4].operator && !tokens[5].operator &&
				!tokens[6].operator && !tokens[7].operator && !tokens[8].operator {
				result = append(result, retentionQuarantineStmt{line: l.number, root: tokens[4], keep: tokens[6], quarantine: tokens[8]})
				index++
				continue
			}
			if len(tokens) != 7 || tokens[0].operator || tokens[0].text != "rsync" ||
				tokens[1].operator || tokens[1].text != "retention" || tokens[2].operator || tokens[2].text != "plan" ||
				tokens[3].operator || tokens[3].text != "in" || tokens[4].operator || tokens[5].operator ||
				tokens[5].text != "keep" || tokens[6].operator {
				return nil, index, p.err(l.number, "expected: rsync retention plan in root keep count")
			}
			result = append(result, retentionPlanStmt{line: l.number, root: tokens[4], keep: tokens[6]})
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
			if condition.kind == commandCondition {
				if err := validateCommandSyntax(condition.text, p.filename, l.number); err != nil {
					return nil, index, err
				}
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
			item, err := p.parseStorage(text, l.number)
			if err != nil {
				return nil, index, err
			}
			result = append(result, item)
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
			item, err := p.parseService(text, l.number)
			if err != nil {
				return nil, index, err
			}
			result = append(result, item)
			index++
			continue
		}
		if text == "journal" || strings.HasPrefix(text, "journal ") {
			item, err := p.parseJournal(text, l.number)
			if err != nil {
				return nil, index, err
			}
			result = append(result, item)
			index++
			continue
		}
		if name, value, ok := assignment(text); ok {
			result = append(result, assignStmt{l.number, name, value})
			index++
			continue
		}
		if err := validateCommandSyntax(text, p.filename, l.number); err != nil {
			return nil, index, err
		}
		result = append(result, commandStmt{l.number, text})
		index++
	}
	return result, index, nil
}

func (p parser) parseStorage(text string, line int) (storageStmt, error) {
	terms, err := words(text)
	if err != nil {
		return storageStmt{}, p.err(line, err.Error())
	}
	return parseStorageStatement(terms, line)
}

func (p parser) parseService(text string, line int) (statement, error) {
	tokens, err := words(text)
	if err != nil {
		return nil, p.err(line, err.Error())
	}
	manager, offset := systemdManager(tokens, 2)
	if len(tokens) == offset+5 && builtinKeyword(tokens[0], "service") &&
		builtinKeyword(tokens[1], "wait") && !tokens[offset].operator &&
		builtinKeyword(tokens[offset+1], "until") && !tokens[offset+2].operator &&
		builtinKeyword(tokens[offset+3], "within") && !tokens[offset+4].operator {
		duration, durationErr := time.ParseDuration(tokens[offset+4].text)
		if durationErr != nil || duration <= 0 {
			return nil, p.err(line, "service wait timeout must be a positive duration")
		}
		return serviceWaitStmt{line: line, manager: manager, unit: tokens[offset], expectedState: tokens[offset+2].text,
			timeout: duration, timeoutText: tokens[offset+4].text}, nil
	}
	if len(tokens) == offset+5 && builtinKeyword(tokens[0], "service") &&
		(builtinKeyword(tokens[1], "start") || builtinKeyword(tokens[1], "restart")) && !tokens[offset].operator &&
		builtinKeyword(tokens[offset+1], "until") && !tokens[offset+2].operator &&
		builtinKeyword(tokens[offset+3], "within") && !tokens[offset+4].operator {
		duration, durationErr := time.ParseDuration(tokens[offset+4].text)
		if durationErr != nil || duration <= 0 {
			return nil, p.err(line, "service "+tokens[1].text+" timeout must be a positive duration")
		}
		return serviceRestartStmt{line: line, manager: manager, action: tokens[1].text, unit: tokens[offset], expectedState: tokens[offset+2].text,
			timeout: duration, timeoutText: tokens[offset+4].text}, nil
	}
	if len(tokens) != offset+1 || !builtinKeyword(tokens[0], "service") ||
		!builtinKeyword(tokens[1], "status") || tokens[offset].operator {
		return nil, p.err(line, "expected: service status unit")
	}
	return serviceStatusStmt{line: line, manager: manager, unit: tokens[offset]}, nil
}

func (p parser) parseJournal(text string, line int) (statement, error) {
	tokens, err := words(text)
	if err != nil {
		return nil, p.err(line, err.Error())
	}
	manager, offset := systemdManager(tokens, 1)
	if len(tokens) == offset+5 && builtinKeyword(tokens[0], "journal") &&
		!tokens[offset].operator && builtinKeyword(tokens[offset+1], "since") && !tokens[offset+2].operator &&
		builtinKeyword(tokens[offset+3], "lines") && !tokens[offset+4].operator {
		since, sinceErr := time.ParseDuration(tokens[offset+2].text)
		lines, linesErr := strconv.Atoi(tokens[offset+4].text)
		if sinceErr != nil || since <= 0 || linesErr != nil || lines <= 0 || lines > 10000 {
			return nil, p.err(line, "journal requires a positive duration and 1..10000 lines")
		}
		return journalStmt{line: line, manager: manager, unit: tokens[offset], since: since, sinceText: tokens[offset+2].text, lines: lines}, nil
	}
	return nil, p.err(line, "expected: journal unit since duration lines count")
}

// systemdManager accepts an optional profile before the operation's unit.
// Keeping the omitted form as "system" preserves existing scripts while the
// explicit form makes the manager choice visible in review and in --plan.
func systemdManager(tokens []commandToken, unitOffset int) (string, int) {
	if len(tokens) > unitOffset && builtinKeyword(tokens[unitOffset], "system") {
		return "system", unitOffset + 1
	}
	if len(tokens) > unitOffset && builtinKeyword(tokens[unitOffset], "user") {
		return "user", unitOffset + 1
	}
	return "system", unitOffset
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
