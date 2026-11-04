//go:build navigator

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

type navigatorPage uint8

const (
	navigatorMain navigatorPage = iota
	navigatorBackupContext
	navigatorBackup
	navigatorReview
	navigatorServicePicker
	navigatorServicePlan
	navigatorServiceHistory
	navigatorServiceReview
)

type navigatorService struct {
	unit        string
	active      string
	sub         string
	description string
}

type navigatorMeasurementSuggestion struct {
	name     string
	duration string
	interval string
	keep     string
	reason   string
}

var navigatorMeasurementSuggestions = []navigatorMeasurementSuggestion{
	{name: "Quick check", duration: "30s", interval: "5s", keep: "24h", reason: "A short look at current resource behaviour."},
	{name: "Short diagnosis", duration: "2m", interval: "10s", keep: "24h", reason: "Enough time to see a small burst or restart."},
	{name: "Long observation", duration: "10m", interval: "30s", keep: "7d", reason: "Useful for intermittent load without constant polling."},
}

type navigatorActivity struct {
	name string
}

type navigatorModel struct {
	page                 navigatorPage
	mainIndex            int
	mainFocus            int
	contextFocus         int
	backupField          int
	sourceKind           int
	sourceKindOpen       bool
	reviewFocus          int
	source               string
	destination          string
	lockPath             string
	outputPath           string
	script               string
	scriptError          error
	planOutput           string
	planError            error
	saveError            error
	saved                bool
	savedPath            string
	savedTemporary       bool
	serviceQuery         string
	serviceIndex         int
	serviceOffset        int
	services             []navigatorService
	serviceManager       string
	serviceUnit          string
	serviceMessage       error
	serviceReady         bool
	servicePlan          int
	servicePlanFocus     int
	serviceSuggestion    int
	serviceDuration      string
	serviceInterval      string
	serviceKeep          string
	serviceSince         string
	serviceScript        string
	servicePlanOutput    string
	serviceRefreshes     int
	serviceError         error
	serviceOutput        string
	serviceSaved         bool
	serviceSavedPath     string
	serviceSaveError     error
	serviceHistory       navigatorMeasurementHistory
	serviceHistoryOK     bool
	serviceHistoryErr    error
	serviceHistoryOffset int
	serviceReviewTab     int
	serviceReviewOffset  int
	serviceReviewFocus   int
}

func navigatorActivities() []navigatorActivity {
	return []navigatorActivity{{name: "Create a backup script"}, {name: "Inspect a service"}, {name: "Quit"}}
}

var (
	navigatorDesktop     = tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)
	navigatorDialog      = tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
	navigatorFrame       = navigatorDialog.Foreground(tcell.ColorBlack)
	navigatorTitle       = navigatorDialog.Foreground(tcell.ColorRed).Bold(true)
	navigatorLabel       = navigatorDialog.Foreground(tcell.ColorBlue)
	navigatorPrompt      = navigatorDialog.Foreground(tcell.ColorBlue).Bold(true)
	navigatorSuccess     = navigatorDialog.Foreground(tcell.ColorDarkGreen).Bold(true)
	navigatorError       = navigatorDialog.Foreground(tcell.ColorRed).Bold(true)
	navigatorSelected    = navigatorDesktop.Foreground(tcell.ColorLightGray)
	navigatorButtonFocus = navigatorDialog.Foreground(tcell.ColorRed).Bold(true)
	navigatorInput       = navigatorDialog.Foreground(tcell.ColorBlack)
	navigatorInputFocus  = navigatorDesktop.Foreground(tcell.ColorLightGray)
)

func runNavigator() int {
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintln(os.Stderr, "yesh navigator:", err)
		return 1
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "yesh navigator:", err)
		return 1
	}
	defer screen.Fini()

	model := navigatorModel{
		source:      "",
		destination: "",
		lockPath:    "backup.lock",
		outputPath:  "backup.yesh",
	}
	for {
		drawNavigator(screen, model)
		screen.Show()
		event := screen.PollEvent()
		if quit := updateNavigator(&model, event); quit {
			return 0
		}
	}
}

func updateNavigator(model *navigatorModel, event tcell.Event) bool {
	if _, ok := event.(*tcell.EventResize); ok {
		return false
	}
	key, ok := event.(*tcell.EventKey)
	if !ok {
		return false
	}
	if model.page == navigatorBackup && model.sourceKindOpen {
		switch key.Key() {
		case tcell.KeyUp, tcell.KeyDown:
			setNavigatorSourceKind(model, (model.sourceKind+1)%2)
		case tcell.KeyEnter, tcell.KeyEscape:
			model.sourceKindOpen = false
		}
		return false
	}

	if key.Key() == tcell.KeyCtrlC || key.Key() == tcell.KeyEscape {
		if model.page == navigatorMain {
			return true
		}
		if model.page == navigatorBackupContext {
			model.page = navigatorMain
			return false
		}
		if model.page == navigatorServicePicker {
			model.page = navigatorMain
			return false
		}
		if model.page == navigatorServicePlan {
			model.page = navigatorServicePicker
			return false
		}
		if model.page == navigatorServiceHistory {
			model.page = navigatorServicePlan
			return false
		}
		if model.page == navigatorServiceReview {
			model.page = navigatorServicePlan
			return false
		}
		model.page = navigatorMain
		return false
	}

	switch model.page {
	case navigatorMain:
		return updateNavigatorMain(model, key)
	case navigatorBackupContext:
		return updateNavigatorBackupContext(model, key)
	case navigatorBackup:
		return updateNavigatorBackup(model, key)
	case navigatorServicePicker:
		return updateNavigatorServicePicker(model, key)
	case navigatorServicePlan:
		return updateNavigatorServicePlan(model, key)
	case navigatorServiceHistory:
		return updateNavigatorServiceHistory(model, key)
	case navigatorServiceReview:
		return updateNavigatorServiceReview(model, key)
	case navigatorReview:
		switch key.Key() {
		case tcell.KeyTab:
			model.reviewFocus = (model.reviewFocus + 1) % 2
		case tcell.KeyBacktab:
			model.reviewFocus = (model.reviewFocus + 1) % 2
		case tcell.KeyLeft, tcell.KeyRight:
			model.reviewFocus = (model.reviewFocus + 1) % 2
		case tcell.KeyEnter:
			if model.reviewFocus == 0 {
				if model.scriptError != nil {
					model.saveError = fmt.Errorf("fix the parser error before saving")
				} else if !model.saved {
					err := writeNavigatorScript(model.outputPath, model.script)
					if err == nil {
						model.saved = true
						model.savedPath = model.outputPath
					} else if os.IsPermission(err) {
						model.savedPath, model.saveError = saveNavigatorScriptTemp(model.script)
						model.saved = model.saveError == nil
						model.savedTemporary = model.saved
						if model.saved {
						}
					} else {
						model.saveError = err
					}
				}
			} else {
				model.page = navigatorBackup
			}
		case tcell.KeyRune:
			if key.Rune() == 's' {
				model.reviewFocus = 0
			}
			if key.Rune() == 'b' {
				model.reviewFocus = 1
			}
		}
		if key.Key() == tcell.KeyEscape {
			model.page = navigatorBackup
		}
	}
	return false
}

