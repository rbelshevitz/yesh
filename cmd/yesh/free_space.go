package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func parseByteSize(text string) (uint64, error) {
	units := []struct {
		name  string
		value uint64
	}{
		{"B", 1}, {"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30}, {"TIB", 1 << 40},
	}
	upper := strings.ToUpper(strings.TrimSpace(text))
	for _, unit := range units {
		if !strings.HasSuffix(upper, unit.name) {
			continue
		}
		digits := strings.TrimSuffix(upper, unit.name)
		if digits == "" {
			continue
		}
		amount, err := strconv.ParseUint(digits, 10, 64)
		if err != nil || amount == 0 {
			continue
		}
		if amount > math.MaxUint64/unit.value {
			continue
		}
		return amount * unit.value, nil
	}
	return 0, fmt.Errorf("free_space expects a whole size such as 20GiB")
}
