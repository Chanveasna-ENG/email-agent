package parser

import (
	"strings"
	"testing"
	"time"

	"email-agent/internal/models"
)

func TestExtractSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{
			name:     "empty input",
			raw:      "",
			expected: "",
		},
		{
			name:     "NONE keyword",
			raw:      "NONE",
			expected: "",
		},
		{
			name:     "NONE with spaces and newline",
			raw:      "  NONE \n",
			expected: "",
		},
		{
			name:     "NONE with period",
			raw:      "None.",
			expected: "",
		},
		{
			name:     "Quoted search terms with following reasoning",
			raw:      "\"budget timeline 2026\"\nReasoning: need to check previous agreement",
			expected: "budget timeline 2026",
		},
		{
			name:     "Prefix keywords:",
			raw:      "Keywords: project roadmap release",
			expected: "project roadmap release",
		},
		{
			name:     "Clean keywords",
			raw:      "invoice payment status",
			expected: "invoice payment status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSearchQuery(tt.raw)
			if got != tt.expected {
				t.Errorf("ExtractSearchQuery(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}

func TestFormatArchiveContext(t *testing.T) {
	date1 := time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
	date2 := time.Date(2026, 9, 21, 10, 15, 0, 0, time.UTC)

	matches := []*models.EmailMessage{
		{
			MessageID: "<msg1@test>",
			Date:      date1,
			Sender:    "alice@test.com",
			Subject:   "Roadmap discussion",
			BodyText:  "We agreed on Q4 delivery.",
		},
		{
			MessageID: "<current@test>",
			Date:      date2,
			Sender:    "alice@test.com",
			Subject:   "Follow up",
			BodyText:  "Current incoming email.",
		},
	}

	result := FormatArchiveContext(matches, "<current@test>")

	if strings.Contains(result, "Current incoming email") {
		t.Errorf("expected current message to be excluded from archive context")
	}
	if !strings.Contains(result, "[Relevant Past Archive Emails]:") {
		t.Errorf("expected header [Relevant Past Archive Emails] in result")
	}
	if !strings.Contains(result, "2026-09-20 14:30") || !strings.Contains(result, "Roadmap discussion") {
		t.Errorf("expected past email content in archive context, got:\n%s", result)
	}

	// Empty matches or only current message
	emptyResult := FormatArchiveContext([]*models.EmailMessage{matches[1]}, "<current@test>")
	if emptyResult != "" {
		t.Errorf("expected empty string when only current message matches, got %q", emptyResult)
	}
}

func TestBuildReplyPrompt(t *testing.T) {
	msg := &models.EmailMessage{
		MessageID: "<inbound@test>",
		Sender:    "Alice <alice@test.com>",
		Subject:   "Project Sync",
		Date:      time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC),
		BodyText:  "What did we decide on the budget?",
	}

	turns := []models.ConversationTurn{
		{Role: "user", Content: "Initial inquiry"},
		{Role: "model", Content: "Initial reply"},
		{Role: "user", Content: "What did we decide on the budget?"},
	}

	archive := "[Relevant Past Archive Emails]:\n- [2026-09-20 14:30] Budget was $5,000"
	sysPrompt := "You are a concise assistant."

	prompt := BuildReplyPrompt(sysPrompt, msg, turns, archive)

	if !strings.Contains(prompt, "[System Instructions]:\nYou are a concise assistant.") {
		t.Errorf("prompt missing system prompt block")
	}
	if !strings.Contains(prompt, "Inbound email from: Alice <alice@test.com>") {
		t.Errorf("prompt missing inbound header")
	}
	if !strings.Contains(prompt, "[Conversation Thread History]:") {
		t.Errorf("prompt missing conversation thread history block")
	}
	if !strings.Contains(prompt, "Turn 1 (User):\nInitial inquiry") {
		t.Errorf("prompt missing Turn 1")
	}
	if !strings.Contains(prompt, "Turn 2 (Assistant):\nInitial reply") {
		t.Errorf("prompt missing Turn 2 with Assistant label")
	}
	if !strings.Contains(prompt, archive) {
		t.Errorf("prompt missing archive section")
	}
	if !strings.Contains(prompt, "Please write a concise, professional reply to the sender.") {
		t.Errorf("prompt missing ending instruction")
	}
}
