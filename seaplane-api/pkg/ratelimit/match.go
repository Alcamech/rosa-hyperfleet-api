package ratelimit

import "strings"

func matchPath(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, pp := range patternParts {
		if pp == "*" {
			if pathParts[i] == "" {
				return false
			}
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}

	return true
}
