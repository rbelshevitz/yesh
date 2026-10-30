package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestConditionGrammarRespectsQuotedData(t *testing.T) {
	for _, tc := range []struct {
		source string
		kind   conditionKind
	}{
		{`echo 'a matches b'`, commandCondition},
		{`printf '%s matches %s' a b`, commandCondition},
		{`echo "exists path"`, commandCondition},
		{`$name matches $pattern`, matchesCondition},
		{`'a matches b' matches $pattern`, matchesCondition},
		{"$name\tmatches\t$pattern", matchesCondition},
		{`$name matches regex '^api'`, matchesCondition},
		{`exists 'a matches b'`, existsCondition},
	} {
		t.Run(tc.source, func(t *testing.T) {
			expr, err := parseCondition(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if expr.kind != tc.kind {
				t.Fatalf("kind=%v want=%v", expr.kind, tc.kind)
			}
		})
	}
}

func TestControlFlowSyntaxErrorsHaveLocations(t *testing.T) {
	for _, tc := range []struct{ source, message string }{
		{"if true\n    echo x\n", "expected ':'"},
		{"if:\n    echo x\n", "requires a condition"},
		{"for item in items\n    echo x\n", "expected ':'"},
		{"for:\n    echo x\n", "for name in values"},
		{"else:\n    echo x\n", "must follow"},
		{"if true:\n    echo x\nelse:\n    echo y\nelse:\n    echo z\n", "must follow"},
		{"if exists(path):\n    echo x\n", "use exists path"},
		{"if exists:\n    echo x\n", "one string"},
		{"if exists a b:\n    echo x\n", "one string"},
		{"if name matches:\n    echo x\n", "text matches regex"},
		{"if name matches pattern | cat:\n    echo x\n", "text matches regex"},
		{"exit\n", "exit code"},
		{"exit 1 | cat\n", "without pipeline"},
		{"exit 0 > file\n", "without pipeline"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			_, err := parse(tc.source, "syntax.yesh")
			if err == nil || !strings.Contains(err.Error(), "syntax.yesh:") || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("wanted %q, got %v", tc.message, err)
			}
		})
	}
}

func TestExistsRequiresPathAndPreservesErrors(t *testing.T) {
	r := &runtime{values: map[string]value{"paths": {isList: true, list: []string{"a"}}, "empty": {}, "nul": {scalar: "a\x00b"}}, filename: "exists.yesh"}
	dir := t.TempDir()
	name := filepath.Join(dir, "a matches b")
	if err := os.WriteFile(name, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	r.values["path"] = value{scalar: name}
	for _, source := range []string{"exists $path", "exists '" + name + "'"} {
		if ok, err := r.condition(source, 4); err != nil || !ok {
			t.Fatalf("%s: %v %v", source, ok, err)
		}
	}
	r.values["missing"] = value{scalar: filepath.Join(dir, "missing")}
	if ok, err := r.condition("exists $missing", 4); err != nil || ok {
		t.Fatalf("missing: %v %v", ok, err)
	}
	for _, source := range []string{"exists $paths", "exists $empty", "exists $nul", `exists ""`} {
		if _, err := r.condition(source, 4); err == nil || !strings.Contains(err.Error(), "exists.yesh:4:") {
			t.Fatalf("%s: %v", source, err)
		}
	}
}

func TestMatchesLiteralAndInlineRegex(t *testing.T) {
	r := &runtime{values: map[string]value{"text": {scalar: "api-42"}}, filename: "matches.yesh"}
	for _, source := range []string{`$text matches regex '^api-[0-9]+$'`, `'a matches b' matches regex '^a matches b$'`} {
		if ok, err := r.condition(source, 1); err != nil || !ok {
			t.Fatalf("%s: %v %v", source, ok, err)
		}
	}
	if _, err := r.condition(`text matches 'api-42'`, 1); err == nil {
		t.Fatal("string accepted in place of regex")
	}
}

func TestExitStopsNestedExecution(t *testing.T) {
	for _, source := range []string{
		"exit 0\nafter = bad\n",
		"if true:\n    exit 7\nafter = bad\n",
		"for code in [3,4]:\n    exit $code\nafter = bad\n",
	} {
		t.Run(source, func(t *testing.T) {
			ast, err := parse(source, "exit.yesh")
			if err != nil {
				t.Fatal(err)
			}
			r := &runtime{values: map[string]value{}, filename: "exit.yesh"}
			err = r.execute(ast)
			var exit scriptExit
			if !errors.As(err, &exit) {
				t.Fatalf("exit was not control flow: %v", err)
			}
			if _, ok := r.values["after"]; ok {
				t.Fatal("execution continued after exit")
			}
		})
	}
	for _, source := range []string{"exit -1", "exit 256", "exit 1.5", "exit [1]", `exit ""`, "exit 999999999999999999999999999999"} {
		t.Run(source, func(t *testing.T) {
			ast, err := parse(source, "exit.yesh")
			if err != nil {
				t.Fatal(err)
			}
			r := &runtime{values: map[string]value{}, filename: "exit.yesh"}
			err = r.execute(ast)
			var exit scriptExit
			if err == nil || errors.As(err, &exit) {
				t.Fatalf("invalid exit accepted: %v", err)
			}
		})
	}
}

func TestExitPlanDoesNotStop(t *testing.T) {
	ast, err := parse("exit 7\nafter = planned\n", "plan.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r := &runtime{values: map[string]value{}, filename: "plan.yesh", plan: true}
	if err := r.execute(ast); err != nil {
		t.Fatal(err)
	}
	if r.values["after"].scalar != "planned" {
		t.Fatal("plan stopped at exit")
	}
}

func FuzzConditionQuotedData(f *testing.F) {
	for _, text := range []string{"a matches b", "exists(path)", "| sh", "", "a\nb", "שלום", `"matches"`} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 4096 {
			t.Skip()
		}
		source := "printf '%s' " + strconv.Quote(text)
		expr, err := parseCondition(source)
		if err != nil {
			t.Fatal(err)
		}
		if expr.kind != commandCondition {
			t.Fatalf("quoted data became a predicate: %q", source)
		}
		if expr.text != source {
			t.Fatal("command source was rewritten")
		}
	})
}
