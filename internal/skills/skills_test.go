package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillFile(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "coder")
	_ = os.MkdirAll(skillDir, 0755)
	skillPath := filepath.Join(skillDir, "SKILL.md")

	content := `---
name: coder
description: Senior software engineer for writing and reviewing code.
---
# Coder Skill
You are an expert software engineer.
`
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed writing test SKILL.md: %v", err)
	}

	meta, err := ParseSkillFile(skillPath)
	if err != nil {
		t.Fatalf("ParseSkillFile failed: %v", err)
	}

	if meta.Name != "coder" {
		t.Errorf("meta.Name = %q, want 'coder'", meta.Name)
	}
	if meta.Description != "Senior software engineer for writing and reviewing code." {
		t.Errorf("meta.Description = %q", meta.Description)
	}
	if !strings.Contains(meta.Prompt, "You are an expert software engineer.") {
		t.Errorf("meta.Prompt missing expected content: %q", meta.Prompt)
	}
}

func TestResolveSkill(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create skills/default/SKILL.md
	defaultDir := filepath.Join(tempDir, "default")
	_ = os.MkdirAll(defaultDir, 0755)
	_ = os.WriteFile(filepath.Join(defaultDir, "SKILL.md"), []byte(`---
name: default
description: Default assistant
---
I am the default skill.`), 0644)

	// 2. Create skills/coder/SKILL.md
	coderDir := filepath.Join(tempDir, "coder")
	_ = os.MkdirAll(coderDir, 0755)
	_ = os.WriteFile(filepath.Join(coderDir, "SKILL.md"), []byte(`---
name: coder
description: Coder assistant
---
I am the coder skill.`), 0644)

	// Case 1: Subject tag with bracket
	prompt, name := ResolveSkill(tempDir, "Bug in Go [coder]", "Please check this error", "fallback")
	if name != "coder" || !strings.Contains(prompt, "I am the coder skill.") {
		t.Errorf("expected coder skill, got name=%q, prompt=%q", name, prompt)
	}

	// Case 2: Body first line with slash tag
	prompt, name = ResolveSkill(tempDir, "Help needed", "/coder please look at this function", "fallback")
	if name != "coder" || !strings.Contains(prompt, "I am the coder skill.") {
		t.Errorf("expected coder skill from slash tag, got name=%q, prompt=%q", name, prompt)
	}

	// Case 3: Untagged email falls back to default
	prompt, name = ResolveSkill(tempDir, "Hello there", "How is the weather?", "fallback")
	if name != "default" || !strings.Contains(prompt, "I am the default skill.") {
		t.Errorf("expected default skill for untagged email, got name=%q, prompt=%q", name, prompt)
	}

	// Case 4: Non-existent tag falls back to default
	prompt, name = ResolveSkill(tempDir, "[astronaut] Launch", "Go to space", "fallback")
	if name != "default" || !strings.Contains(prompt, "I am the default skill.") {
		t.Errorf("expected default skill fallback for unknown tag, got name=%q, prompt=%q", name, prompt)
	}

	// Case 5: Empty skillsDir falls back to fallbackPrompt
	prompt, name = ResolveSkill("", "Hello", "World", "fallback prompt")
	if name != "default" || prompt != "fallback prompt" {
		t.Errorf("expected fallback prompt when skillsDir empty, got name=%q, prompt=%q", name, prompt)
	}
}

func TestListSkillsAndOverview(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempDir, "default"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "default", "SKILL.md"), []byte(`---
name: default
description: General assistant
---
Prompt`), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "writer"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "writer", "SKILL.md"), []byte(`---
name: writer
description: Prose editor
---
Prompt`), 0644)

	list, err := ListSkills(tempDir)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(list))
	}

	overview := FormatSkillsOverview(list)
	if !strings.Contains(overview, "default: General assistant") || !strings.Contains(overview, "writer: Prose editor") {
		t.Errorf("overview missing expected entries: %s", overview)
	}
}
