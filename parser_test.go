package main

import (
	"reflect"
	"testing"
)

func TestExtractEmailAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alice Doe <ALICE@Example.COM>", "alice@example.com"},
		{"bob@test.com", "bob@test.com"},
		{"<carol@sample.org>", "carol@sample.org"},
		{"", ""},
		{"   ", ""},
	}

	for _, tc := range tests {
		got := ExtractEmailAddress(tc.input)
		if got != tc.expected {
			t.Errorf("ExtractEmailAddress(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestParseReferences(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"<id1@mail> <id2@mail>", []string{"<id1@mail>", "<id2@mail>"}},
		{"<single@mail>", []string{"<single@mail>"}},
		{"invalid without brackets", nil},
		{"", nil},
	}

	for _, tc := range tests {
		got := ParseReferences(tc.input)
		if !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("ParseReferences(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestStripQuotedReply(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Gmail style quote",
			input:    "Hello agent!\n\nOn Fri, Oct 10 wrote:\n> past message here",
			expected: "Hello agent!",
		},
		{
			name:     "Outlook style quote",
			input:    "Please help me.\n--- Original Message ---\nFrom: Bob\nOld message",
			expected: "Please help me.",
		},
		{
			name:     "Leading angle bracket quotes",
			input:    "> quote line 1\n> quote line 2\nActual new response",
			expected: "Actual new response",
		},
		{
			name:     "No quote",
			input:    "Just a plain email message.",
			expected: "Just a plain email message.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StripQuotedReply(tc.input)
			if got != tc.expected {
				t.Errorf("StripQuotedReply() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestIsLoopOrAutoReply(t *testing.T) {
	tests := []struct {
		name          string
		sender        string
		agent         string
		autoSubmitted string
		xAutoReply    string
		expected      bool
	}{
		{
			name:          "Self message",
			sender:        "bot@gmail.com",
			agent:         "bot@gmail.com",
			autoSubmitted: "",
			xAutoReply:    "",
			expected:      true,
		},
		{
			name:          "Auto-submitted header",
			sender:        "user@gmail.com",
			agent:         "bot@gmail.com",
			autoSubmitted: "auto-replied",
			xAutoReply:    "",
			expected:      true,
		},
		{
			name:          "X-Autoreply header",
			sender:        "user@gmail.com",
			agent:         "bot@gmail.com",
			autoSubmitted: "",
			xAutoReply:    "yes",
			expected:      true,
		},
		{
			name:          "Normal user message",
			sender:        "user@gmail.com",
			agent:         "bot@gmail.com",
			autoSubmitted: "no",
			xAutoReply:    "no",
			expected:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLoopOrAutoReply(tc.sender, tc.agent, tc.autoSubmitted, tc.xAutoReply)
			if got != tc.expected {
				t.Errorf("IsLoopOrAutoReply() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestNormalizeSubject(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello world", "Re: Hello world"},
		{"Re: Hello world", "Re: Hello world"},
		{"re: RE: Re: Hello world", "Re: Hello world"},
		{"", "Re: (No Subject)"},
	}

	for _, tc := range tests {
		got := NormalizeSubject(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeSubject(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
