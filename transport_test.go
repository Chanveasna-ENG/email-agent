package main

import (
	"strings"
	"testing"
)

func TestBuildMimeMessage(t *testing.T) {
	from := "agent@gmail.com"
	to := "Alice <alice@test.com>"
	subject := "Re: Hello"
	inReplyTo := "<msg1@mail>"
	references := []string{"<msg0@mail>"}
	htmlBody := "<p>Hello Alice</p>"
	textBody := "Hello Alice"

	raw, err := buildMimeMessage(from, to, subject, inReplyTo, references, htmlBody, textBody)
	if err != nil {
		t.Fatalf("buildMimeMessage returned unexpected error: %v", err)
	}

	msgStr := string(raw)

	expectedHeaders := []string{
		"From: agent@gmail.com",
		"To: Alice <alice@test.com>",
		"Subject: Re: Hello",
		"In-Reply-To: <msg1@mail>",
		"References: <msg0@mail> <msg1@mail>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"Content-Type: text/html; charset=\"utf-8\"",
		"Hello Alice",
		"<p>Hello Alice</p>",
	}

	for _, exp := range expectedHeaders {
		if !strings.Contains(msgStr, exp) {
			t.Errorf("MIME message missing expected substring %q. Generated message:\n%s", exp, msgStr)
		}
	}
}

func TestSliceContains(t *testing.T) {
	slice := []string{"alice@test.com", "bob@test.com"}

	if !sliceContains(slice, "ALICE@TEST.COM") {
		t.Error("expected sliceContains to match case-insensitively")
	}
	if !sliceContains(slice, " bob@test.com ") {
		t.Error("expected sliceContains to match trimmed spaces")
	}
	if sliceContains(slice, "charlie@test.com") {
		t.Error("expected sliceContains to return false for non-member")
	}
}
