package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPipelinePreflightRejectsUnsafeForms(t *testing.T) {
	r := &runtime{values: map[string]value{
		"shell": {scalar: "/bin/sh"}, "command": {list: []string{"/bin/bash", "-c", "echo unsafe"}},
		"empty": {isList: true}, "paths": {isList: true, list: []string{"a", "b"}},
		"nul": {scalar: "a\x00b"},
	}, filename: "safety.yesh"}
	for _, command := range []string{
		"", "| cat", "cat |", "cat || cat", "cat | | cat", "cat ; cat", "cat && cat", "cat &", "cat << input",
		"cat >", "cat > | cat", "cat > > out", "cat < > out", "cat > a > b", "cat < a < b", "cat >> a > b",
		"cat > a | cat", "cat | cat < a", "cat > $empty", "cat > $paths", `cat > ""`, "$empty", `"" arg`, "cat $nul", "cat > $nul",
		"cat 2> out", "cat 2>> out", "cat 0< input", "cat $(id)", "cat `id`", "cat 'unterminated", "cat \\",
		"sh script", "bash -c echo", "/bin/sh -c echo", "cat | /bin/sh", "cat | $shell -c echo", "$command",
		`cat | "sh"`, "cat | /usr/bin/python3.12", "cat | perl", "cat | ruby", "cat | node", "cat | php", "cat | lua",
		"cat | xargs rm", "cat | env sh", "env sh | cat", "cat | sudo sh", "cat | busybox sh", "cat | timeout 1 sh",
		"cat | nohup sh", "cat | setsid sh", "cat | nice sh", "cat | doas sh", "cat | command sh", "cat | eval",
		"cat\x00x", "cat\nother", "cat\rother",
		strings.Repeat("cat | ", maxPipelineStages) + "cat",
	} {
		t.Run(command, func(t *testing.T) {
			if _, err := r.parseCommand(command, 17); err == nil {
				t.Fatalf("accepted %q", command)
			} else if !strings.Contains(err.Error(), "safety.yesh:17:") {
				t.Fatalf("missing source location: %v", err)
			}
		})
	}
}

func TestPipelineArgumentsRemainData(t *testing.T) {
	r := &runtime{values: map[string]value{
		"service": {scalar: "api server"}, "items": {isList: true, list: []string{"one two", "|", "$(id)", ""}},
		"injection": {scalar: "x | sh ; touch marker"}, "empty": {isList: true},
	}, filename: "argv.yesh"}
	for _, tc := range []struct {
		source string
		want   []string
	}{
		{`echo $service "service" 'service'`, []string{"echo", "api server", "service", "service"}},
		{`printf $items`, []string{"printf", "one two", "|", "$(id)", ""}},
		{`printf $injection`, []string{"printf", "x | sh ; touch marker"}},
		{`printf "" '' $empty`, []string{"printf", "", ""}},
		{`printf '|' '>' '<' '>>' ';' '&' '&&' '||'`, []string{"printf", "|", ">", "<", ">>", ";", "&", "&&", "||"}},
		{"printf \\| \\> \\;", []string{"printf", "|", ">", ";"}},
		{"printf '$(id)' '`id`' '*?[a]'", []string{"printf", "$(id)", "`id`", "*?[a]"}},
		{`printf "items"`, []string{"printf", "items"}},
		{`printf a" b"c`, []string{"printf", "a bc"}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			stages, err := r.parseCommand(tc.source, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(stages) != 1 || !reflect.DeepEqual(stages[0].argv, tc.want) {
				t.Fatalf("got %#v, want %#v", stages, tc.want)
			}
		})
	}
}

func TestBareWordsStayLiteralAndVariablesAreExplicit(t *testing.T) {
	r := &runtime{values: map[string]value{"service": {scalar: "api server"}}, filename: "variables.yesh"}
	for _, tc := range []struct {
		source string
		want   []string
	}{
		{`echo service`, []string{"echo", "service"}},
		{`echo $service`, []string{"echo", "api server"}},
		{`echo "$service"`, []string{"echo", "$service"}},
	} {
		stages, err := r.parseCommand(tc.source, 1)
		if err != nil || !reflect.DeepEqual(stages[0].argv, tc.want) {
			t.Fatalf("%s: %#v %v", tc.source, stages, err)
		}
	}
	for _, source := range []string{`echo $missing`, `echo ${service}`, `echo $9bad`} {
		if _, err := r.parseCommand(source, 1); err == nil {
			t.Fatalf("accepted invalid variable reference %q", source)
		}
	}
}

func TestPipelineAppendAndLimit(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "parse.yesh"}
	for _, source := range []string{`cat<input>>output`, `cat < 'input file' >> 'output file'`} {
		stages, err := r.parseCommand(source, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(stages) != 1 || !stages[0].append || stages[0].input == "" || stages[0].output == "" {
			t.Fatalf("lost redirection: %#v", stages)
		}
	}
	source := strings.Repeat("cat | ", maxPipelineStages-1) + "cat"
	if stages, err := r.parseCommand(source, 1); err != nil || len(stages) != maxPipelineStages {
		t.Fatalf("boundary: %d %v", len(stages), err)
	}
}

func FuzzPipelinePreflight(f *testing.F) {
	for _, seed := range []string{"cat | cat > out", `printf "|"`, "cat || sh", "cat >> out", "cat > > out", "''", "cat\x00x"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 16384 {
			t.Skip()
		}
		r := &runtime{values: map[string]value{}, filename: "fuzz.yesh"}
		stages, err := r.parseCommand(source, 1)
		if err != nil {
			return
		}
		if len(stages) == 0 || len(stages) > maxPipelineStages {
			t.Fatal("empty accepted pipeline")
		}
		for i, s := range stages {
			if len(s.argv) == 0 || s.argv[0] == "" {
				t.Fatal(fmt.Sprint(stages))
			}
			if i > 0 && s.input != "" || i < len(stages)-1 && s.output != "" {
				t.Fatal("ignored intermediate redirect")
			}
			for _, arg := range s.argv {
				if strings.ContainsRune(arg, 0) {
					t.Fatal("NUL accepted")
				}
			}
		}
	})
}

func FuzzLiteralArgumentRoundTrip(f *testing.F) {
	for _, arg := range []string{"", "'", `\`, "| sh > file", "service", "$(id)", "a b", "שלום"} {
		f.Add(arg)
	}
	f.Fuzz(func(t *testing.T, arg string) {
		if len(arg) > 16384 || !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
			t.Skip()
		}
		source := "printf '" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		r := &runtime{values: map[string]value{"service": {scalar: "should not expand"}}, filename: "fuzz.yesh"}
		stages, err := r.parseCommand(source, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(stages) != 1 || !reflect.DeepEqual(stages[0].argv, []string{"printf", arg}) {
			t.Fatalf("literal changed: %#v", stages)
		}
	})
}
