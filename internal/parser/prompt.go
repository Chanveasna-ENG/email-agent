package parser

import (
	"fmt"
	"strings"
	"time"

	"email-agent/internal/models"
)

// BuildSearchPrompt constructs the pass-1 prompt to determine whether archive search is needed.
func BuildSearchPrompt(subject, body string) string {
	return fmt.Sprintf(`You are an email assistant deciding if an inbound email needs information from past email archives.

Inbound Email:
Subject: %s
Body:
%s

If this email refers to or asks about previous conversations, past agreements, or context not found above, reply with 1 to 4 space-separated search keywords.
If no past email archive search is needed, reply with: NONE.
Output only the search keywords or NONE, with no explanations or punctuation.`, strings.TrimSpace(subject), strings.TrimSpace(body))
}

// ExtractSearchQuery parses and cleans the model's search keyword recommendation.
func ExtractSearchQuery(rawOutput string) string {
	trimmed := strings.TrimSpace(rawOutput)
	if trimmed == "" {
		return ""
	}

	// Read first line
	lines := strings.Split(trimmed, "\n")
	firstLine := strings.TrimSpace(lines[0])

	// Strip code blocks or quotes
	firstLine = strings.Trim(firstLine, "`\"' \t\r")

	if strings.EqualFold(firstLine, "NONE") || strings.EqualFold(firstLine, "NONE.") {
		return ""
	}

	// Remove common leading phrases if model included any
	lower := strings.ToLower(firstLine)
	if strings.HasPrefix(lower, "keywords:") {
		firstLine = strings.TrimSpace(firstLine[len("keywords:"):])
	} else if strings.HasPrefix(lower, "search:") {
		firstLine = strings.TrimSpace(firstLine[len("search:"):])
	}

	firstLine = strings.Trim(firstLine, "`\"' \t\r.,;:")

	if strings.EqualFold(firstLine, "NONE") {
		return ""
	}

	return firstLine
}

// FormatArchiveContext serializes relevant past emails into a prompt section.
func FormatArchiveContext(matches []*models.EmailMessage, currentMsgID string) string {
	var valid []*models.EmailMessage
	for _, m := range matches {
		if m == nil {
			continue
		}
		if currentMsgID != "" && strings.TrimSpace(m.MessageID) == strings.TrimSpace(currentMsgID) {
			continue
		}
		valid = append(valid, m)
	}

	if len(valid) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Relevant Past Archive Emails]:\n")
	for _, m := range valid {
		bodySnippet := strings.TrimSpace(m.BodyText)
		if len(bodySnippet) > 300 {
			bodySnippet = bodySnippet[:300] + "..."
		}
		sb.WriteString(fmt.Sprintf("- [%s] From: %s | Subject: %q | %s\n",
			m.Date.Format("2006-01-02 15:04"), m.Sender, m.Subject, bodySnippet))
	}

	return strings.TrimSpace(sb.String())
}

// BuildReplyPrompt generates the complete pass-2 prompt for drafting an email reply.
func BuildReplyPrompt(systemPrompt string, msg *models.EmailMessage, turns []models.ConversationTurn, archiveContext string) string {
	var sb strings.Builder

	if cleanPrompt := strings.TrimSpace(systemPrompt); cleanPrompt != "" {
		sb.WriteString("[System Instructions]:\n")
		sb.WriteString(cleanPrompt)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("Inbound email from: %s\nSubject: %s\nDate: %s\n",
		msg.Sender, msg.Subject, msg.Date.Format(time.RFC1123Z)))

	if len(msg.Attachments) > 0 {
		sb.WriteString("Attachments:\n")
		for _, att := range msg.Attachments {
			sb.WriteString(fmt.Sprintf("- %s (%s, path: %s)\n", att.Filename, att.ContentType, att.Path))
		}
	}

	if len(turns) > 1 {
		sb.WriteString("\n[Conversation Thread History]:\n")
		for i, t := range turns {
			roleLabel := "User"
			if t.Role == "model" {
				roleLabel = "Assistant"
			}
			sb.WriteString(fmt.Sprintf("Turn %d (%s):\n%s\n\n", i+1, roleLabel, strings.TrimSpace(t.Content)))
		}
	} else {
		sb.WriteString(fmt.Sprintf("\nEmail Content:\n%s\n", msg.BodyText))
	}

	if cleanArchive := strings.TrimSpace(archiveContext); cleanArchive != "" {
		sb.WriteString("\n")
		sb.WriteString(cleanArchive)
		sb.WriteString("\n")
	}

	sb.WriteString("\nPlease write a concise, professional reply to the sender.")

	return sb.String()
}
