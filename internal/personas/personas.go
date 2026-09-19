package personas

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var tagRegex = regexp.MustCompile(`(?i)\[([a-zA-Z0-9_\-]+)\]`)

// ResolvePersona determines which persona prompt to use based on [tag] syntax in the subject or body.
func ResolvePersona(personasDir, subject, body, fallbackPrompt string) (string, string) {
	tag := extractTag(subject)
	if tag == "" {
		tag = extractTagFromFirstLine(body)
	}

	if tag != "" && personasDir != "" {
		tagPath := filepath.Join(personasDir, strings.ToLower(tag)+".txt")
		if content, err := os.ReadFile(tagPath); err == nil && len(strings.TrimSpace(string(content))) > 0 {
			return strings.TrimSpace(string(content)), strings.ToLower(tag)
		}
	}

	// Fallback to default.txt in personasDir
	if personasDir != "" {
		defaultPath := filepath.Join(personasDir, "default.txt")
		if content, err := os.ReadFile(defaultPath); err == nil && len(strings.TrimSpace(string(content))) > 0 {
			return strings.TrimSpace(string(content)), "default"
		}
	}

	return strings.TrimSpace(fallbackPrompt), "default"
}

func extractTag(text string) string {
	matches := tagRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractTagFromFirstLine(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) > 0 {
		return extractTag(lines[0])
	}
	return ""
}
