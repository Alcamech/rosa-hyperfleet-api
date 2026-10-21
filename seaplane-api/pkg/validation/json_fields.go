package validation

import "strings"

// HasJSONField reports whether a dotted field path exists in a decoded JSON
// object. Field names are matched case-insensitively, like encoding/json's
// struct decoding. A field explicitly set to null, an empty string, or a zero
// value is present; only an omitted path returns false.
func HasJSONField(object map[string]any, fieldPath string) bool {
	if object == nil || fieldPath == "" {
		return false
	}

	parts := strings.Split(fieldPath, ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
	}
	return hasJSONFieldPath(object, parts)
}

func hasJSONFieldPath(value any, parts []string) bool {
	if len(parts) == 0 {
		return true
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return false
	}

	for name, child := range fields {
		if strings.EqualFold(name, parts[0]) && hasJSONFieldPath(child, parts[1:]) {
			return true
		}
	}
	return false
}
