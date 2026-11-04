//go:build navigator

package main

import (
	"errors"
	"os"
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func assertNavigatorPrivateMode(t *testing.T, info os.FileInfo) {
	t.Helper()
	if goRuntime.GOOS == "windows" {
		// Windows does not expose POSIX permission bits through FileMode.Perm.
		return
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("saved mode = %o, want 600", info.Mode().Perm())
	}
}

func TestNavigatorMainMenuHasOnlyImplementedActivities(t *testing.T) {
	activities := navigatorActivities()
	if len(activities) != 3 {
		t.Fatalf("activity count = %d, want backup, service inspection, and quit", len(activities))
	}
	if activities[0].name != "Create a backup script" || activities[1].name != "Inspect a service" || activities[2].name != "Quit" {
		t.Fatalf("activities = %#v", activities)
	}
}

func TestNavigatorServiceCompletionUsesFullUnitName(t *testing.T) {
	services := []navigatorService{{unit: "example.service"}, {unit: "worker.service"}}
	model := navigatorModel{page: navigatorServicePicker, serviceReady: true, services: services, serviceQuery: "exa"}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if model.serviceQuery != "example.service" {
		t.Fatalf("completed query = %q, want example.service", model.serviceQuery)
	}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.serviceUnit != "example.service" || model.page != navigatorServicePlan {
		t.Fatalf("selected service = %q, want example.service", model.serviceUnit)
	}
}

func TestNavigatorServicePlansAreParseable(t *testing.T) {
	for plan := 0; plan < 3; plan++ {
		script, errText := navigatorServicePlanScript("system", "example.service", plan, "30s", "5s", "24h", "1h")
		if errText != nil {
			t.Fatalf("plan %d failed: %v", plan, errText)
		}
		if script == "" {
			t.Fatalf("plan %d produced empty script", plan)
		}
		if _, err := parse(script, "navigator-service.yesh"); err != nil {
			t.Fatalf("plan %d does not parse: %v", plan, err)
		}
	}
}

func TestRenderPlanUsesParsedASTAndDoesNotRunCommands(t *testing.T) {
	items, err := parse("echo do-not-run\n", "navigator-plan.yesh")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	plan, err := renderPlan(items, "navigator-plan.yesh")
	if err != nil {
		t.Fatalf("render plan: %v", err)
	}
	if !strings.Contains(plan, "PLAN") || !strings.Contains(plan, "do-not-run") {
		t.Fatalf("plan = %q", plan)
	}
}

func TestRenderPlanKeepsLoopOutputInTheCapturedWriter(t *testing.T) {
	items, err := parse("items = [one, two]\nfor item in $items:\n    echo $item\n", "navigator-loop.yesh")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	plan, err := renderPlan(items, "navigator-loop.yesh")
	if err != nil {
		t.Fatalf("render plan: %v", err)
	}
	if !strings.Contains(plan, "PLAN [\"echo\" \"one\"]") || !strings.Contains(plan, "PLAN [\"echo\" \"two\"]") {
		t.Fatalf("loop plan = %q", plan)
	}
}

func TestNavigatorMeasurementSuggestionIsEditable(t *testing.T) {
	model := navigatorModel{serviceSuggestion: 0}
	navigatorApplyMeasurementSuggestion(&model)
	first := model.serviceDuration + "/" + model.serviceInterval + "/" + model.serviceKeep
	model.serviceSuggestion = 1
	navigatorApplyMeasurementSuggestion(&model)
	second := model.serviceDuration + "/" + model.serviceInterval + "/" + model.serviceKeep
	if first == second || second != "2m/10s/24h" {
		t.Fatalf("suggestions = %q then %q", first, second)
	}
}

func TestNavigatorServicePlanRefreshesWhenChoiceChanges(t *testing.T) {
	model := navigatorModel{
		page:             navigatorServicePlan,
		serviceManager:   "system",
		serviceUnit:      "example.service",
		serviceDuration:  "30s",
		serviceInterval:  "5s",
		serviceKeep:      "24h",
		serviceSince:     "1h",
		servicePlan:      0,
		servicePlanFocus: 0,
	}
	navigatorRefreshServiceScript(&model)
	updateNavigatorServicePlan(&model, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if model.servicePlan != 1 || !strings.Contains(model.serviceScript, "measure service") {
		t.Fatalf("plan/script = %d/%q", model.servicePlan, model.serviceScript)
	}
	if model.serviceError != nil || !strings.Contains(model.servicePlanOutput, "PLAN measure service") {
		t.Fatalf("service plan output/error = %q/%v", model.servicePlanOutput, model.serviceError)
	}
}

func TestNavigatorShowsSavedMeasurementsOnlyWhenHistoryExists(t *testing.T) {
	model := navigatorModel{
		page:             navigatorServicePlan,
		serviceHistoryOK: true,
		serviceHistory: navigatorMeasurementHistory{
			path:  "/tmp/api.service.jsonl",
			count: 1,
			first: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			last:  time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC),
		},
		servicePlanFocus: 5,
	}
	updateNavigatorServicePlan(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.page != navigatorServiceHistory {
		t.Fatalf("page = %d, want saved measurements page", model.page)
	}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if model.page != navigatorServicePlan {
		t.Fatalf("after escape page = %d, want service plan", model.page)
	}
}

func TestNavigatorServiceReviewIsOptionalAndUsesSeparateScreen(t *testing.T) {
	model := navigatorModel{
		page:              navigatorServicePlan,
		servicePlanFocus:  5,
		serviceScript:     "service resources system example.service\n",
		servicePlanOutput: "PLAN [\"systemctl\"]\n",
	}
	updateNavigatorServicePlan(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.page != navigatorServiceReview {
		t.Fatalf("page = %d, want service review", model.page)
	}
	if model.serviceReviewTab != 0 {
		t.Fatalf("review tab = %d, want plan tab", model.serviceReviewTab)
	}
}

func TestNavigatorRejectsBadInputWithoutRepairLoop(t *testing.T) {
	model := navigatorModel{
		page:             navigatorServicePlan,
		serviceManager:   "system",
		serviceUnit:      "example.service",
		serviceDuration:  "not-a-duration",
		serviceInterval:  "5s",
		serviceKeep:      "24h",
		serviceSince:     "1h",
		servicePlan:      1,
		servicePlanFocus: 1,
	}
	updateNavigatorServicePlan(&model, tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if model.serviceRefreshes != 1 {
		t.Fatalf("refresh count = %d, want exactly one pass", model.serviceRefreshes)
	}
	if model.serviceError == nil {
		t.Fatal("invalid input was silently repaired or accepted")
	}
}

func TestNavigatorServicePlanKeepsSelectedManager(t *testing.T) {
	script, errText := navigatorServicePlanScript("user", "example.service", 0, "30s", "5s", "24h", "1h")
	if errText != nil {
		t.Fatalf("user plan failed: %v", errText)
	}
	if !strings.Contains(script, "service resources user example.service") {
		t.Fatalf("user manager missing from script:\n%s", script)
	}
}

func TestNavigatorServiceNameIgnoresSpaces(t *testing.T) {
	model := navigatorModel{page: navigatorServicePicker, serviceReady: true, services: []navigatorService{{unit: "example.service"}}}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if model.serviceQuery != "" {
		t.Fatalf("service query = %q, want empty after whitespace", model.serviceQuery)
	}
}

func TestNavigatorFreezesServiceChoiceWhenSystemdIsUnavailable(t *testing.T) {
	model := navigatorModel{
		page:           navigatorServicePicker,
		serviceReady:   false,
		serviceQuery:   "exa",
		serviceIndex:   0,
		services:       []navigatorService{{unit: "example.service"}},
		serviceMessage: errors.New("systemd is unavailable"),
	}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone))
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.serviceManager != "" || model.serviceQuery != "exa" || model.serviceIndex != 0 || model.page != navigatorServicePicker {
		t.Fatalf("unavailable service picker changed state: %#v", model)
	}
}

func TestParseNavigatorServices(t *testing.T) {
	services := parseNavigatorServices("  example.service loaded active running A web server\n  stopped.service loaded inactive dead Stopped\n")
	if len(services) != 2 || services[0].unit != "example.service" || services[0].description != "A web server" {
		t.Fatalf("services = %#v", services)
	}
}

func TestNavigatorServiceSelectionScrollsWithCursor(t *testing.T) {
	services := make([]navigatorService, 12)
	for index := range services {
		services[index].unit = "service-" + string(rune('a'+index)) + ".service"
	}
	model := navigatorModel{serviceReady: true, services: services, serviceIndex: 0}
	for i := 0; i < 9; i++ {
		updateNavigatorServicePicker(&model, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	if model.serviceIndex != 9 || model.serviceOffset != 2 {
		t.Fatalf("index/offset = %d/%d, want 9/2", model.serviceIndex, model.serviceOffset)
	}
	updateNavigatorServicePicker(&model, tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if model.serviceIndex != 8 || model.serviceOffset != 2 {
		t.Fatalf("after up index/offset = %d/%d, want 8/2", model.serviceIndex, model.serviceOffset)
	}
}

func TestNavigatorTabNeverQuits(t *testing.T) {
	pages := []navigatorPage{navigatorMain, navigatorBackupContext, navigatorBackup, navigatorReview, navigatorServicePicker}
	for _, page := range pages {
		model := navigatorModel{page: page, services: []navigatorService{{unit: "example.service"}}}
		if updateNavigator(&model, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) {
			t.Fatalf("Tab quit on page %d", page)
		}
	}
}

func TestNavigatorMainMenuOpensBackupForm(t *testing.T) {
	model := navigatorModel{}
	if updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) {
		t.Fatal("opening backup unexpectedly quit")
	}
	if model.page != navigatorBackupContext {
		t.Fatalf("page = %d, want backup context page", model.page)
	}
}

func TestNavigatorContextExplainsRemoteChoice(t *testing.T) {
	model := navigatorModel{page: navigatorBackupContext}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if model.sourceKind != 1 {
		t.Fatalf("source kind = %d, want remote", model.sourceKind)
	}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.page != navigatorBackup {
		t.Fatalf("page = %d, want backup page", model.page)
	}
}

func TestNavigatorContextEnterConfirmsChoice(t *testing.T) {
	model := navigatorModel{page: navigatorBackupContext, sourceKind: 1}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.sourceKind != 1 {
		t.Fatalf("source kind = %d, want selected remote kind", model.sourceKind)
	}
	if model.contextFocus != 1 {
		t.Fatalf("context focus = %d, want continue", model.contextFocus)
	}
}

func TestNavigatorParameterHelpExplainsEveryBackupControl(t *testing.T) {
	for field := 0; field <= 6; field++ {
		title, text := navigatorParameterHelp(field)
		if title == "" || text == "" {
			t.Fatalf("field %d has incomplete help: %q / %q", field, title, text)
		}
	}
}

func TestNavigatorMainMenuTabsThroughButtons(t *testing.T) {
	model := navigatorModel{}
	for want := 1; want <= 2; want++ {
		updateNavigator(&model, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		if model.mainFocus != want {
			t.Fatalf("tab focus = %d, want %d", model.mainFocus, want)
		}
	}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if model.mainFocus != 0 {
		t.Fatalf("tab focus after cancel = %d, want menu", model.mainFocus)
	}
}

func TestNavigatorOKDoesNotQuitServiceActivity(t *testing.T) {
	model := navigatorModel{mainIndex: 1, mainFocus: 1}
	if updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) {
		t.Fatal("OK on service activity quit the navigator")
	}
	if model.page != navigatorServicePicker {
		t.Fatalf("page = %d, want service picker", model.page)
	}
}

func TestNavigatorEscapeReturnsToMainMenu(t *testing.T) {
	model := navigatorModel{page: navigatorBackup}
	if updateNavigator(&model, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)) {
		t.Fatal("escape from a child page unexpectedly quit")
	}
	if model.page != navigatorMain {
		t.Fatalf("page = %d, want main page", model.page)
	}
}

func TestNavigatorBackupScriptUsesVisibleAndParseableRsync(t *testing.T) {
	script := navigatorBackupScript(0, "/srv/data", "/backup/current", "/run/lock/backup.lock")
	if _, err := parse(script, "navigator.yesh"); err != nil {
		t.Fatalf("generated backup script does not parse: %v\n%s", err, script)
	}
	for _, want := range []string{
		`source = "/srv/data"`,
		`target = "/backup/current"`,
		`with lock $lock:`,
		`rsync -a --itemize-changes -- $source $target`,
		`exit 2`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("generated script missing %q:\n%s", want, script)
		}
	}
}

func TestNavigatorRemoteBackupSkipsLocalSourceCheck(t *testing.T) {
	script := navigatorBackupScript(1, "backup-host:/srv/data", "/backup/current", "/run/lock/backup.lock")
	if _, err := parse(script, "navigator-remote.yesh"); err != nil {
		t.Fatalf("generated remote backup script does not parse: %v\n%s", err, script)
	}
	if strings.Contains(script, "if exists $source") {
		t.Fatalf("remote backup unexpectedly checks source locally:\n%s", script)
	}
}

func TestNavigatorSourceKindChangesVisibleSourceDefault(t *testing.T) {
	model := navigatorModel{source: "/srv/data"}
	setNavigatorSourceKind(&model, 1)
	if model.source != "backup-host:/srv/data" {
		t.Fatalf("remote source = %q", model.source)
	}
	setNavigatorSourceKind(&model, 0)
	if model.source != "/srv/data" {
		t.Fatalf("local source = %q", model.source)
	}
	model.source = "operator-selected:/data"
	setNavigatorSourceKind(&model, 1)
	if model.source != "operator-selected:/data" {
		t.Fatalf("manual source was overwritten: %q", model.source)
	}
}

func TestNavigatorSourceKindDoesNotInventEmptyPath(t *testing.T) {
	model := navigatorModel{}
	setNavigatorSourceKind(&model, 1)
	if model.source != "" {
		t.Fatalf("empty source became an example path: %q", model.source)
	}
}

func TestSaveNavigatorScriptDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.yesh")
	script := "echo backup\n"
	if err := saveNavigatorScript(path, script); err != nil {
		t.Fatalf("save generated error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved script: %v", err)
	}
	if string(data) != script {
		t.Fatalf("saved script = %q, want %q", data, script)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved script: %v", err)
	}
	assertNavigatorPrivateMode(t, info)
	if err := saveNavigatorScript(path, "changed\n"); err == nil {
		t.Fatal("saving over an existing script unexpectedly succeeded")
	}
}

func TestSaveNavigatorScriptReportsUnavailableDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "backup.yesh")
	errText := saveNavigatorScript(path, "echo backup\n")
	if errText == nil || !strings.Contains(errText.Error(), "cannot save yesh script") {
		t.Fatalf("error = %v, want a clear save error", errText)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected output file after failed save: %v", err)
	}
}