func updateNavigatorMain(model *navigatorModel, key *tcell.EventKey) bool {
	activities := navigatorActivities()
	switch key.Key() {
	case tcell.KeyUp:
		if model.mainFocus == 0 && model.mainIndex > 0 {
			model.mainIndex--
		}
	case tcell.KeyDown:
		if model.mainFocus == 0 && model.mainIndex < len(activities)-1 {
			model.mainIndex++
		}
	case tcell.KeyTab:
		model.mainFocus = (model.mainFocus + 1) % 3
	case tcell.KeyBacktab:
		model.mainFocus = (model.mainFocus + 2) % 3
	case tcell.KeyLeft:
		if model.mainFocus == 2 {
			model.mainFocus = 1
		}
	case tcell.KeyRight:
		if model.mainFocus == 1 {
			model.mainFocus = 2
		}
	case tcell.KeyEnter:
		if model.mainFocus == 2 {
			return true
		}
		if model.mainIndex == 0 {
			model.page = navigatorBackupContext
		} else if model.mainIndex == 1 {
			model.page = navigatorServicePicker
			if model.serviceManager == "" {
				model.serviceManager = "system"
			}
			model.services, model.serviceMessage = navigatorListServices(model.serviceManager)
			model.serviceReady = model.serviceMessage == nil
			model.serviceIndex = 0
			model.serviceOffset = 0
		} else {
			return true
		}
	case tcell.KeyRune:
		if strings.EqualFold(string(key.Rune()), "q") {
			return true
		}
	}
	return false
}

