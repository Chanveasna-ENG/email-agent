package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePersona(t *testing.T) {
	tempDir := t.TempDir()

	coderPrompt := "You are an expert software engineer."
	defaultPrompt := "You are a helpful email assistant."
	fallbackPrompt := "Fallback prompt."

	if err := os.WriteFile(filepath.Join(tempDir, "coder.txt"), []byte(coderPrompt), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "default.txt"), []byte(defaultPrompt), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		subject      string
		body         string
		wantPrompt   string
		wantPersona  string
	}{
		{
			name:        "Subject with tag",
			subject:     "[coder] Review this implementation",
			body:        "Here is the code",
			wantPrompt:  coderPrompt,
			wantPersona: "coder",
		},
		{
			name:        "Body with tag on first line",
			subject:     "Question about architecture",
			body:        "[coder]\nWhat do you think of this design?",
			wantPrompt:  coderPrompt,
			wantPersona: "coder",
		},
		{
			name:        "No tag uses default.txt",
			subject:     "General inquiry",
			body:        "Hello there",
			wantPrompt:  defaultPrompt,
			wantPersona: "default",
		},
		{
			name:        "Unknown tag falls back to default.txt",
			subject:     "[astronaut] Launch sequence",
			body:        "Status report",
			wantPrompt:  defaultPrompt,
			wantPersona: "default",
		},
		{
			name:        "Empty personasDir uses fallback",
			subject:     "Hello",
			body:        "World",
			wantPrompt:  fallbackPrompt,
			wantPersona: "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempDir
			if tc.name == "Empty personasDir uses fallback" {
				dir = ""
			}
			prompt, name := ResolvePersona(dir, tc.subject, tc.body, fallbackPrompt)
			if prompt != tc.wantPrompt {
				t.Errorf("prompt = %q, want %q", prompt, tc.wantPrompt)
			}
			if name != tc.wantPersona {
				t.Errorf("personaName = %q, want %q", name, tc.wantPersona)
			}
		})
	}
}
