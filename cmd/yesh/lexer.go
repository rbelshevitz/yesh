package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type commandToken struct {
	text     string
	literal  bool
	operator bool
}

func words(s string) ([]commandToken, error) {
	var result []commandToken
	var b strings.Builder
	escaped, active, literal := false, false, false
	flush := func() {
		if active {
			result = append(result, commandToken{text: b.String(), literal: literal})
			b.Reset()
			active, literal = false, false
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if escaped {
			b.WriteRune(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped, active, literal = true, true, true
			continue
		}
		if c == '\'' || c == '"' {
			end := i + 1
			for end < len(runes) {
				if runes[end] == c {
					break
				}
				if c == '"' && runes[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(runes) {
				return nil, errors.New("unterminated quote")
			}
			text := string(runes[i+1 : end])
			if c == '"' {
				decoded, err := strconv.Unquote(string(runes[i : end+1]))
				if err != nil {
					return nil, fmt.Errorf("invalid string: %w", err)
				}
				text = decoded
			}
			b.WriteString(text)
			active, literal = true, true
			i = end
			continue
		}
		if c == ' ' || c == '\t' {
			flush()
			continue
		}
		if strings.ContainsRune("|><;&", c) {
			if (c == '>' || c == '<') && active && !literal {
				if _, err := strconv.Atoi(b.String()); err == nil {
					return nil, errors.New("descriptor redirection is disabled")
				}
			}
			flush()
			op := string(c)
			if i+1 < len(runes) && runes[i+1] == c {
				op += string(c)
				i++
			}
			result = append(result, commandToken{text: op, operator: true})
			continue
		}
		if c == '\x00' || c == '\n' || c == '\r' {
			return nil, errors.New("command contains NUL or newline")
		}
		b.WriteRune(c)
		active = true
	}
	if escaped {
		return nil, errors.New("unterminated escape")
	}
	flush()
	return result, nil
}

func validateWords(tokens []commandToken, filename string, line int) error {
	for _, token := range tokens {
		if token.operator {
			switch token.text {
			case "|", "<", ">", ">>":
			default:
				return fmt.Errorf("%s:%d: %q is disabled; use a yesh block", filename, line, token.text)
			}
		}
		if !token.literal && (strings.Contains(token.text, "$(") || strings.Contains(token.text, "`")) {
			return fmt.Errorf("%s:%d: command substitution is disabled", filename, line)
		}
	}
	return nil
}