func updateNavigatorServicePicker(model *navigatorModel, key *tcell.EventKey) bool {
	if !model.serviceReady {
		return false
	}
	switch key.Key() {
	case tcell.KeyUp:
		if model.serviceIndex > 0 {
			model.serviceIndex--
			navigatorKeepServiceVisible(model)
		}
	case tcell.KeyDown:
		if model.serviceIndex < len(navigatorFilteredServices(model.services, model.serviceQuery))-1 {
			model.serviceIndex++
			navigatorKeepServiceVisible(model)
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(model.serviceQuery) > 0 {
			model.serviceQuery = model.serviceQuery[:len(model.serviceQuery)-1]
			model.serviceIndex = 0
			model.serviceOffset = 0
		}
	case tcell.KeyTab:
		filtered := navigatorFilteredServices(model.services, model.serviceQuery)
		if len(filtered) > 0 {
			model.serviceQuery = filtered[model.serviceIndex].unit
		}
	case tcell.KeyEnter:
		filtered := navigatorFilteredServices(model.services, model.serviceQuery)
		if len(filtered) > 0 {
			model.serviceUnit = filtered[model.serviceIndex].unit
			model.servicePlan = 0
			model.servicePlanFocus = 0
			model.serviceSuggestion = 0
			navigatorApplyMeasurementSuggestion(model)
			model.serviceSince = "1h"
			model.serviceOutput = "service-" + strings.TrimSuffix(model.serviceUnit, ".service") + ".yesh"
			model.serviceSaved = false
			model.serviceSavedPath = ""
			model.serviceSaveError = nil
			model.serviceHistory = navigatorMeasurementHistory{}
			model.serviceHistoryOK = false
			model.serviceHistoryErr = nil
			model.serviceHistory, model.serviceHistoryErr = navigatorFindMeasurementHistory(model.serviceManager, model.serviceUnit)
			model.serviceHistoryOK = model.serviceHistoryErr == nil && model.serviceHistory.count > 0
			navigatorRefreshServiceScript(model)
			model.page = navigatorServicePlan
		}
	case tcell.KeyRune:
		if model.serviceQuery == "" && (key.Rune() == 'm' || key.Rune() == 'M') {
			if model.serviceManager == "user" {
				model.serviceManager = "system"
			} else {
				model.serviceManager = "user"
			}
			model.services, model.serviceMessage = navigatorListServices(model.serviceManager)
			model.serviceReady = model.serviceMessage == nil
			model.serviceIndex = 0
			model.serviceOffset = 0
			return false
		}
		if unicode.IsSpace(key.Rune()) {
			return false
		}
		model.serviceQuery += string(key.Rune())
		model.serviceIndex = 0
		model.serviceOffset = 0
	}
	return false
}

func updateNavigatorServicePlan(model *navigatorModel, key *tcell.EventKey) bool {
	viewFocus := 5
	reviewFocus := 5
	if model.serviceHistoryOK {
		reviewFocus = 6
	}
	saveFocus := reviewFocus + 1
	backFocus := reviewFocus + 2
	focusCount := backFocus + 1
	switch key.Key() {
	case tcell.KeyUp, tcell.KeyDown:
		if model.servicePlanFocus == 0 {
			model.servicePlan = (model.servicePlan + 1) % 3
			navigatorMarkServiceDirty(model)
			navigatorRefreshServiceScript(model)
		}
	case tcell.KeyTab:
		model.servicePlanFocus = (model.servicePlanFocus + 1) % focusCount
	case tcell.KeyBacktab:
		model.servicePlanFocus = (model.servicePlanFocus + focusCount - 1) % focusCount
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		navigatorDeleteServicePlanField(model)
		navigatorMarkServiceDirty(model)
	case tcell.KeyEnter:
		if model.serviceHistoryOK && model.servicePlanFocus == viewFocus {
			model.page = navigatorServiceHistory
		} else if model.servicePlanFocus == reviewFocus {
			if model.serviceError != nil {
				model.serviceSaveError = fmt.Errorf("fix the validation error before reviewing")
			} else {
				model.serviceReviewTab = 0
				model.serviceReviewOffset = 0
				model.page = navigatorServiceReview
			}
		} else if model.servicePlanFocus == backFocus {
			model.page = navigatorServicePicker
		} else if model.servicePlanFocus == saveFocus {
			if model.serviceError != nil {
				model.serviceSaveError = fmt.Errorf("fix the validation error before saving")
			} else if !model.serviceSaved {
				if err := writeNavigatorScript(model.serviceOutput, model.serviceScript); err == nil {
					model.serviceSaved = true
					model.serviceSavedPath = model.serviceOutput
				} else if os.IsPermission(err) {
					model.serviceSavedPath, model.serviceSaveError = saveNavigatorScriptTemp(model.serviceScript)
					model.serviceSaved = model.serviceSaveError == nil
				} else {
					model.serviceSaveError = fmt.Errorf("cannot save yesh script to %q: %w", model.serviceOutput, err)
				}
			}
		} else {
			navigatorMarkServiceDirty(model)
			navigatorRefreshServiceScript(model)
		}
	case tcell.KeyRune:
		if model.servicePlan == 1 && (key.Rune() == 'p' || key.Rune() == 'P') {
			model.serviceSuggestion = (model.serviceSuggestion + 1) % len(navigatorMeasurementSuggestions)
			navigatorApplyMeasurementSuggestion(model)
			navigatorMarkServiceDirty(model)
			navigatorRefreshServiceScript(model)
			return false
		}
		if model.servicePlanFocus >= 1 && model.servicePlanFocus <= 4 && !unicode.IsSpace(key.Rune()) {
			navigatorAppendServicePlanField(model, key.Rune())
			navigatorMarkServiceDirty(model)
		}
	}
	if model.servicePlanFocus >= 1 && model.servicePlanFocus <= 4 {
		navigatorRefreshServiceScript(model)
	}
	return false
}

func navigatorMarkServiceDirty(model *navigatorModel) {
	model.serviceSaved = false
	model.serviceSavedPath = ""
	model.serviceSaveError = nil
}

func navigatorRefreshServiceScript(model *navigatorModel) {
	model.serviceRefreshes++
	model.servicePlanOutput = ""
	model.serviceScript, model.serviceError = navigatorServicePlanScript(model.serviceManager, model.serviceUnit, model.servicePlan, model.serviceDuration, model.serviceInterval, model.serviceKeep, model.serviceSince)
	if model.serviceError == nil {
		items, err := parse(model.serviceScript, "navigator-service.yesh")
		if err != nil {
			model.serviceError = err
		} else {
			model.servicePlanOutput, err = renderPlan(items, "navigator-service.yesh")
			if err != nil {
				model.serviceError = err
			}
		}
	}
}

func navigatorApplyMeasurementSuggestion(model *navigatorModel) {
	suggestion := navigatorMeasurementSuggestions[model.serviceSuggestion]
	model.serviceDuration = suggestion.duration
	model.serviceInterval = suggestion.interval
	model.serviceKeep = suggestion.keep
}

func navigatorAppendServicePlanField(model *navigatorModel, character rune) {
	switch model.servicePlanFocus {
	case 1:
		model.serviceDuration += string(character)
	case 2:
		model.serviceInterval += string(character)
	case 3:
		if model.servicePlan == 2 {
			model.serviceSince += string(character)
		} else {
			model.serviceKeep += string(character)
		}
	case 4:
		model.serviceOutput += string(character)
	}
}

func navigatorDeleteServicePlanField(model *navigatorModel) {
	var value *string
	switch model.servicePlanFocus {
	case 1:
		value = &model.serviceDuration
	case 2:
		value = &model.serviceInterval
	case 3:
		if model.servicePlan == 2 {
			value = &model.serviceSince
		} else {
			value = &model.serviceKeep
		}
	case 4:
		value = &model.serviceOutput
	}
	if value != nil && len(*value) > 0 {
		*value = (*value)[:len(*value)-1]
	}
}

func navigatorKeepServiceVisible(model *navigatorModel) {
	visibleRows := 8
	if model.serviceIndex < model.serviceOffset {
		model.serviceOffset = model.serviceIndex
	}
	if model.serviceIndex >= model.serviceOffset+visibleRows {
		model.serviceOffset = model.serviceIndex - visibleRows + 1
	}
}

func updateNavigatorBackupContext(model *navigatorModel, key *tcell.EventKey) bool {
	switch key.Key() {
	case tcell.KeyUp, tcell.KeyDown:
		if model.contextFocus == 0 {
			setNavigatorSourceKind(model, (model.sourceKind+1)%2)
		}
	case tcell.KeyTab:
		model.contextFocus = (model.contextFocus + 1) % 3
	case tcell.KeyBacktab:
		model.contextFocus = (model.contextFocus + 2) % 3
	case tcell.KeyLeft:
		if model.contextFocus == 2 {
			model.contextFocus = 1
		}
	case tcell.KeyRight:
		if model.contextFocus == 1 {
			model.contextFocus = 2
		}
	case tcell.KeyEnter:
		switch model.contextFocus {
		case 0:
			model.contextFocus = 1
		case 1:
			model.page = navigatorBackup
		case 2:
			model.page = navigatorMain
		}
	case tcell.KeyRune:
		switch strings.ToLower(string(key.Rune())) {
		case "l":
			setNavigatorSourceKind(model, 0)
		case "r":
			setNavigatorSourceKind(model, 1)
		}
	}
	return false
}

func updateNavigatorBackup(model *navigatorModel, key *tcell.EventKey) bool {
	const fieldCount = 7
	switch key.Key() {
	case tcell.KeyUp:
		if model.backupField > 0 {
			model.backupField--
		}
	case tcell.KeyDown:
		if model.backupField < fieldCount-1 {
			model.backupField++
		}
	case tcell.KeyTab:
		model.backupField = (model.backupField + 1) % fieldCount
	case tcell.KeyBacktab:
		model.backupField = (model.backupField + fieldCount - 1) % fieldCount
	case tcell.KeyEnter:
		if model.backupField == 0 {
			model.sourceKindOpen = true
		} else if model.backupField == 5 {
			recipe := model.backupRecipe()
			model.scriptError = nil
			model.planOutput = ""
			model.planError = nil
			model.saveError = nil
			model.saved = false
			model.savedPath = ""
			model.savedTemporary = false
			if err := recipe.validate(); err != nil {
				model.scriptError = err
			} else {
				model.script = recipe.script()
			}
			if model.scriptError == nil {
				items, err := parse(model.script, "navigator.yesh")
				if err != nil {
					model.scriptError = err
				} else {
					model.planOutput, err = renderPlan(items, "navigator.yesh")
					if err != nil {
						model.planError = err
					}
				}
			}
			model.page = navigatorReview
		} else if model.backupField == 6 {
			model.page = navigatorMain
		} else if model.backupField < 5 {
			model.backupField++
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		navigatorDeleteFieldCharacter(model)
	case tcell.KeyRune:
		if key.Rune() == 'q' {
			model.page = navigatorMain
		} else if model.backupField >= 1 && model.backupField < 5 {
			navigatorAppendFieldCharacter(model, key.Rune())
		}
	}
	return false
}

func navigatorAppendFieldCharacter(model *navigatorModel, character rune) {
	switch model.backupField {
	case 1:
		model.source += string(character)
	case 2:
		model.destination += string(character)
	case 3:
		model.lockPath += string(character)
	case 4:
		model.outputPath += string(character)
	}
}

func (model navigatorModel) backupRecipe() navigatorBackupRecipe {
	return navigatorBackupRecipe{
		sourceKind:  model.sourceKind,
		source:      model.source,
		destination: model.destination,
		lockPath:    model.lockPath,
		outputPath:  model.outputPath,
	}
}

func navigatorDeleteFieldCharacter(model *navigatorModel) {
	var value *string
	switch model.backupField {
	case 1:
		value = &model.source
	case 2:
		value = &model.destination
	case 3:
		value = &model.lockPath
	case 4:
		value = &model.outputPath
	}
	if value != nil && len(*value) > 0 {
		*value = (*value)[:len(*value)-1]
	}
}

func drawNavigator(screen tcell.Screen, model navigatorModel) {
	width, height := screen.Size()
	screen.Fill(' ', navigatorDesktop)
	switch model.page {
	case navigatorMain:
		drawMainDialog(screen, width, height, model)
	case navigatorBackupContext:
		drawBackupContextDialog(screen, width, height, model)
	case navigatorBackup:
		drawBackupDialog(screen, width, height, model)
	case navigatorReview:
		drawReviewDialog(screen, width, height, model)
	case navigatorServicePicker:
		drawServicePickerDialog(screen, width, height, model)
	case navigatorServicePlan:
		drawServicePlanDialog(screen, width, height, model)
	case navigatorServiceHistory:
		drawServiceHistoryDialog(screen, width, height, model)
	}
}

func drawServiceHistoryDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	if width < 72 || height < 18 {
		drawNavigatorTooSmall(screen)
		return
	}
	boxWidth, boxHeight := 112, 22
	compact := false
	if width < boxWidth {
		boxWidth = width - 2
		compact = true
	}
	if height < boxHeight {
		boxHeight = height - 2
	}
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	drawBox(screen, x, y, boxWidth, boxHeight, "Saved measurements")
	putNavigator(screen, x+3, y+2, "Read-only history", navigatorPrompt)
	putNavigator(screen, x+3, y+4, "Service:", navigatorLabel)
	putNavigator(screen, x+14, y+4, model.serviceManager+"/"+model.serviceUnit, navigatorInput)
	putNavigator(screen, x+3, y+5, "File:", navigatorLabel)
	putNavigator(screen, x+14, y+5, model.serviceHistory.path, navigatorInput)
	putNavigator(screen, x+3, y+7, navigatorMeasurementHistoryText(model.serviceHistory), navigatorInput)
	if compact {
		putNavigator(screen, x+3, y+9, "FINISHED              MEMORY        CHANGE  STATE", navigatorLabel)
	} else {
		putNavigator(screen, x+3, y+9, "FINISHED              DURATION  MEM  CHANGE                 STATE", navigatorLabel)
	}
	visibleRows := 8
	if boxHeight < 20 {
		visibleRows = boxHeight - 12
	}
	for index := 0; index < visibleRows; index++ {
		rowIndex := model.serviceHistoryOffset + index
		if rowIndex >= len(model.serviceHistory.rows) {
			break
		}
		row := model.serviceHistory.rows[rowIndex]
		memory := "?"
		if row.memoryAvailable {
			change := "?"
			if row.comparisonOK {
				change = navigatorSignedBytes(row.comparisonDelta)
			}
			memory = navigatorBytes(row.memoryEnd) + " " + row.marker + " " + change
		}
		state := "complete"
		if !row.complete {
			state = "incomplete: " + row.reason
		}
		line := fmt.Sprintf("%-21s %-9s %-35s %s", row.finished.UTC().Format("2006-01-02 15:04:05Z"), row.duration.Round(time.Second), memory, state)
		if compact {
			line = fmt.Sprintf("%-21s %-15s %-8s %s", row.finished.UTC().Format("2006-01-02 15:04:05Z"), navigatorBytes(row.memoryEnd), row.marker+" "+navigatorSignedBytes(row.comparisonDelta), state)
		}
		putNavigator(screen, x+3, y+10+index, padNavigator(line, boxWidth-6), navigatorInput)
	}
	putNavigator(screen, x+3, y+boxHeight-2, "Esc: back", navigatorLabel)
}

func drawServiceReviewDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	if width < 72 || height < 20 {
		drawNavigatorTooSmall(screen)
		return
	}
	boxWidth, boxHeight := 96, 22
	if width < boxWidth {
		boxWidth = width - 2
	}
	if height < boxHeight {
		boxHeight = height - 2
	}
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	drawBox(screen, x, y, boxWidth, boxHeight, "Review service script")
	putNavigator(screen, x+3, y+2, "Review is read-only; no service command will run.", navigatorPrompt)
	drawNavigatorButton(screen, x+3, y+4, "Plan", model.serviceReviewTab == 0)
	drawNavigatorButton(screen, x+12, y+4, "Script", model.serviceReviewTab == 1)
	lines := strings.Split(model.servicePlanOutput, "\n")
	if model.serviceReviewTab == 1 {
		lines = strings.Split(model.serviceScript, "\n")
	}
	putNavigator(screen, x+3, y+6, map[bool]string{true: "Yesh script:", false: "Plan:"}[model.serviceReviewTab == 1], navigatorLabel)
	visibleRows := boxHeight - 10
	for index := 0; index < visibleRows; index++ {
		lineIndex := model.serviceReviewOffset + index
		if lineIndex >= len(lines) {
			break
		}
		putNavigator(screen, x+5, y+7+index, padNavigator(lines[lineIndex], boxWidth-8), navigatorInput)
	}
	drawNavigatorButton(screen, x+boxWidth-29, y+boxHeight-3, "Save", model.serviceReviewFocus == 2)
	drawNavigatorButton(screen, x+boxWidth-15, y+boxHeight-3, "Back", model.serviceReviewFocus == 3)
	putNavigator(screen, x+3, y+boxHeight-2, "Tab: tab/controls   Up/Down: scroll   Enter: select   Esc: back", navigatorLabel)
}