func TestSaveNavigatorScriptTempCreatesPrivateCopy(t *testing.T) {
	path, errText := saveNavigatorScriptTemp("echo backup\n")
	if errText != nil {
		t.Fatalf("temporary save failed: %v", errText)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temporary script: %v", err)
	}
	if string(data) != "echo backup\n" {
		t.Fatalf("temporary script = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat temporary script: %v", err)
	}
	assertNavigatorPrivateMode(t, info)
}

func TestNavigatorBackupRecipeExplainsRemoteBoundary(t *testing.T) {
	recipe := navigatorBackupRecipe{
		sourceKind:  1,
		source:      "backup-host:/srv/data",
		destination: "/backup/current",
		lockPath:    "/run/lock/backup.lock",
		outputPath:  "backup.yesh",
	}
	if err := recipe.validate(); err != nil {
		t.Fatalf("valid recipe rejected: %v", err)
	}
	joined := strings.Join(recipe.plan(), "\n") + "\n" + strings.Join(recipe.warnings(), "\n")
	for _, want := range []string{
		"Leave remote source availability to rsync",
		"A remote source is not checked with local exists.",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("recipe explanation missing %q: %s", want, joined)
		}
	}
}

func TestNavigatorReviewCannotSaveParserError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.yesh")
	model := navigatorModel{
		page:        navigatorReview,
		reviewFocus: 0,
		outputPath:  path,
		script:      "not valid yesh",
		scriptError: errors.New("navigator.yesh:1: invalid script"),
	}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.saved {
		t.Fatal("invalid script was marked saved")
	}
	if model.saveError == nil {
		t.Fatal("save error was not reported")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid script was written: %v", err)
	}
}

