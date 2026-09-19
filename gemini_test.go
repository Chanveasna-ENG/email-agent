package main

import (
	"testing"

	"google.golang.org/genai"
)

func TestBuildGeminiContents(t *testing.T) {
	turns := []ConversationTurn{
		{Role: "user", Content: "Hello world"},
		{Role: "model", Content: "Hello! How can I help?"},
		{Role: "user", Content: "Tell me a joke"},
	}

	contents := buildGeminiContents(turns)
	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	expectedRoles := []genai.Role{"user", "model", "user"}
	for i, c := range contents {
		if c.Role != string(expectedRoles[i]) {
			t.Errorf("content[%d].Role = %q, want %q", i, c.Role, expectedRoles[i])
		}
		if len(c.Parts) != 1 {
			t.Errorf("content[%d] expected 1 part, got %d", i, len(c.Parts))
		}
		if c.Parts[0].Text != turns[i].Content {
			t.Errorf("content[%d].Parts[0].Text = %q, want %q", i, c.Parts[0].Text, turns[i].Content)
		}
	}
}

func TestBuildGeminiContentsEmpty(t *testing.T) {
	contents := buildGeminiContents(nil)
	if len(contents) != 0 {
		t.Errorf("expected 0 contents for nil turns, got %d", len(contents))
	}
}