func drawNavigatorTooSmall(screen tcell.Screen) {
	putNavigator(screen, 1, 1, "Terminal is too small. Resize to at least 72x20.", navigatorError)
	putNavigator(screen, 1, 3, "Esc: back", navigatorLabel)
}

func updateNavigatorServiceHistory(model *navigatorModel, key *tcell.EventKey) bool {
	const visibleRows = 8
	switch key.Key() {
	case tcell.KeyUp:
		if model.serviceHistoryOffset > 0 {
			model.serviceHistoryOffset--
		}
	case tcell.KeyDown:
		if model.serviceHistoryOffset+visibleRows < len(model.serviceHistory.rows) {
			model.serviceHistoryOffset++
		}
	case tcell.KeyPgUp:
		model.serviceHistoryOffset -= visibleRows
		if model.serviceHistoryOffset < 0 {
			model.serviceHistoryOffset = 0
		}
	case tcell.KeyPgDn:
		model.serviceHistoryOffset += visibleRows
		maxOffset := len(model.serviceHistory.rows) - visibleRows
		if maxOffset < 0 {
			maxOffset = 0
		}
		if model.serviceHistoryOffset > maxOffset {
			model.serviceHistoryOffset = maxOffset
		}
	}
	return false
}

func updateNavigatorServiceReview(model *navigatorModel, key *tcell.EventKey) bool {
	lines := strings.Split(model.servicePlanOutput, "\n")
	if model.serviceReviewTab == 1 {
		lines = strings.Split(model.serviceScript, "\n")
	}
	visibleRows := 10
	switch key.Key() {
	case tcell.KeyTab, tcell.KeyLeft, tcell.KeyRight:
		model.serviceReviewTab = (model.serviceReviewTab + 1) % 2
		model.serviceReviewOffset = 0
	case tcell.KeyUp:
		if model.serviceReviewOffset > 0 {
			model.serviceReviewOffset--
		}
	case tcell.KeyDown:
		if model.serviceReviewOffset+visibleRows < len(lines) {
			model.serviceReviewOffset++
		}
	case tcell.KeyEnter:
		if model.serviceReviewFocus == 0 {
			model.serviceReviewFocus = 1
		} else if model.serviceReviewFocus == 1 {
			model.serviceReviewFocus = 2
		} else if model.serviceReviewFocus == 2 {
			if err := writeNavigatorScript(model.serviceOutput, model.serviceScript); err == nil {
				model.serviceSaved = true
				model.serviceSavedPath = model.serviceOutput
			} else if os.IsPermission(err) {
				model.serviceSavedPath, model.serviceSaveError = saveNavigatorScriptTemp(model.serviceScript)
				model.serviceSaved = model.serviceSaveError == nil
			} else {
				model.serviceSaveError = fmt.Errorf("cannot save yesh script to %q: %w", model.serviceOutput, err)
			}
		} else {
			model.page = navigatorServicePlan
		}
	case tcell.KeyRune:
		if key.Rune() == 'p' || key.Rune() == 'P' {
			model.serviceReviewTab = 0
			model.serviceReviewOffset = 0
		} else if key.Rune() == 's' || key.Rune() == 'S' {
			model.serviceReviewTab = 1
			model.serviceReviewOffset = 0
		}
	}
	return false
}

