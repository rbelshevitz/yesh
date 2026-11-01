//go:build linux

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type serviceResourceSummary struct {
	records, complete, incomplete, samples   int
	first, last                              time.Time
	memoryStart, memoryEnd, memoryMax        uint64
	rssStart, rssEnd, rssMax                 uint64
	cpuDelta, oomKillDelta                   uint64
	hasMemory, hasRSS, hasCPU, hasOOM        bool
	memoryPoints, rssPoints                  []serviceGaugePoint
	memorySlope, rssSlope                    float64
	hasMemorySlope, hasRSSSlope              bool
	memoryTrend, rssTrend                    string
	processTop                               serviceProcessGrowth
	processHasTop, processAvailable          bool
	processPeak                              serviceProcessGrowth
	processHasPeak                           bool
	processIncomplete                        bool
	processStarted, processExited            int
	processComparisons                       int
	memoryLimit                              string
	ioDeltas                                 map[string]serviceIODelta
	ioAvailable, ioUnavailable, ioIncomplete bool
}

type serviceGaugePoint struct {
	at    time.Time
	value float64
}

type serviceGaugeAnalysis struct {
	slope                                   float64
	trend                                   string
	ok                                      bool
	firstIncreaseAt, lastIncreaseAt, peakAt time.Time
	peakValue                               float64
}

func (r *runtime) runServiceResourceSummary(manager, unit, stateDir string, since time.Duration) error {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return fmt.Errorf("read resource state directory: %w", err)
	}
	states := make([]serviceResourceState, 0, len(entries))
	cutoff := time.Now().UTC().Add(-since)
	journalPath := filepath.Join(stateDir, serviceResourceJournalName)
	if _, err := os.Stat(journalPath); err == nil {
		journalStates, err := readServiceResourceJournal(journalPath)
		if err != nil {
			return err
		}
		for _, state := range journalStates {
			if state.Manager == manager && state.Unit == unit && !state.FinishedAt.Before(cutoff) {
				states = append(states, state)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat resource state journal %q: %w", journalPath, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(stateDir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open resource state %q: %w", path, err)
		}
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		var state serviceResourceState
		decodeErr := decoder.Decode(&state)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode resource state %q: %w", path, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close resource state %q: %w", path, closeErr)
		}
		if err := state.validate(); err != nil {
			return fmt.Errorf("validate resource state %q: %w", path, err)
		}
		if state.Manager != manager || state.Unit != unit || state.FinishedAt.Before(cutoff) {
			continue
		}
		states = append(states, state)
	}
	if len(states) == 0 {
		return fmt.Errorf("no resource measurements for %s in the requested period", unit)
	}
	if err := validateServiceResourceSummaryIdentity(states); err != nil {
		return err
	}
	sort.Slice(states, func(i, j int) bool { return states[i].StartedAt.Before(states[j].StartedAt) })
	summary := serviceResourceSummary{ioDeltas: make(map[string]serviceIODelta)}
	for _, state := range states {
		summary.records++
		if state.Complete {
			summary.complete++
		} else {
			summary.incomplete++
		}
		summary.samples += len(state.Samples)
		if summary.first.IsZero() || state.StartedAt.Before(summary.first) {
			summary.first = state.StartedAt
		}
		if state.FinishedAt.After(summary.last) {
			summary.last = state.FinishedAt
		}
		first, last := state.Samples[0], state.Samples[len(state.Samples)-1]
		if summary.memoryLimit == "" || summary.memoryLimit == "unavailable" {
			summary.memoryLimit = first.Metrics["memory.max"]
		}
		updateGauge(&summary.memoryStart, &summary.memoryEnd, &summary.memoryMax, &summary.hasMemory, first.Metrics["memory.current"], last.Metrics["memory.current"], state.Samples, "memory.current")
		updateGauge(&summary.rssStart, &summary.rssEnd, &summary.rssMax, &summary.hasRSS, fmt.Sprint(first.RSSBytes), fmt.Sprint(last.RSSBytes), nil, "")
		for _, sample := range state.Samples {
			if value, ok := parseServiceResourceUint(sample.Metrics["memory.current"]); ok {
				summary.memoryPoints = append(summary.memoryPoints, serviceGaugePoint{at: sample.ObservedAt, value: float64(value)})
			}
			if !sample.ProcessesIncomplete {
				summary.rssPoints = append(summary.rssPoints, serviceGaugePoint{at: sample.ObservedAt, value: float64(sample.RSSBytes)})
			}
		}
		if before, ok := parseServiceResourceUint(first.Metrics["cpu.stat.usage_usec"]); ok {
			if after, ok := parseServiceResourceUint(last.Metrics["cpu.stat.usage_usec"]); ok && after >= before {
				summary.cpuDelta += after - before
				summary.hasCPU = true
			}
		}
		if before, ok := parseServiceResourceUint(first.Metrics["memory.events.oom_kill"]); ok {
			if after, ok := parseServiceResourceUint(last.Metrics["memory.events.oom_kill"]); ok && after >= before {
				summary.oomKillDelta += after - before
				summary.hasOOM = true
			}
		}
		ioDeltas, ioAvailable, ioComplete := compareServiceIOStats(first.Metrics, last.Metrics)
		if !ioAvailable {
			summary.ioUnavailable = true
		} else {
			summary.ioAvailable = true
			if !ioComplete || !mergeServiceIODeltas(summary.ioDeltas, ioDeltas) {
				summary.ioIncomplete = true
			}
		}
	}
	processSummary := serviceProcessSummary{}
	summarizeServiceProcessSamples(&processSummary, states)
	summary.processTop, summary.processHasTop = processSummary.TopDelta, processSummary.HasTop
	summary.processPeak, summary.processHasPeak = processSummary.Peak, processSummary.HasPeak
	summary.processStarted, summary.processExited = processSummary.Started, processSummary.Exited
	summary.processComparisons = processSummary.Comparisons
	summary.processIncomplete, summary.processAvailable = processSummary.Incomplete, processSummary.Available
	memoryAnalysis := analyzeServiceGaugeDetails(summary.memoryPoints)
	rssAnalysis := analyzeServiceGaugeDetails(summary.rssPoints)
	summary.memorySlope, summary.memoryTrend, summary.hasMemorySlope = memoryAnalysis.slope, memoryAnalysis.trend, memoryAnalysis.ok
	summary.rssSlope, summary.rssTrend, summary.hasRSSSlope = rssAnalysis.slope, rssAnalysis.trend, rssAnalysis.ok
	writeHumanServiceSummary(os.Stdout, unit, manager, summary, memoryAnalysis, rssAnalysis)
	writeServiceProcessSummary(os.Stdout, unit, summary)
	return nil
}

