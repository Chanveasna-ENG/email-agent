package main

import (
	"testing"

	"google.golang.org/genai"
)

func TestBuildGeminiContents(t *testing.T) {
	turns := []ConversationTurn{
		{Role: "user", Content: "Hello world"},
		{Role: "model", Content: "Hello! How can I help?"},
		{
			Role:    "user",
			Content: "Inspect this attachment",
			Attachments: []Attachment{
				{
					Filename:    "doc.pdf",
					ContentType: "application/pdf",
					Data:        []byte("%PDF-fake-header"),
				},
			},
		},
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
	}

	// Verify multimodal turn 2 has both text part and binary part
	if len(contents[2].Parts) != 2 {
		t.Fatalf("content[2] expected 2 parts, got %d", len(contents[2].Parts))
	}
	if contents[2].Parts[0].Text != "Inspect this attachment" {
		t.Errorf("content[2].Parts[0].Text = %q, want 'Inspect this attachment'", contents[2].Parts[0].Text)
	}
	if contents[2].Parts[1].InlineData == nil || contents[2].Parts[1].InlineData.MIMEType != "application/pdf" {
		t.Errorf("content[2].Parts[1] missing InlineData or wrong MIME type: %v", contents[2].Parts[1].InlineData)
	}
}

func TestBuildGeminiContentsEmpty(t *testing.T) {
	contents := buildGeminiContents(nil)
	if len(contents) != 0 {
		t.Errorf("expected 0 contents for nil turns, got %d", len(contents))
	}
}
