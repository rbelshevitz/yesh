package main

import "testing"

func TestProcessQueryAndShowParseAsLanguageStatements(t *testing.T) {
	items, err := parse("nginx = processes where name is \"nginx\"\nshow nginx fields pid rss cgroup\nsummarize nginx memory rss\n", "processes.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d statements", len(items))
	}
	if _, ok := items[0].(assignStmt); !ok {
		t.Fatalf("first statement is %T", items[0])
	}
	show, ok := items[1].(showStmt)
	if !ok || show.name != "nginx" || len(show.fields) != 3 || show.fields[1] != "rss" {
		t.Fatalf("unexpected show statement: %#v", items[1])
	}
	if summary, ok := items[2].(summarizeStmt); !ok || summary.name != "nginx" || summary.metric != "rss" {
		t.Fatalf("unexpected summarize statement: %#v", items[2])
	}
}
