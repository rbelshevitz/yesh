package yeshcore

import yeshversion "yesh/internal/version"

// Version reports the parser/runtime version embedded in this binary.
func Version() string {
	return yeshversion.Version
}
