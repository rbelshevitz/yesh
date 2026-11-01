//go:build linux

package main

import (
	"sort"
	"strconv"
	"strings"
)

type serviceIODelta struct {
	rbytes, wbytes uint64
	rios, wios     uint64
	dbytes, dios   uint64
}

var serviceIOStatCounters = []string{"rbytes", "wbytes", "rios", "wios", "dbytes", "dios"}

func compareServiceIOStats(first, last map[string]string) (map[string]serviceIODelta, bool, bool) {
	firstValues, firstAvailable := serviceIOStatValues(first)
	lastValues, lastAvailable := serviceIOStatValues(last)
	if !firstAvailable || !lastAvailable {
		return nil, false, false
	}
	deltas := make(map[string]serviceIODelta)
	complete := true
	keys := make(map[string]struct{}, len(firstValues)+len(lastValues))
	for key := range firstValues {
		keys[key] = struct{}{}
	}
	for key := range lastValues {
		keys[key] = struct{}{}
	}
	for key := range keys {
		before, beforeOK := firstValues[key]
		after, afterOK := lastValues[key]
		if !beforeOK || !afterOK || after < before {
			complete = false
			continue
		}
		device, counter, ok := strings.Cut(key, "\x00")
		if !ok {
			complete = false
			continue
		}
		delta := after - before
		value := deltas[device]
		switch counter {
		case "rbytes":
			value.rbytes = delta
		case "wbytes":
			value.wbytes = delta
		case "rios":
			value.rios = delta
		case "wios":
			value.wios = delta
		case "dbytes":
			value.dbytes = delta
		case "dios":
			value.dios = delta
		}
		deltas[device] = value
	}
	return deltas, true, complete
}

func serviceIOStatValues(metrics map[string]string) (map[string]uint64, bool) {
	if metrics["io.stat.available"] != "yes" {
		return nil, false
	}
	values := make(map[string]uint64)
	for key, value := range metrics {
		if !strings.HasPrefix(key, "io.stat.") || key == "io.stat.available" {
			continue
		}
		rest := strings.TrimPrefix(key, "io.stat.")
		device, counter, ok := strings.Cut(rest, ".")
		if !ok || device == "" || !containsServiceIOStatCounter(counter) {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			continue
		}
		values[device+"\x00"+counter] = parsed
	}
	return values, true
}

func containsServiceIOStatCounter(value string) bool {
	for _, counter := range serviceIOStatCounters {
		if counter == value {
			return true
		}
	}
	return false
}

func mergeServiceIODeltas(target map[string]serviceIODelta, source map[string]serviceIODelta) bool {
	devices := make([]string, 0, len(source))
	for device := range source {
		devices = append(devices, device)
	}
	sort.Strings(devices)
	for _, device := range devices {
		before := target[device]
		after := source[device]
		if ^uint64(0)-before.rbytes < after.rbytes || ^uint64(0)-before.wbytes < after.wbytes ||
			^uint64(0)-before.rios < after.rios || ^uint64(0)-before.wios < after.wios ||
			^uint64(0)-before.dbytes < after.dbytes || ^uint64(0)-before.dios < after.dios {
			return false
		}
		target[device] = serviceIODelta{
			rbytes: before.rbytes + after.rbytes, wbytes: before.wbytes + after.wbytes,
			rios: before.rios + after.rios, wios: before.wios + after.wios,
			dbytes: before.dbytes + after.dbytes, dios: before.dios + after.dios,
		}
	}
	return true
}
