package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	bracketTagRegex = regexp.MustCompile(`(?i)\[([a-zA-Z0-9_\-]+)\]`)
	slashTagRegex   = regexp.MustCompile(`(?i)(?:^|\s)/([a-zA-Z0-9_\-]+)`)
)

// SkillMeta contains metadata and instructions for a parsed skill.
type SkillMeta struct {
	Name        string
	Description string
	Path        string
	Prompt      string
}

// ParseSkillFile reads a SKILL.md or text file, extracting YAML frontmatter if present.
func ParseSkillFile(filePath string) (*SkillMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read skill file: %w", err)
	}

	content := string(data)
	meta := &SkillMeta{
		Path: filePath,
	}

	// Default name based on parent folder or file base
	dirName := filepath.Base(filepath.Dir(filePath))
	fileName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	if strings.EqualFold(fileName, "SKILL") && dirName != "." && dirName != "/" {
		meta.Name = strings.ToLower(dirName)
	} else {
		meta.Name = strings.ToLower(fileName)
	}

	// Check for YAML frontmatter between --- fences
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "---") {
		parts := strings.SplitN(trimmed[3:], "---", 2)
		if len(parts) == 2 {
			frontmatter := parts[0]
			body := strings.TrimSpace(parts[1])

			scanner := bufio.NewScanner(strings.NewReader(frontmatter))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(strings.ToLower(line), "name:") {
					val := strings.TrimSpace(strings.TrimPrefix(line[5:], " "))
					val = strings.Trim(val, `"'`)
					if val != "" {
						meta.Name = strings.ToLower(val)
					}
				} else if strings.HasPrefix(strings.ToLower(line), "description:") {
					val := strings.TrimSpace(strings.TrimPrefix(line[12:], " "))
					val = strings.Trim(val, `"'`)
					meta.Description = val
				}
			}

			meta.Prompt = body
			return meta, nil
		}
	}

	meta.Prompt = trimmed
	return meta, nil
}

// ListSkills searches skillsDir for all valid skills (in skills/<name>/SKILL.md or flat files).
func ListSkills(skillsDir string) ([]SkillMeta, error) {
	if skillsDir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var list []SkillMeta
	seen := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillPath); err == nil {
				if meta, err := ParseSkillFile(skillPath); err == nil {
					if !seen[meta.Name] {
						seen[meta.Name] = true
						list = append(list, *meta)
					}
				}
			}
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".md" || ext == ".txt" {
				skillPath := filepath.Join(skillsDir, entry.Name())
				if meta, err := ParseSkillFile(skillPath); err == nil {
					if !seen[meta.Name] {
						seen[meta.Name] = true
						list = append(list, *meta)
					}
				}
			}
		}
	}

	return list, nil
}

// ResolveSkill resolves which skill to use based on [tag] or /tag in subject or body.
func ResolveSkill(skillsDir, subject, body, fallbackPrompt string) (string, string) {
	tag := extractTag(subject)
	if tag == "" {
		tag = extractTagFromFirstLine(body)
	}

	cleanTag := strings.ToLower(strings.TrimSpace(tag))

	if cleanTag != "" && skillsDir != "" {
		// 1. Check skills/<tag>/SKILL.md
		dirSkill := filepath.Join(skillsDir, cleanTag, "SKILL.md")
		if meta, err := ParseSkillFile(dirSkill); err == nil && len(meta.Prompt) > 0 {
			return meta.Prompt, meta.Name
		}
		// 2. Check flat file: skills/<tag>.md or .txt
		for _, ext := range []string{".md", ".txt"} {
			flatSkill := filepath.Join(skillsDir, cleanTag+ext)
			if meta, err := ParseSkillFile(flatSkill); err == nil && len(meta.Prompt) > 0 {
				return meta.Prompt, meta.Name
			}
		}
	}

	// Fallback to default skill
	if skillsDir != "" {
		defaultDir := filepath.Join(skillsDir, "default", "SKILL.md")
		if meta, err := ParseSkillFile(defaultDir); err == nil && len(meta.Prompt) > 0 {
			return meta.Prompt, "default"
		}
		for _, ext := range []string{".md", ".txt"} {
			defaultFlat := filepath.Join(skillsDir, "default"+ext)
			if meta, err := ParseSkillFile(defaultFlat); err == nil && len(meta.Prompt) > 0 {
				return meta.Prompt, "default"
			}
		}
	}

	return strings.TrimSpace(fallbackPrompt), "default"
}

// FormatSkillsOverview formats a list of skills as a summary block for prompting.
func FormatSkillsOverview(skills []SkillMeta) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Available skills:\n")
	for _, s := range skills {
		desc := s.Description
		if desc == "" {
			desc = "Specialized skill"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, desc))
	}
	return sb.String()
}

func extractTag(text string) string {
	if matches := bracketTagRegex.FindStringSubmatch(text); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	if matches := slashTagRegex.FindStringSubmatch(text); len(matches) > 1 {
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
