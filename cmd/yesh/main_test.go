package main

import "testing"

func TestParserRejectsWhile(t *testing.T) {
	_, err := parse("while true:\n    echo loop\n", "test.yesh")
	if err == nil {
		t.Fatal("while must be rejected")
	}
}

func TestRuntimeRejectsUnsafeCommandForms(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	for _, command := range []string{
		"echo loop &",
		"echo loop |",
		"bash -c echo",
		"echo $(whoami)",
	} {
		if err := r.run(command, 1, true); err == nil {
			t.Errorf("expected %q to be rejected", command)
		}
	}
}

func TestPipelineIsParsedBeforeExecution(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	stages, err := r.parseCommand("first hello | second world > output.txt", 1)
	if err != nil {
		t.Fatalf("pipeline parse failed: %v", err)
	}
	if len(stages) != 2 || stages[0].argv[0] != "first" || stages[1].argv[0] != "second" {
		t.Fatalf("unexpected pipeline: %#v", stages)
	}
	if stages[1].output != "output.txt" {
		t.Fatalf("expected output redirection, got %#v", stages[1])
	}
}

func TestConditionDistinguishesFalseFromExecutionError(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	if ok, err := r.condition("false", 1); err != nil || ok {
		t.Fatalf("false should be a normal false condition, got ok=%v err=%v", ok, err)
	}
	if _, err := r.condition("command-that-does-not-exist", 1); err == nil {
		t.Fatal("missing command must be an execution error, not false")
	}
}

func TestRegexValueMatchesWithoutShell(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	v, err := r.eval(`regex '^api-[0-9]+$'`, 3)
	if err != nil {
		t.Fatalf("regex compile failed: %v", err)
	}
	r.values["name"] = value{scalar: "api-42"}
	r.values["pattern"] = v
	ok, err := r.condition("name matches pattern", 4)
	if err != nil || !ok {
		t.Fatalf("expected regex match, got ok=%v err=%v", ok, err)
	}
}

func TestRegexRejectsInvalidAndDuplicateNames(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	for _, raw := range []string{`regex '['`, `regex '^(?P<x>a)(?P<x>b)$'`} {
		if _, err := r.eval(raw, 7); err == nil {
			t.Errorf("expected regex %q to fail", raw)
		}
	}
}
