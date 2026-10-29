package main

import (
	"testing"
)

func TestParserAcceptsServiceStatus(t *testing.T) {
	items, err := parse("unit = nginx.service\nservice status $unit\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected assignment and service status, got %#v", items)
	}
	if _, ok := items[1].(serviceStatusStmt); !ok {
		t.Fatalf("expected service status statement, got %#v", items[1])
	}
}

func TestParserAcceptsServiceRestartWithPostcondition(t *testing.T) {
	items, err := parse("service restart nginx.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(serviceRestartStmt)
	if !ok || stmt.expectedState != "active" || stmt.timeoutText != "2s" {
		t.Fatalf("unexpected restart statement: %#v", items[0])
	}
}

func TestParserAcceptsServiceStartWithPostcondition(t *testing.T) {
	items, err := parse("service start nginx.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(serviceRestartStmt)
	if !ok || stmt.action != "start" || stmt.expectedState != "active" || stmt.timeoutText != "2s" {
		t.Fatalf("unexpected start statement: %#v", items[0])
	}
}

func TestParserAcceptsServiceWaitWithPostcondition(t *testing.T) {
	items, err := parse("service wait user api.service until active within 2s\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(serviceWaitStmt)
	if !ok || stmt.manager != "user" || stmt.expectedState != "active" || stmt.timeoutText != "2s" {
		t.Fatalf("unexpected wait statement: %#v", items[0])
	}
}

func TestParserAcceptsBoundedJournalQuery(t *testing.T) {
	items, err := parse("journal nginx.service since 15m lines 20\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(journalStmt)
	if !ok || stmt.sinceText != "15m" || stmt.lines != 20 {
		t.Fatalf("unexpected journal statement: %#v", items[0])
	}
}

func TestParserAcceptsExplicitSystemdManagers(t *testing.T) {
	items, err := parse("service status user api.service\njournal system api.service since 15m lines 2\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	status, ok := items[0].(serviceStatusStmt)
	if !ok || status.manager != "user" {
		t.Fatalf("unexpected user manager statement: %#v", items[0])
	}
	journal, ok := items[1].(journalStmt)
	if !ok || journal.manager != "system" {
		t.Fatalf("unexpected system manager statement: %#v", items[1])
	}
}

func TestParserRejectsAmbiguousServiceStatus(t *testing.T) {
	for _, source := range []string{
		"service\n",
		"service start nginx.service\n",
		"service status\n",
		"service status nginx.service extra\n",
		"service status nginx.service | cat\n",
		"service restart nginx.service until active within 0s\n",
		"service restart nginx.service until active within forever\n",
		"service wait nginx.service until active within 0s\n",
		"service wait nginx.service until active within forever\n",
		"journal nginx.service since 0s lines 20\n",
		"journal nginx.service since 15m lines 0\n",
		"journal nginx.service since 15m lines 10001\n",
		"journal nginx.service since 15m\n",
	} {
		if _, err := parse(source, "service.yesh"); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}

func TestServiceStatusUnitValidation(t *testing.T) {
	for _, unit := range []string{"", "api", "-bad.service", "path/bad.service", "*.service", "api service", "api@.service"} {
		if err := validateUnitName(unit); err == nil {
			t.Errorf("accepted invalid unit %q", unit)
		}
	}
	if err := validateUnitName("api@blue.service"); err != nil {
		t.Fatal(err)
	}
}

func TestParseServiceStatus(t *testing.T) {
	got, err := parseServiceStatus([]byte("Id=nginx.service\nLoadState=loaded\nActiveState=active\nSubState=running\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "nginx.service" || got.LoadState != "loaded" || got.ActiveState != "active" || got.SubState != "running" {
		t.Fatalf("unexpected status: %#v", got)
	}
	for _, data := range []string{
		"Id=nginx.service\nLoadState=loaded\nActiveState=active\n",
		"Id=nginx.service\nLoadState=loaded\nActiveState=active\nSubState=running\nExtra=value\n",
		"Id=nginx.service\nId=other.service\nLoadState=loaded\nActiveState=active\nSubState=running\n",
	} {
		if _, err := parseServiceStatus([]byte(data)); err == nil {
			t.Errorf("accepted invalid status %q", data)
		}
	}
}

func TestParseServiceRestartSnapshotFields(t *testing.T) {
	got, err := parseServiceStatus([]byte("Id=api.service\nLoadState=loaded\nActiveState=active\nSubState=running\nResult=success\nExecMainCode=exited\nExecMainStatus=0\nActiveEnterTimestampMonotonic=123456\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != "success" || got.ExecMainCode != "exited" || got.ExecMainStatus != "0" || got.ActiveEnterTimestampMonotonic != "123456" {
		t.Fatalf("unexpected restart snapshot: %#v", got)
	}
}

func TestServiceStatusPlanDoesNotQuerySystemctl(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r := &runtime{values: map[string]value{}, filename: "service.yesh", plan: true}
	items, err := parse("service status nginx.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
}
