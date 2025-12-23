package utils

import (
	"strings"
)

func ExtractJSONFromContent(s string) string {
	if idx := strings.Index(s, "```"); idx >= 0 {
		rest := s[idx+3:]
		if strings.HasPrefix(rest, "json") {
			rest = rest[4:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			return strings.TrimSpace(rest[:end])
		}
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end >= 0 && end >= start {
		return s[start : end+1]
	}
	return s
}
