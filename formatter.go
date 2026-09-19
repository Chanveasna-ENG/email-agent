package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var mdRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
	),
)

// FormatReply converts a Markdown text into minimal HTML and a clean plain-text fallback.
func FormatReply(markdownText string) (string, string, error) {
	trimmed := strings.TrimSpace(markdownText)
	if trimmed == "" {
		return "", "", nil
	}

	var buf bytes.Buffer
	if err := mdRenderer.Convert([]byte(trimmed), &buf); err != nil {
		return "", "", fmt.Errorf("render markdown to html: %w", err)
	}

	rawHTML := buf.String()

	// Minimal inline CSS styling tailored for email client simplicity
	htmlEmail := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
    font-size: 14px;
    line-height: 1.5;
    color: #1f2328;
    margin: 0;
    padding: 0;
  }
  p { margin: 0 0 12px 0; }
  ul, ol { margin: 0 0 12px 0; padding-left: 20px; }
  li { margin-bottom: 4px; }
  code {
    background-color: #f6f8fa;
    padding: 2px 5px;
    border-radius: 4px;
    font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace;
    font-size: 13px;
  }
  pre {
    background-color: #f6f8fa;
    padding: 10px 12px;
    border-radius: 6px;
    overflow-x: auto;
    font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace;
    font-size: 13px;
    line-height: 1.45;
  }
  pre code {
    background-color: transparent;
    padding: 0;
  }
  blockquote {
    border-left: 3px solid #d0d7de;
    margin: 0 0 12px 0;
    padding: 0 12px;
    color: #57606a;
  }
  a { color: #0969da; text-decoration: underline; }
</style>
</head>
<body>
%s
</body>
</html>`, rawHTML)

	return htmlEmail, trimmed, nil
}