func navigatorBytes(value uint64) string {
	return fmt.Sprintf("%dB", value)
}

func navigatorSignedBytes(value int64) string {
	return fmt.Sprintf("%+dB", value)
}

func drawServicePlanDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	boxWidth, boxHeight := 96, 24
	if width < 72 || height < 20 {
		drawNavigatorTooSmall(screen)
		return
	}
	if width < boxWidth {
		boxWidth = width - 2
	}
	if height < boxHeight {
		boxHeight = height - 2
	}
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	compact := boxWidth < 96
	drawBox(screen, x, y, boxWidth, boxHeight, "Inspect a service")
	putNavigator(screen, x+3, y+2, "Step 2 of 2 — build and save a yesh script", navigatorPrompt)
	putNavigator(screen, x+3, y+3, "What do you want to learn about "+model.serviceUnit+"?", navigatorLabel)
	options := []string{
		"Take one resource snapshot",
		"Measure resources for " + model.serviceDuration,
		"Summarize history since " + model.serviceSince,
	}
	for index, option := range options {
		marker := "[ ] "
		style := navigatorDialog
		if index == model.servicePlan {
			marker = "[x] "
			style = navigatorSelected
		}
		putNavigator(screen, x+5, y+4+index, padNavigator(marker+option, boxWidth-10), style)
	}
	drawNavigatorField(screen, x+3, y+7, "Duration:", model.serviceDuration, model.servicePlanFocus == 1, boxWidth)
	drawNavigatorField(screen, x+3, y+8, "Interval:", model.serviceInterval, model.servicePlanFocus == 2, boxWidth)
	thirdLabel, thirdValue := "Keep history:", model.serviceKeep
	if model.servicePlan == 2 {
		thirdLabel, thirdValue = "Since:", model.serviceSince
	}
	drawNavigatorField(screen, x+3, y+9, thirdLabel, thirdValue, model.servicePlanFocus == 3, boxWidth)
	drawNavigatorField(screen, x+3, y+10, "Script file:", model.serviceOutput, model.servicePlanFocus == 4, boxWidth)
	if model.servicePlan == 1 {
		suggestion := navigatorMeasurementSuggestions[model.serviceSuggestion]
		putNavigator(screen, x+3, y+11, "Profile:", navigatorLabel)
		putNavigator(screen, x+13, y+11, "[ "+suggestion.name+" ]", navigatorInputFocus)
		putNavigator(screen, x+32, y+11, "(P: next suggestion)", navigatorLabel)
	}
	putNavigator(screen, x+3, y+12, "Why this matters:", navigatorLabel)
	if model.servicePlan == 0 {
		putNavigator(screen, x+5, y+13, "A finite read-only view of cgroup and process resources.", navigatorInput)
	} else if model.servicePlan == 1 {
		putNavigator(screen, x+5, y+13, navigatorMeasurementSuggestions[model.serviceSuggestion].reason, navigatorInput)
	} else {
		putNavigator(screen, x+5, y+13, "Reads saved observations; it does not collect new samples.", navigatorInput)
	}
	if model.serviceHistoryErr != nil {
		putNavigator(screen, x+3, y+14, "Saved measurements could not be read:", navigatorError)
		putNavigator(screen, x+5, y+15, model.serviceHistoryErr.Error(), navigatorError)
	} else if model.serviceError != nil {
		putNavigator(screen, x+3, y+14, "The generated fragment failed validation:", navigatorError)
		putNavigator(screen, x+5, y+15, model.serviceError.Error(), navigatorError)
	} else if model.serviceSaved {
		putNavigator(screen, x+3, y+14, "Yesh script saved.", navigatorSuccess)
		putNavigator(screen, x+3, y+15, "File:", navigatorLabel)
		putNavigator(screen, x+10, y+15, model.serviceSavedPath, navigatorInput)
	} else if model.serviceSaveError != nil {
		putNavigator(screen, x+3, y+14, model.serviceSaveError.Error(), navigatorError)
	} else if compact {
		putNavigator(screen, x+3, y+14, "Plan:", navigatorLabel)
		for index, line := range strings.Split(model.servicePlanOutput, "\n") {
			if index >= 3 {
				break
			}
			putNavigator(screen, x+5, y+15+index, line, navigatorInput)
		}
	} else {
		putNavigator(screen, x+3, y+14, "Plan:", navigatorLabel)
		for index, line := range strings.Split(model.servicePlanOutput, "\n") {
			if index >= 3 {
				break
			}
			putNavigator(screen, x+5, y+15+index, line, navigatorInput)
		}
		putNavigator(screen, x+55, y+14, "Yesh script:", navigatorLabel)
		for index, line := range strings.Split(model.serviceScript, "\n") {
			if index >= 3 {
				break
			}
			putNavigator(screen, x+57, y+15+index, line, navigatorInput)
		}
	}
	if model.serviceHistoryOK {
		drawNavigatorButton(screen, x+3, y+boxHeight-3, "View measurements", model.servicePlanFocus == 5)
		drawNavigatorButton(screen, x+24, y+boxHeight-3, "Review", model.servicePlanFocus == 6)
	} else {
		drawNavigatorButton(screen, x+3, y+boxHeight-3, "Review", model.servicePlanFocus == 5)
	}
	saveFocus := 6
	backFocus := 7
	if model.serviceHistoryOK {
		saveFocus, backFocus = 7, 8
	}
	drawNavigatorButton(screen, x+boxWidth-31, y+boxHeight-3, "Save", model.servicePlanFocus == saveFocus)
	drawNavigatorButton(screen, x+boxWidth-17, y+boxHeight-3, "Back", model.servicePlanFocus == backFocus)
	putNavigator(screen, x+3, y+boxHeight-2, "Tab: next   Enter on Save: write the file   Esc: back", navigatorLabel)
}

