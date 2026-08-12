package handlers

import (
	"math"
	"strings"
)

// redact masks the first half of s with asterisks for safe logging of
// customer identifiers.
func redact(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	half := int(math.Ceil(float64(len(runes)) / 2))
	return strings.Repeat("*", half) + string(runes[half:])
}
