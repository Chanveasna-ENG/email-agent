package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set up required environment variables
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "secret123")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("ALLOWED_SENDERS", " Alice@Domain.COM , bob@test.com ")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GMAIL_APP_PASSWORD")
		os.Unsetenv("GCP_PROJECT_ID")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned unexpected error: %v", err)
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
}

func TestLoadConfigMissingRequired(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Unsetenv("GMAIL_APP_PASSWORD")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("ALLOWED_SENDERS", "user@test.com")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GCP_PROJECT_ID")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig expected error when GMAIL_APP_PASSWORD missing, got nil")
	}
	if !strings.Contains(err.Error(), "GMAIL_APP_PASSWORD is required") {
		t.Errorf("expected error message to mention GMAIL_APP_PASSWORD, got: %v", err)
	}
}
