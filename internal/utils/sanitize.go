package utils

import (
	"strings"
	"unicode"
)

func SanitizeForPath(s string) string {
	if s == "" {
		return "unknown"
	}
	
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	
	return sb.String()
}