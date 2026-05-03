package email

import (
	"strconv"
	"strings"
)

func ParseAcceptLanguage(header string) string {
	bestLanguage := ""
	bestQ := -1.0
	for _, rawPart := range strings.Split(header, ",") {
		tag, q := parseAcceptLanguagePart(rawPart)
		if tag == "" || q < bestQ {
			continue
		}
		language, ok := supportedLanguageTag(tag)
		if !ok {
			continue
		}
		bestLanguage = language
		bestQ = q
	}
	if bestLanguage == "" {
		return "en"
	}
	return bestLanguage
}

func ResolveLanguage(candidates ...string) string {
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if language, ok := supportedLanguageTag(trimmed); ok {
			return language
		}
	}
	return "en"
}

func parseAcceptLanguagePart(rawPart string) (string, float64) {
	segments := strings.Split(strings.TrimSpace(rawPart), ";")
	if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
		return "", 0
	}

	q := 1.0
	for _, segment := range segments[1:] {
		keyValue := strings.SplitN(strings.TrimSpace(segment), "=", 2)
		if len(keyValue) != 2 || strings.TrimSpace(keyValue[0]) != "q" {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(keyValue[1]), 64)
		if err == nil {
			q = parsed
		}
	}
	return strings.TrimSpace(segments[0]), q
}

func supportedLanguageTag(tag string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(tag))
	switch {
	case normalized == "zh" || strings.HasPrefix(normalized, "zh-"):
		return "zh", true
	case normalized == "en" || strings.HasPrefix(normalized, "en-"):
		return "en", true
	default:
		return "", false
	}
}