func TestNavigatorBackupFlowParsesPlansAndSaves(t *testing.T) {
	output := filepath.Join(t.TempDir(), "backup.yesh")
	model := navigatorModel{
		page:        navigatorBackup,
		backupField: 5,
		source:      "/tmp/source",
		destination: "/tmp/destination",
		lockPath:    "/tmp/backup.lock",
		outputPath:  output,
	}

	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.page != navigatorReview || model.scriptError != nil || model.planError != nil {
		t.Fatalf("generated review is not valid: page=%d script=%v plan=%v", model.page, model.scriptError, model.planError)
	}
	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !model.saved || model.savedPath != output {
		t.Fatalf("generated script was not saved: saved=%v path=%q error=%v", model.saved, model.savedPath, model.saveError)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("saved script missing: %v", err)
	}
	assertNavigatorPrivateMode(t, info)
}

func TestNavigatorBackupFlowRejectsEmptyDestination(t *testing.T) {
	model := navigatorModel{
		page:        navigatorBackup,
		backupField: 5,
		source:      "/tmp/source",
		destination: "",
		lockPath:    "/tmp/backup.lock",
		outputPath:  filepath.Join(t.TempDir(), "backup.yesh"),
	}

	updateNavigator(&model, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.page != navigatorReview {
		t.Fatalf("invalid form did not open review: page=%d", model.page)
	}
	if model.scriptError == nil || !strings.Contains(model.scriptError.Error(), "destination is empty") {
		t.Fatalf("empty destination error = %v", model.scriptError)
	}
	if model.planOutput != "" {
		t.Fatalf("invalid form unexpectedly produced a plan: %q", model.planOutput)
	}
}

func TestNavigatorShowsTooSmallTerminalMessage(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("simulation screen init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(60, 15)
	drawNavigatorTooSmall(screen)
	screen.Show()
	cells, width, height := screen.GetContents()
	if width != 60 || height != 15 {
		t.Fatalf("screen size = %dx%d, want 60x15", width, height)
	}
	var output strings.Builder
	for _, cell := range cells {
		output.WriteString(string(cell.Runes))
	}
	if !strings.Contains(output.String(), "Terminal is too small") {
		t.Fatalf("small terminal message missing from screen: %q", output.String())
	}
}

func TestNavigatorResizeEventKeepsCurrentPage(t *testing.T) {
	model := navigatorModel{page: navigatorBackup, backupField: 2}
	if quit := updateNavigator(&model, tcell.NewEventResize(60, 15)); quit {
		t.Fatal("resize event unexpectedly quit navigator")
	}
	if model.page != navigatorBackup || model.backupField != 2 {
		t.Fatalf("resize changed navigator state: page=%d field=%d", model.page, model.backupField)
	}
}
