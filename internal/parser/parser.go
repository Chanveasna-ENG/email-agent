package parser

import (
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strings"

	"email-agent/internal/models"

	gomail "github.com/emersion/go-message/mail"
)

var (
	referencesRegex = regexp.MustCompile(`<[^>]+>`)
	reSubjectRegex  = regexp.MustCompile(`(?i)^(re:\s*)+`)
	quotePatterns   = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^On\s+.+wrote:\s*$`),
		regexp.MustCompile(`(?m)^-{3,}\s*Original Message\s*-{3,}`),
		regexp.MustCompile(`(?m)^_{10,}`),
		regexp.MustCompile(`(?m)^From:\s+.+`),
	}
)

const maxAttachmentBytes = 20 * 1024 * 1024 // 20MB ceiling

// ExtractEmailAddress parses a header like "Alice <alice@example.com>" and returns "alice@example.com".
func ExtractEmailAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	addr, err := mail.ParseAddress(raw)
	if err == nil && addr.Address != "" {
		return strings.ToLower(strings.TrimSpace(addr.Address))
	}
	// Fallback for bare email or angle-bracketed formats
	matches := referencesRegex.FindString(raw)
	if matches != "" {
		return strings.ToLower(strings.Trim(matches, "<>"))
	}
	return strings.ToLower(raw)
}

// ParseReferences extracts message IDs enclosed in angle brackets.
func ParseReferences(raw string) []string {
	if raw == "" {
		return nil
	}
	matches := referencesRegex.FindAllString(raw, -1)
	if len(matches) == 0 {
		return nil
	}
	var refs []string
	for _, m := range matches {
		trimmed := strings.TrimSpace(m)
		if trimmed != "" {
			refs = append(refs, trimmed)
		}
	}
	return refs
}

// StripQuotedReply removes email reply trails such as "On ... wrote:" and quoted lines.
func StripQuotedReply(body string) string {
	clean := strings.TrimSpace(body)
	if clean == "" {
		return ""
	}

	cutoff := len(clean)
	for _, p := range quotePatterns {
		loc := p.FindStringIndex(clean)
		if loc != nil && loc[0] < cutoff {
			cutoff = loc[0]
		}
	}

	result := strings.TrimSpace(clean[:cutoff])

	// Also strip leading/trailing quote marks if the entire block is quoted
	lines := strings.Split(result, "\n")
	var unquotedLines []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, ">") {
			continue
		}
		unquotedLines = append(unquotedLines, line)
	}

	return strings.TrimSpace(strings.Join(unquotedLines, "\n"))
}

// IsLoopOrAutoReply detects if an incoming message is from the agent itself or an automated response.
func IsLoopOrAutoReply(senderEmail, agentEmail string, autoSubmitted, xAutoReply string) bool {
	cleanSender := strings.ToLower(strings.TrimSpace(senderEmail))
	cleanAgent := strings.ToLower(strings.TrimSpace(agentEmail))

	if cleanSender != "" && cleanSender == cleanAgent {
		return true
	}

	autoSub := strings.ToLower(strings.TrimSpace(autoSubmitted))
	if autoSub != "" && autoSub != "no" {
		return true
	}

	if strings.ToLower(strings.TrimSpace(xAutoReply)) == "yes" {
		return true
	}

	return false
}

// NormalizeSubject ensures the subject has a single clean "Re: " prefix.
func NormalizeSubject(subj string) string {
	clean := strings.TrimSpace(subj)
	if clean == "" {
		return "Re: (No Subject)"
	}
	clean = reSubjectRegex.ReplaceAllString(clean, "")
	clean = strings.TrimSpace(clean)
	return fmt.Sprintf("Re: %s", clean)
}

// ExtractEmailParts iterates over email MIME parts, extracting plain text body and attachments.
func ExtractEmailParts(mr *gomail.Reader) (string, []models.Attachment, error) {
	var bodyText string
	var attachments []models.Attachment

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}

		switch h := part.Header.(type) {
		case *gomail.InlineHeader:
			contentType, _, _ := h.ContentType()
			disp := h.Get("Content-Disposition")
			filename := ""
			if strings.Contains(disp, "filename=") {
				filename = extractFilenameFromHeader(disp)
			}

			if filename != "" {
				data, err := io.ReadAll(io.LimitReader(part.Body, maxAttachmentBytes))
				if err == nil && len(data) > 0 {
					attachments = append(attachments, models.Attachment{
						Filename:    filename,
						ContentType: contentType,
						Data:        data,
						Size:        int64(len(data)),
					})
				}
			} else if strings.HasPrefix(contentType, "text/plain") && bodyText == "" {
				b, _ := io.ReadAll(part.Body)
				bodyText = string(b)
			}
		case *gomail.AttachmentHeader:
			filename, _ := h.Filename()
			if filename == "" {
				filename = "attachment"
			}
			contentType, _, _ := h.ContentType()
			data, err := io.ReadAll(io.LimitReader(part.Body, maxAttachmentBytes))
			if err == nil && len(data) > 0 {
				attachments = append(attachments, models.Attachment{
					Filename:    filename,
					ContentType: contentType,
					Data:        data,
					Size:        int64(len(data)),
				})
			}
		}
	}

	return bodyText, attachments, nil
}

func extractFilenameFromHeader(disp string) string {
	idx := strings.Index(disp, "filename=")
	if idx == -1 {
		return ""
	}
	fn := strings.TrimSpace(disp[idx+9:])
	fn = strings.Trim(fn, `";'`)
	return fn
}
