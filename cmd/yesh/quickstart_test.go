package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func yeshBlocks(t *testing.T, filename string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", filename))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	var blocks []string
	for {
		_, rest, found := strings.Cut(text, "```yesh\n")
		if !found {
			break
		}
		block, remaining, closed := strings.Cut(rest, "\n```")
		if !closed {
			t.Fatalf("unclosed yesh block in %s", filename)
		}
		blocks = append(blocks, strings.TrimSpace(block))
		text = remaining
	}
	return blocks
}

func TestQuickstartExamplesMatchAndParse(t *testing.T) {
	en, ru := yeshBlocks(t, "quickstart.md"), yeshBlocks(t, "quickstart.ru.md")
	if len(en) != 4 {
		t.Fatalf("quickstart scenarios changed; update execution coverage: %d", len(en))
	}
	if !reflect.DeepEqual(en, ru) {
		t.Fatal("English and Russian executable examples differ")
	}
	for i, source := range en {
		if _, err := parse(source+"\n", "quickstart"); err != nil {
			t.Fatalf("block %d: %v", i+1, err)
		}
	}
	example, err := os.ReadFile(filepath.Join("..", "..", "examples", "service-check.yesh"))
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(source string) string {
		var lines []string
		for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
			if trim := strings.TrimSpace(line); trim != "" && !strings.HasPrefix(trim, "#") {
				lines = append(lines, strings.TrimRight(line, " \t"))
			}
		}
		return strings.Join(lines, "\n")
	}
	if normalize(en[0]) != normalize(string(example)) {
		t.Fatal("quickstart service check differs from runnable example")
	}
}

func TestSyntaxGuideExamplesParse(t *testing.T) {
	blocks := yeshBlocks(t, "syntax-guide.ru.md")
	if len(blocks) != 17 {
		t.Fatalf("syntax guide examples changed; update execution coverage: %d", len(blocks))
	}
	for i, source := range blocks {
		if _, err := parse(source+"\n", "syntax-guide.ru.md"); err != nil {
			t.Fatalf("block %d: %v", i+1, err)
		}
	}
}
