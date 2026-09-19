package main

import (
	"strings"
	"testing"
)

func TestFormatReply(t *testing.T) {
	tests := []struct {
		name         string
		markdown     string
		expectInHTML []string
		expectText   string
	}{
		{
			name:         "Empty text",
			markdown:     "",
			expectInHTML: nil,
			expectText:   "",
		},
		{
			name:     "Bold and bullet list",
			markdown: "**Important Note**:\n- Task 1\n- Task 2",
			expectInHTML: []string{
				"<strong>Important Note</strong>",
				"<ul>",
				"<li>Task 1</li>",
				"<li>Task 2</li>",
			},
			expectText: "**Important Note**:\n- Task 1\n- Task 2",
		},
		{
			name:     "Code block",
			markdown: "Check this command:\n```bash\ngo test ./...\n```",
			expectInHTML: []string{
				"<pre><code",
				"go test ./...",
			},
			expectText: "Check this command:\n```bash\ngo test ./...\n```",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			htmlOut, textOut, err := FormatReply(tc.markdown)
			if err != nil {
				t.Fatalf("FormatReply returned unexpected error: %v", err)
			}
			if textOut != tc.expectText {
				t.Errorf("textOut = %q, want %q", textOut, tc.expectText)
			}
			for _, expectedSub := range tc.expectInHTML {
				if !strings.Contains(htmlOut, expectedSub) {
					t.Errorf("htmlOut missing expected substring %q. Got:\n%s", expectedSub, htmlOut)
				}
			}
		})
	}
}