func navigatorServicePlanScript(manager, unit string, plan int, duration, interval, keep, since string) (string, error) {
	managerWord := manager + " "
	if plan == 1 {
		if duration == "" || interval == "" || keep == "" {
			return "", fmt.Errorf("duration, interval, and history must not be empty")
		}
		script := fmt.Sprintf("measure service %s%s for %s every %s:\n    keep history for %s\n", managerWord, unit, duration, interval, keep)
		return script, nil
	}
	if plan == 2 {
		if since == "" {
			return "", fmt.Errorf("since duration must not be empty")
		}
		script := fmt.Sprintf("summarize service %s%s since %s\n", managerWord, unit, since)
		return script, nil
	}
	script := fmt.Sprintf("service status %s%s\nservice resources %s%s\n", managerWord, unit, managerWord, unit)
	return script, nil
}

func drawServicePickerDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	boxWidth, boxHeight := 96, 22
	if width < 72 || height < 20 {
		drawNavigatorTooSmall(screen)
		return
	}
	if width < boxWidth {
		boxWidth = width - 2
	}
	if height < boxHeight {
		boxHeight = height - 2
	}
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	drawBox(screen, x, y, boxWidth, boxHeight, "Inspect a service")
	putNavigator(screen, x+3, y+2, "Step 1 of 2 — choose a service", navigatorPrompt)
	putNavigator(screen, x+3, y+3, "Loaded services only; nothing will be started or stopped.", navigatorLabel)
	putNavigator(screen, x+3, y+4, "Manager:", navigatorLabel)
	putNavigator(screen, x+13, y+4, "[ "+model.serviceManager+" ]", navigatorInputFocus)
	putNavigator(screen, x+30, y+4, "(press M to switch)", navigatorLabel)
	putNavigator(screen, x+3, y+5, "Service name:", navigatorLabel)
	putNavigator(screen, x+17, y+5, "["+padNavigator(model.serviceQuery, 38)+"]", navigatorInputFocus)
	putNavigator(screen, x+5, y+6, "UNIT                            STATE             DESCRIPTION", navigatorLabel)

	filtered := navigatorFilteredServices(model.services, model.serviceQuery)
	const visibleRows = 8
	for row := 0; row < visibleRows; row++ {
		index := model.serviceOffset + row
		if index >= len(filtered) {
			break
		}
		service := filtered[index]
		style := navigatorDialog
		if index == model.serviceIndex {
			style = navigatorSelected
		}
		line := fmt.Sprintf("[ ] %-30s %-16s %s", service.unit, service.active+"/"+service.sub, service.description)
		if index == model.serviceIndex {
			line = fmt.Sprintf("[x] %-30s %-16s %s", service.unit, service.active+"/"+service.sub, service.description)
		}
		putNavigator(screen, x+5, y+7+row, padNavigator(line, boxWidth-10), style)
	}
	if len(filtered) > visibleRows {
		drawNavigatorScrollbar(screen, x+boxWidth-3, y+7, visibleRows, len(filtered), model.serviceOffset)
	}
	if len(filtered) > 0 {
		first := model.serviceOffset + 1
		last := model.serviceOffset + visibleRows
		if last > len(filtered) {
			last = len(filtered)
		}
		putNavigator(screen, x+boxWidth-25, y+16, fmt.Sprintf("%d-%d of %d", first, last, len(filtered)), navigatorLabel)
	}
	if model.serviceMessage != nil {
		putNavigator(screen, x+3, y+16, model.serviceMessage.Error(), navigatorPrompt)
	}
	if model.serviceUnit != "" {
		putNavigator(screen, x+3, y+17, "Prepared yesh:", navigatorLabel)
		putNavigator(screen, x+5, y+18, "service resources "+model.serviceUnit, navigatorInput)
	}
	putNavigator(screen, x+3, y+boxHeight-2, "M: manager   Type to filter   Tab: complete   Up/Down: choose and scroll   Enter: select   Esc: back", navigatorLabel)
}

func drawNavigatorScrollbar(screen tcell.Screen, x, y, height, total, offset int) {
	putNavigator(screen, x, y, "▲", navigatorLabel)
	putNavigator(screen, x, y+height-1, "▼", navigatorLabel)
	trackHeight := height - 2
	for row := 0; row < trackHeight; row++ {
		putNavigator(screen, x, y+1+row, "│", navigatorLabel)
	}
	thumbHeight := trackHeight * trackHeight / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	maxOffset := total - trackHeight
	thumbTop := 0
	if maxOffset > 0 {
		thumbTop = offset * (trackHeight - thumbHeight) / maxOffset
	}
	for row := 0; row < thumbHeight; row++ {
		putNavigator(screen, x, y+1+thumbTop+row, "█", navigatorSelected)
	}
}

func drawBackupContextDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	boxWidth, boxHeight := 78, 18
	if width < 72 || height < 18 {
		drawNavigatorTooSmall(screen)
		return
	}
	if width < boxWidth {
		boxWidth = width - 2
	}
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	drawBox(screen, x, y, boxWidth, boxHeight, "Create a backup script")
	putNavigator(screen, x+3, y+2, "Step 1 of 3 — choose the source context", navigatorPrompt)
	putNavigator(screen, x+3, y+3, "What kind of source do you have?", navigatorLabel)

	contextOptions := []string{
		"Local folder",
		"Remote host via rsync",
	}
	for index, option := range contextOptions {
		style := navigatorDialog
		if model.contextFocus == 0 && index == model.sourceKind {
			style = navigatorSelected
		}
		marker := "[ ] "
		if index == model.sourceKind {
			marker = "[x] "
		}
		putNavigator(screen, x+5, y+5+index, padNavigator(marker+option, boxWidth-10), style)
	}

	putNavigator(screen, x+3, y+9, "Why this matters:", navigatorLabel)
	if model.sourceKind == 0 {
		putNavigator(screen, x+5, y+10, "yesh checks the local source before it starts rsync.", navigatorInput)
		putNavigator(screen, x+5, y+11, "The generated script will use: if exists $source", navigatorInput)
	} else {
		putNavigator(screen, x+5, y+10, "yesh cannot prove a remote path with local exists.", navigatorInput)
		putNavigator(screen, x+5, y+11, "rsync remains responsible for contacting the remote host.", navigatorInput)
	}
	drawNavigatorButton(screen, x+boxWidth-33, y+boxHeight-3, "Continue", model.contextFocus == 1)
	drawNavigatorButton(screen, x+boxWidth-18, y+boxHeight-3, "Back", model.contextFocus == 2)
	putNavigator(screen, x+3, y+boxHeight-2, "L/R: choose source   Tab: move   Enter: select   Esc: back", navigatorLabel)
}

func drawMainDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	boxWidth, boxHeight := 56, 12
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	drawBox(screen, x, y, boxWidth, boxHeight, "yesh navigator")
	putNavigator(screen, x+3, y+2, "Choose a script to compose", navigatorPrompt)
	for index, activity := range navigatorActivities() {
		style := navigatorDialog
		if model.mainFocus == 0 && index == model.mainIndex {
			style = navigatorSelected
		}
		putNavigator(screen, x+4, y+4+index, padNavigator(activity.name, boxWidth-8), style)
	}
	drawNavigatorButton(screen, x+boxWidth-24, y+boxHeight-3, "OK", model.mainFocus == 1)
	drawNavigatorButton(screen, x+boxWidth-14, y+boxHeight-3, "Cancel", model.mainFocus == 2)
	putNavigator(screen, x+3, y+boxHeight-2, "Tab: choose   Shift+Tab: back   Enter: select", navigatorLabel)
}

func drawBackupDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	boxWidth, boxHeight := 78, 18
	if width < 72 || height < 18 {
		drawNavigatorTooSmall(screen)
		return
	}
	if width < boxWidth {
		boxWidth = width - 2
	}
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	drawBox(screen, x, y, boxWidth, boxHeight, "Create a backup script")
	putNavigator(screen, x+3, y+2, "Step 2 of 3 — fill in the backup parameters", navigatorPrompt)
	drawNavigatorSelect(screen, x+3, y+4, "Source:", sourceKindName(model.sourceKind), model.backupField == 0, boxWidth)
	drawNavigatorField(screen, x+3, y+5, "Data folder:", model.source, model.backupField == 1, boxWidth)
	drawNavigatorField(screen, x+3, y+6, "Destination:", model.destination, model.backupField == 2, boxWidth)
	drawNavigatorField(screen, x+3, y+7, "Lock file:", model.lockPath, model.backupField == 3, boxWidth)
	drawNavigatorField(screen, x+3, y+8, "Script file:", model.outputPath, model.backupField == 4, boxWidth)
	if model.sourceKindOpen {
		drawNavigatorDropdown(screen, x+15, y+5, 29, model.sourceKind)
	}
	helpTitle, helpText := navigatorParameterHelp(model.backupField)
	putNavigator(screen, x+3, y+11, helpTitle, navigatorLabel)
	putNavigator(screen, x+5, y+12, helpText, navigatorInput)
	drawNavigatorButton(screen, x+boxWidth-33, y+boxHeight-3, "Check script", model.backupField == 5)
	drawNavigatorButton(screen, x+boxWidth-18, y+boxHeight-3, "Back", model.backupField == 6)
	putNavigator(screen, x+3, y+boxHeight-2, "Tab: move   Enter on Check script: review   Esc: back", navigatorLabel)
}

func drawReviewDialog(screen tcell.Screen, width, height int, model navigatorModel) {
	boxWidth, boxHeight := 96, 22
	if width < 72 || height < 22 {
		drawNavigatorTooSmall(screen)
		return
	}
	if width < boxWidth {
		boxWidth = width - 2
	}
	x, y := centeredBox(width, height, boxWidth, boxHeight)
	drawBox(screen, x, y, boxWidth, boxHeight, "Review and save")
	putNavigator(screen, x+3, y+1, "Step 3 of 3 — review and save the yesh script", navigatorPrompt)
	if model.saved {
		if model.savedTemporary {
			putNavigator(screen, x+3, y+2, "Yesh script saved to a temporary file.", navigatorSuccess)
		} else {
			putNavigator(screen, x+3, y+2, "Yesh script saved.", navigatorSuccess)
		}
		putNavigator(screen, x+3, y+3, "File:", navigatorLabel)
		putNavigator(screen, x+10, y+3, model.savedPath, navigatorInput)
		if model.savedTemporary {
			putNavigator(screen, x+3, y+7, "The requested folder was not writable.", navigatorPrompt)
		}
	} else if model.scriptError == nil && model.planError == nil && model.saveError == nil {
		putNavigator(screen, x+3, y+2, "No command has been run. The script passed the yesh parser.", navigatorPrompt)
	} else {
		message := ""
		if model.scriptError != nil {
			message = model.scriptError.Error()
		}
		if message == "" && model.planError != nil {
			message = model.planError.Error()
		}
		if message == "" && model.saveError != nil {
			message = model.saveError.Error()
		}
		putNavigator(screen, x+3, y+2, "The operation failed:", navigatorError)
		putNavigator(screen, x+3, y+3, message, navigatorError)
	}
	putNavigator(screen, x+3, y+4, "Source:", navigatorLabel)
	putNavigator(screen, x+17, y+4, model.source, navigatorInput)
	putNavigator(screen, x+3, y+5, "Source:", navigatorLabel)
	putNavigator(screen, x+17, y+5, sourceKindName(model.sourceKind), navigatorInput)
	putNavigator(screen, x+3, y+6, "Destination:", navigatorLabel)
	putNavigator(screen, x+17, y+6, model.destination, navigatorInput)
	putNavigator(screen, x+3, y+8, "Plan:", navigatorLabel)
	for index, line := range strings.Split(model.planOutput, "\n") {
		if index >= 4 {
			break
		}
		putNavigator(screen, x+5, y+9+index, line, navigatorInput)
	}
	putNavigator(screen, x+3, y+14, "Warning: no deletion, no command execution, no overwrite.", navigatorPrompt)
	putNavigator(screen, x+3, y+16, "Yesh script:", navigatorLabel)
	for index, line := range strings.Split(model.script, "\n") {
		if index >= 4 {
			break
		}
		putNavigator(screen, x+5, y+17+index, line, navigatorInput)
	}
	saveLabel := "Save"
	if model.saved {
		saveLabel = "Saved"
	}
	drawNavigatorButton(screen, x+boxWidth-28, y+boxHeight-2, saveLabel, model.reviewFocus == 0)
	drawNavigatorButton(screen, x+boxWidth-15, y+boxHeight-2, "Back", model.reviewFocus == 1)
	putNavigator(screen, x+3, y+boxHeight-2, "Tab: choose   Enter: select   Esc: back", navigatorLabel)
}

