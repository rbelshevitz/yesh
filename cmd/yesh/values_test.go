package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestStringsHaveOneMeaningAcrossContexts(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "strings.yesh"}
	for _, text := range []string{"", "a\nb", "a\tb", `a\b`, `a"b`, "a'b", "שלום", "| ; $(id)", "a,b", "a  b"} {
		t.Run(strconv.Quote(text), func(t *testing.T) {
			source := strconv.Quote(text)
			v, err := r.eval(source, 7)
			if err != nil {
				t.Fatal(err)
			}
			stages, err := r.parseCommand("printf "+source, 7)
			if err != nil {
				t.Fatal(err)
			}
			list, err := r.eval("["+source+"]", 7)
			if err != nil {
				t.Fatal(err)
			}
			if v.scalar != text || !reflect.DeepEqual(stages[0].argv, []string{"printf", text}) || !reflect.DeepEqual(list.list, []string{text}) {
				t.Fatalf("inconsistent literal: value=%#v stages=%#v list=%#v", v, stages, list)
			}
		})
	}
}

func TestListsPreserveQuotedDelimiters(t *testing.T) {
	r := &runtime{values: map[string]value{"item": {scalar: "resolved"}}, filename: "lists.yesh"}
	for _, tc := range []struct {
		source string
		want   []string
	}{
		{`["a,b", "c"]`, []string{"a,b", "c"}},
		{`['a,b', 'c']`, []string{"a,b", "c"}},
		{`["[a,b]", "]", "["]`, []string{"[a,b]", "]", "["}},
		{`["a\"b,c", 'd']`, []string{"a\"b,c", "d"}},
		{`["", '', item]`, []string{"", "", "resolved"}},
		{`['a  b', "c  d"]`, []string{"a  b", "c  d"}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			v, err := r.eval(tc.source, 3)
			if err != nil {
				t.Fatal(err)
			}
			if !v.isList || !reflect.DeepEqual(v.list, tc.want) {
				t.Fatalf("got %#v want %q", v, tc.want)
			}
		})
	}
	for _, source := range []string{`["unterminated]`, `[one`, `[one,]`, `[,one]`, `[one,,two]`, `[[one,two],three]`, `[one],two]`, `"bad\q"`, `'unterminated`} {
		t.Run(source, func(t *testing.T) {
			if _, err := r.eval(source, 11); err == nil {
				t.Fatalf("accepted %s", source)
			} else if !strings.Contains(err.Error(), "lists.yesh:11:") {
				t.Fatal(err)
			}
		})
	}
}

func TestForPreservesWhitespaceInsideValues(t *testing.T) {
	items, err := parse("for item in ['a  b', 'c,d']:\n    echo item\n", "for.yesh")
	if err != nil {
		t.Fatal(err)
	}
	loop, ok := items[0].(forStmt)
	if !ok {
		t.Fatalf("%#v", items)
	}
	r := &runtime{values: map[string]value{}, filename: "for.yesh"}
	v, err := r.eval(loop.values, 1)
	if err != nil || !reflect.DeepEqual(v.list, []string{"a  b", "c,d"}) {
		t.Fatalf("loop changed values: %#v %v", v, err)
	}
}

func TestParserRejectsTabIndentation(t *testing.T) {
	for _, source := range []string{"\techo x\n", "if true:\n\techo x\n", "if true:\n  \techo x\n"} {
		if _, err := parse(source, "tabs.yesh"); err == nil {
			t.Fatalf("tab indentation accepted: %q", source)
		}
	}
}