func validateServiceResourceSummaryIdentity(states []serviceResourceState) error {
	if len(states) == 0 {
		return nil
	}
	first := states[0]
	for _, reason := range first.IncompleteReasons {
		if reason == "identity_changed" {
			return fmt.Errorf("resource measurements contain a measurement that crossed service identity")
		}
	}
	for _, state := range states[1:] {
		for _, reason := range state.IncompleteReasons {
			if reason == "identity_changed" {
				return fmt.Errorf("resource measurements contain a measurement that crossed service identity")
			}
		}
		if state.Cgroup != first.Cgroup || state.InvocationID != first.InvocationID {
			return fmt.Errorf("resource measurements span multiple service identities; use a narrower period")
		}
	}
	return nil
}

func readServiceResourceJournal(path string) ([]serviceResourceState, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open resource state journal %q: %w", path, err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	states := make([]serviceResourceState, 0)
	for {
		line, complete, err := readServiceResourceJournalLine(reader)
		if err != nil {
			return nil, fmt.Errorf("read resource state journal %q: %w", path, err)
		}
		if !complete {
			// A crash may leave only the final partial JSON line. The journal
			// remains readable; the incomplete record is not a measurement.
			return states, nil
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		var state serviceResourceState
		if err := decoder.Decode(&state); err != nil {
			return nil, fmt.Errorf("decode resource state journal %q: %w", path, err)
		}
		if err := state.validate(); err != nil {
			return nil, fmt.Errorf("validate resource state journal %q: %w", path, err)
		}
		states = append(states, state)
	}
}

func readServiceResourceJournalLine(reader *bufio.Reader) ([]byte, bool, error) {
	line := make([]byte, 0, 1024)
	for {
		chunk, err := reader.ReadSlice('\n')
		if len(line)+len(chunk) > maxServiceResourceRecord+1 {
			return nil, false, fmt.Errorf("resource state journal record exceeds %d bytes", maxServiceResourceRecord)
		}
		line = append(line, chunk...)
		if err == nil {
			return line, true, nil
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			return line, false, nil
		}
		return nil, false, err
	}
}

func writeHumanServiceSummary(out interface{ Write([]byte) (int, error) }, unit, manager string, summary serviceResourceSummary, memoryAnalysis, rssAnalysis serviceGaugeAnalysis) {
	state := "complete"
	if summary.incomplete > 0 || summary.processIncomplete {
		state = "incomplete"
	}
	duration := summary.last.Sub(summary.first).Round(time.Second)
	if duration < 0 {
		duration = 0
	}
	trend := humanServiceTrend(summary.memoryTrend)
	fmt.Fprintf(out, "Service %s was observed for %s; memory was %s; observation %s.\n", unit, duration, trend, state)
	fmt.Fprintf(out, "Memory: %s -> %s; peak %s; trend %s; limit %s.\n", humanServiceMetric(summary.memoryStart, summary.hasMemory), humanServiceMetric(summary.memoryEnd, summary.hasMemory), humanServiceMetric(summary.memoryMax, summary.hasMemory), humanServiceTrend(memoryAnalysis.trend), humanServiceLimit(summary.memoryLimit))
	writeHumanServiceTimeline(out, "Memory", memoryAnalysis)
	fmt.Fprintf(out, "RSS: %s -> %s; peak %s; trend %s.\n", humanServiceMetric(summary.rssStart, summary.hasRSS), humanServiceMetric(summary.rssEnd, summary.hasRSS), humanServiceMetric(summary.rssMax, summary.hasRSS), humanServiceTrend(rssAnalysis.trend))
	writeHumanServiceTimeline(out, "RSS", rssAnalysis)
	if summary.hasCPU {
		fmt.Fprintf(out, "CPU time used: %s.\n", formatServiceDuration(summary.cpuDelta))
	} else {
		fmt.Fprintln(out, "CPU time used: not available.")
	}
	if summary.hasOOM {
		fmt.Fprintf(out, "OOM kills: %d.\n", summary.oomKillDelta)
	} else {
		fmt.Fprintln(out, "OOM kills: not available.")
	}
	writeHumanServiceIODelta(out, summary)
	fmt.Fprintf(out, "Scope: %s manager; %d measurement record(s), %d sample(s), from %s to %s.\n", manager, summary.records, summary.samples, summary.first.UTC().Format(time.RFC3339), summary.last.UTC().Format(time.RFC3339))
	if summary.incomplete > 0 || summary.processIncomplete {
		fmt.Fprintln(out, "Completeness: some observations were incomplete; the report describes evidence, not a proven cause.")
	} else {
		fmt.Fprintln(out, "Completeness: all collected observations were complete; the report describes evidence, not a proven cause.")
	}
}

func writeHumanServiceIODelta(out interface{ Write([]byte) (int, error) }, summary serviceResourceSummary) {
	if !summary.ioAvailable {
		fmt.Fprintln(out, "Block I/O: not available.")
		return
	}
	if len(summary.ioDeltas) == 0 {
		fmt.Fprintln(out, "Block I/O: no per-device counter increase was observed.")
	} else {
		devices := make([]string, 0, len(summary.ioDeltas))
		for device := range summary.ioDeltas {
			devices = append(devices, device)
		}
		sort.Strings(devices)
		fmt.Fprint(out, "Block I/O:")
		for _, device := range devices {
			delta := summary.ioDeltas[device]
			fmt.Fprintf(out, " %s read +%d B/%d I/O, write +%d B/%d I/O, discard +%d B/%d I/O;", device, delta.rbytes, delta.rios, delta.wbytes, delta.wios, delta.dbytes, delta.dios)
		}
		fmt.Fprintln(out)
	}
	if summary.ioIncomplete || summary.ioUnavailable {
		fmt.Fprintln(out, "Block I/O completeness: some samples were unavailable or had a changed device set.")
	}
}

func writeHumanServiceTimeline(out interface{ Write([]byte) (int, error) }, name string, analysis serviceGaugeAnalysis) {
	if analysis.peakAt.IsZero() {
		fmt.Fprintf(out, "%s timeline: no increase was observed.\n", name)
		return
	}
	if analysis.firstIncreaseAt.IsZero() {
		fmt.Fprintf(out, "%s timeline: no increase was observed; peak %s.\n", name, humanServiceTime(analysis.peakAt))
		return
	}
	fmt.Fprintf(out, "%s timeline: first observed increase %s; peak %s; last observed increase %s.\n", name, humanServiceTime(analysis.firstIncreaseAt), humanServiceTime(analysis.peakAt), humanServiceTime(analysis.lastIncreaseAt))
}

func humanServiceMetric(value uint64, present bool) string {
	if !present {
		return "not available"
	}
	return formatServiceBytes(value)
}

func humanServiceLimit(value string) string {
	if value == "" || value == "unavailable" {
		return "not available"
	}
	if value == "max" {
		return "unlimited"
	}
	n, ok := parseServiceResourceUint(value)
	if !ok {
		return value
	}
	return formatServiceBytes(n)
}

func humanServiceTime(value time.Time) string {
	if value.IsZero() {
		return "not available"
	}
	return value.UTC().Format("2006-01-02 15:04:05Z07:00")
}

func humanServiceTrend(value string) string {
	switch value {
	case "rising":
		return "rising"
	case "falling":
		return "falling"
	case "volatile":
		return "volatile"
	case "stable":
		return "stable"
	default:
		return "not available"
	}
}

func formatServiceDuration(microseconds uint64) string {
	seconds := float64(microseconds) / 1_000_000
	if seconds < 1 {
		return fmt.Sprintf("%.0f ms", seconds*1000)
	}
	if seconds < 60 {
		return fmt.Sprintf("%.2f s", seconds)
	}
	minutes := int(seconds / 60)
	remaining := int(seconds) % 60
	return fmt.Sprintf("%dm %ds", minutes, remaining)
}

func updateGauge(start, end, maximum *uint64, present *bool, firstText, lastText string, samples []serviceResourceStateSample, key string) {
	first, ok := parseServiceResourceUint(firstText)
	if !ok {
		return
	}
	last, ok := parseServiceResourceUint(lastText)
	if !ok {
		return
	}
	if !*present {
		*start, *maximum, *present = first, first, true
	}
	*end = last
	if samples == nil {
		if last > *maximum {
			*maximum = last
		}
		return
	}
	for _, sample := range samples {
		if value, ok := parseServiceResourceUint(sample.Metrics[key]); ok && value > *maximum {
			*maximum = value
		}
	}
}

func summaryUint(value uint64, present bool) string {
	if !present {
		return "unavailable"
	}
	return fmt.Sprintf("%d", value)
}

func summaryDelta(start, end uint64, present bool) string {
	if !present {
		return "unavailable"
	}
	if end >= start {
		return "+" + strconv.FormatUint(end-start, 10)
	}
	return "-" + strconv.FormatUint(start-end, 10)
}

func summaryTrend(start, end uint64, present bool) string {
	if !present {
		return "unavailable"
	}
	if end > start {
		return "rising"
	}
	if end < start {
		return "falling"
	}
	return "stable"
}

func analyzeServiceGauge(points []serviceGaugePoint) (float64, string, bool) {
	analysis := analyzeServiceGaugeDetails(points)
	return analysis.slope, analysis.trend, analysis.ok
}

func analyzeServiceGaugeDetails(points []serviceGaugePoint) serviceGaugeAnalysis {
	if len(points) < 2 {
		return serviceGaugeAnalysis{trend: "unavailable"}
	}
	base := points[0].at
	var sumTime, sumValue float64
	for _, point := range points {
		sumTime += point.at.Sub(base).Seconds()
		sumValue += point.value
	}
	meanTime := sumTime / float64(len(points))
	meanValue := sumValue / float64(len(points))
	var covariance, variance float64
	hasPositive, hasNegative := false, false
	for index, point := range points {
		timeValue := point.at.Sub(base).Seconds() - meanTime
		value := point.value - meanValue
		covariance += timeValue * value
		variance += timeValue * timeValue
		if index > 0 {
			delta := point.value - points[index-1].value
			if delta > 0 {
				hasPositive = true
			} else if delta < 0 {
				hasNegative = true
			}
		}
	}
	if variance == 0 {
		return serviceGaugeAnalysis{trend: "unavailable"}
	}
	trend := "stable"
	switch {
	case hasPositive && hasNegative:
		trend = "volatile"
	case hasPositive:
		trend = "rising"
	case hasNegative:
		trend = "falling"
	}
	analysis := serviceGaugeAnalysis{slope: covariance / variance, trend: trend, ok: true, peakAt: points[0].at, peakValue: points[0].value}
	for index := 1; index < len(points); index++ {
		if points[index].value > points[index-1].value {
			if analysis.firstIncreaseAt.IsZero() {
				analysis.firstIncreaseAt = points[index].at
			}
			analysis.lastIncreaseAt = points[index].at
		}
		if points[index].value > analysis.peakValue {
			analysis.peakValue = points[index].value
			analysis.peakAt = points[index].at
		}
	}
	return analysis
}

func summarySlope(value float64, present bool) string {
	if !present {
		return "unavailable"
	}
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func summaryTime(value time.Time) string {
	if value.IsZero() {
		return "unavailable"
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func summaryGaugeValue(value float64, present bool) string {
	if !present {
		return "unavailable"
	}
	return strconv.FormatUint(uint64(value), 10)
}
