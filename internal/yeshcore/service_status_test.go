package yeshcore

import (
	"testing"
	"time"
)

func TestParserAcceptsServiceStatus(t *testing.T) {
	items, err := parse("unit = nginx.service\nservice status $unit\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected assignment and service status, got %#v", items)
	}
	status, ok := items[1].(serviceStatusStmt)
	if !ok || status.manager != "auto" {
		t.Fatalf("expected service status statement, got %#v", items[1])
	}
}

func TestParserAcceptsServiceResources(t *testing.T) {
	items, err := parse("service resources user api.service\n", "service.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(serviceResourcesStmt)
	if !ok || stmt.manager != "user" || stmt.unit.text != "api.service" {
		t.Fatalf("unexpected service resources statement: %#v", items[0])
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
	if !ok || stmt.manager != "system" || stmt.sinceText != "15m" || stmt.lines != 20 {
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

func TestParserAcceptsBoundedMeasurement(t *testing.T) {
	items, err := parse("measure service user api.service for 10s every 2s save to state\n", "measure.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(serviceMeasureStmt)
	if !ok || stmt.manager != "user" || stmt.unit.text != "api.service" || stmt.duration != 10*time.Second || stmt.interval != 2*time.Second || stmt.statePath.text != "state" {
		t.Fatalf("unexpected measurement statement: %#v", items[0])
	}
}

func TestParserAcceptsMeasuredHistoryDirective(t *testing.T) {
	source := "measure service user api.service for 10s every 2s:\n    save to state\n    keep history for 24h\n"
	items, err := parse(source, "measure-history.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(serviceMeasureStmt)
	if !ok || stmt.manager != "user" || stmt.statePath.text != "state" || stmt.keepFor != 24*time.Hour || stmt.keepForText != "24h" {
		t.Fatalf("unexpected measurement history statement: %#v", items[0])
	}
}

func TestParserRejectsInvalidMeasuredHistoryDirective(t *testing.T) {
	for _, source := range []string{
		"measure service api.service for 10s every 2s:\n    keep history for 0s\n",
		"measure service api.service for 10s every 2s:\n    keep history for 24h\n    keep history for 48h\n",
		"measure service api.service for 10s every 2s:\n    save state\n    keep history for 24h\n",
		"measure service api.service for 10s every 2s:\n    save to state\n",
	} {
		if _, err := parse(source, "invalid-measure-history.yesh"); err == nil {
			t.Fatalf("expected parser error for %q", source)
		}
	}
}

func TestParserAcceptsServiceSummary(t *testing.T) {
	items, err := parse("summarize service user api.service from state since 1h\nsummarize service openrc lighttpd from state since 1h\n", "summary.yesh")
	if err != nil {
		t.Fatal(err)
	}
	stmt, ok := items[0].(serviceSummarizeStmt)
	if !ok || stmt.manager != "user" || stmt.unit.text != "api.service" || stmt.statePath.text != "state" || stmt.since != time.Hour {
		t.Fatalf("unexpected service summary statement: %#v", items[0])
	}
	openrc, ok := items[1].(serviceSummarizeStmt)
	if !ok || openrc.manager != "openrc" || openrc.unit.text != "lighttpd" || openrc.statePath.text != "state" || openrc.since != time.Hour {
		t.Fatalf("unexpected OpenRC service summary statement: %#v", items[1])
	}
}

func TestParserAcceptsDefaultServiceResourceStatePath(t *testing.T) {
	items, err := parse("measure service api.service for 10s every 2s\nsummarize service api.service since 1h\n", "default-state.yesh")
	if err != nil {
		t.Fatal(err)
	}
	measure, ok := items[0].(serviceMeasureStmt)
	if !ok || measure.statePath.text != "" {
		t.Fatalf("unexpected default measurement statement: %#v", items[0])
	}
	summary, ok := items[1].(serviceSummarizeStmt)
	if !ok || summary.statePath.text != "" {
		t.Fatalf("unexpected default summary statement: %#v", items[1])
	}
}

func TestParserRejectsAmbiguousServiceStatus(t *testing.T) {
	for _, source := range []string{
		"service\n",
		"service start nginx.service\n",
		"service status\n",
		"service status nginx.service extra\n",
		"service resources nginx.service extra\n",
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

func TestParseOpenRCServiceStatements(t *testing.T) {
	items, err := parse("service status openrc sshd\nservice resources openrc sshd\nmeasure service openrc sshd for 2s every 1s:\n    keep history for 1h\n", "openrc.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d statements, want 3", len(items))
	}
	status, ok := items[0].(serviceStatusStmt)
	if !ok || status.manager != "openrc" || status.unit.text != "sshd" {
		t.Fatalf("unexpected OpenRC status: %#v", items[0])
	}
	resources, ok := items[1].(serviceResourcesStmt)
	if !ok || resources.manager != "openrc" || resources.unit.text != "sshd" {
		t.Fatalf("unexpected OpenRC resources: %#v", items[1])
	}
	measure, ok := items[2].(serviceMeasureStmt)
	if !ok || measure.manager != "openrc" || measure.unit.text != "sshd" {
		t.Fatalf("unexpected OpenRC measurement: %#v", items[2])
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
