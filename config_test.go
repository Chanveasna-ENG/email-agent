package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "secret123")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("ALLOWED_SENDERS", " Alice@Domain.COM , bob@test.com ")
	os.Unsetenv("GEMINI_MODEL")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GMAIL_APP_PASSWORD")
		os.Unsetenv("GCP_PROJECT_ID")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig returned unexpected error: %v", err)
	}

	expectedSenders := []string{"alice@domain.com", "bob@test.com"}
	if !reflect.DeepEqual(cfg.AllowedSenders, expectedSenders) {
		t.Errorf("AllowedSenders = %v, want %v", cfg.AllowedSenders, expectedSenders)
	}

	if cfg.GmailAddress != "agent@gmail.com" {
		t.Errorf("GmailAddress = %q, want agent@gmail.com", cfg.GmailAddress)
	}
	if cfg.GeminiModel != "gemini-2.5-flash" {
		t.Errorf("GeminiModel default = %q, want gemini-2.5-flash", cfg.GeminiModel)
	}
	if cfg.DBPath != "emails.db" {
		t.Errorf("DBPath default = %q, want emails.db", cfg.DBPath)
	}
}

func TestParseConfigAPIKeyMode(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "secret123")
	os.Setenv("GEMINI_API_KEY", "AIzaSyTestKey123")
	os.Unsetenv("GCP_PROJECT_ID")
	os.Setenv("ALLOWED_SENDERS", "alice@domain.com")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GMAIL_APP_PASSWORD")
		os.Unsetenv("GEMINI_API_KEY")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig API key mode failed: %v", err)
	}

	if cfg.GeminiAPIKey != "AIzaSyTestKey123" {
		t.Errorf("GeminiAPIKey = %q, want AIzaSyTestKey123", cfg.GeminiAPIKey)
	}
}

func TestParseConfigMissingRequired(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Unsetenv("GMAIL_APP_PASSWORD")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("ALLOWED_SENDERS", "user@test.com")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GCP_PROJECT_ID")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	_, err := parseConfig()
	if err == nil {
		t.Fatal("parseConfig expected error when GMAIL_APP_PASSWORD missing, got nil")
	}
	if !strings.Contains(err.Error(), "GMAIL_APP_PASSWORD is required") {
		t.Errorf("expected error message to mention GMAIL_APP_PASSWORD, got: %v", err)
	}
}

func TestParseConfigMissingBothAuth(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "secret123")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GCP_PROJECT_ID")
	os.Setenv("ALLOWED_SENDERS", "user@test.com")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GMAIL_APP_PASSWORD")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	_, err := parseConfig()
	if err == nil {
		t.Fatal("parseConfig expected error when both GEMINI_API_KEY and GCP_PROJECT_ID missing, got nil")
	}
	if !strings.Contains(err.Error(), "either GEMINI_API_KEY or GCP_PROJECT_ID is required") {
		t.Errorf("expected error message to mention either auth key, got: %v", err)
	}
}