func sourceKindName(kind int) string {
	if kind == 1 {
		return "Folder on another computer"
	}
	return "Folder on this computer"
}

func setNavigatorSourceKind(model *navigatorModel, kind int) {
	if model.sourceKind == kind {
		return
	}
	if kind == 1 && model.source == "/srv/data" {
		model.source = "backup-host:/srv/data"
	}
	if kind == 0 && model.source == "backup-host:/srv/data" {
		model.source = "/srv/data"
	}
	model.sourceKind = kind
}

func navigatorBackupScript(sourceKind int, source, destination, lockPath string) string {
	if sourceKind == 1 {
		return fmt.Sprintf(`source = %s
target = %s
lock = %s

if exists $target:
    with lock $lock:
        rsync -a --itemize-changes -- $source $target
else:
    echo "destination directory is missing"
    exit 2
`, navigatorQuote(source), navigatorQuote(destination), navigatorQuote(lockPath))
	}
	return fmt.Sprintf(`source = %s
target = %s
lock = %s

if exists $source:
    if exists $target:
        with lock $lock:
            rsync -a --itemize-changes -- $source $target
    else:
        echo "destination directory is missing"
        exit 2
else:
    echo "source directory is missing"
    exit 2
`, navigatorQuote(source), navigatorQuote(destination), navigatorQuote(lockPath))
}

func navigatorQuote(value string) string {
	return strconv.Quote(value)
}

func saveNavigatorScript(path, script string) error {
	if err := writeNavigatorScript(path, script); err != nil {
		return fmt.Errorf("cannot save yesh script to %q: %w", path, err)
	}
	return nil
}

func writeNavigatorScript(path, script string) error {
	if path == "" {
		return fmt.Errorf("output file is empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func saveNavigatorScriptTemp(script string) (string, error) {
	file, err := os.CreateTemp("", "yesh-navigator-*.yesh")
	if err != nil {
		return "", fmt.Errorf("cannot save temporary yesh script: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("cannot save temporary yesh script: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("cannot save temporary yesh script: %w", err)
	}
	return path, nil
}

func drawNavigatorField(screen tcell.Screen, x, y int, label, value string, focused bool, boxWidth int) {
	putNavigator(screen, x, y, label, navigatorLabel)
	style := navigatorInput
	if focused {
		style = navigatorInputFocus
	}
	fieldWidth := boxWidth - len(label) - 9
	putNavigator(screen, x+len(label)+1, y, "["+padNavigator(value, fieldWidth)+"]", style)
}

func drawNavigatorSelect(screen tcell.Screen, x, y int, label, value string, focused bool, boxWidth int) {
	putNavigator(screen, x, y, label, navigatorLabel)
	style := navigatorInput
	if focused {
		style = navigatorInputFocus
	}
	fieldWidth := boxWidth - len(label) - 13
	putNavigator(screen, x+len(label)+1, y, "["+padNavigator(value+" v", fieldWidth)+"]", style)
}

func drawNavigatorDropdown(screen tcell.Screen, x, y, width, selected int) {
	height := 4
	for row := y; row < y+height; row++ {
		for column := x; column < x+width; column++ {
			screen.SetContent(column, row, ' ', nil, navigatorDialog)
		}
	}
	for column := x + 1; column < x+width-1; column++ {
		putNavigator(screen, column, y, "─", navigatorFrame)
		putNavigator(screen, column, y+height-1, "─", navigatorFrame)
	}
	for row := y + 1; row < y+height-1; row++ {
		putNavigator(screen, x, row, "│", navigatorFrame)
		putNavigator(screen, x+width-1, row, "│", navigatorFrame)
	}
	putNavigator(screen, x, y, "┌", navigatorFrame)
	putNavigator(screen, x+width-1, y, "┐", navigatorFrame)
	putNavigator(screen, x, y+height-1, "└", navigatorFrame)
	putNavigator(screen, x+width-1, y+height-1, "┘", navigatorFrame)

	options := []string{"Folder on this computer", "Folder on another computer"}
	for index, option := range options {
		style := navigatorDialog
		if index == selected {
			style = navigatorSelected
		}
		putNavigator(screen, x+2, y+1+index, padNavigator(option, width-4), style)
	}
}

func drawNavigatorButton(screen tcell.Screen, x, y int, label string, focused bool) {
	if focused {
		putNavigator(screen, x, y, "[ "+label+" ]", navigatorButtonFocus)
		return
	}
	putNavigator(screen, x, y, "  "+label+"  ", navigatorDialog)
}

func navigatorParameterHelp(field int) (string, string) {
	switch field {
	case 0:
		return "Source", "Choose a folder on this computer or on another computer."
	case 1:
		return "Data folder", "Enter a path, for example /srv/data; for another computer use host:/path."
	case 2:
		return "Destination", "Enter an existing folder, for example /backup/current."
	case 3:
		return "Lock file", "A local file that prevents overlapping runs; example: backup.lock."
	case 4:
		return "Script file", "Choose a new .yesh filename; existing files are never overwritten."
	case 5:
		return "Check script", "Review the checks and visible rsync command before saving."
	default:
		return "Back", "Return to the previous screen without running or saving anything."
	}
}

func centeredBox(screenWidth, screenHeight, boxWidth, boxHeight int) (int, int) {
	return (screenWidth - boxWidth) / 2, (screenHeight - boxHeight) / 2
}

func drawBox(screen tcell.Screen, x, y, width, height int, title string) {
	for row := y; row < y+height; row++ {
		for column := x; column < x+width; column++ {
			screen.SetContent(column, row, ' ', nil, navigatorDialog)
		}
	}
	for column := x + 1; column < x+width-1; column++ {
		putNavigator(screen, column, y, "─", navigatorFrame)
		putNavigator(screen, column, y+height-1, "─", navigatorFrame)
	}
	for row := y + 1; row < y+height-1; row++ {
		putNavigator(screen, x, row, "│", navigatorFrame)
		putNavigator(screen, x+width-1, row, "│", navigatorFrame)
	}
	putNavigator(screen, x, y, "┌", navigatorFrame)
	putNavigator(screen, x+width-1, y, "┐", navigatorFrame)
	putNavigator(screen, x, y+height-1, "└", navigatorFrame)
	putNavigator(screen, x+width-1, y+height-1, "┘", navigatorFrame)
	putNavigator(screen, x+2, y, " "+title+" ", navigatorTitle)
}

func putNavigator(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	for _, character := range text {
		screen.SetContent(x, y, character, nil, style)
		x++
	}
}

func padNavigator(value string, width int) string {
	if len(value) > width {
		return value[:width]
	}
	return value + strings.Repeat(" ", width-len(value))
}
