package config

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

	cfg, err := ParseConfig()
	if err != nil {
		t.Fatalf("ParseConfig returned unexpected error: %v", err)
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
	if cfg.DBPath != "data/emails.db" {
		t.Errorf("DBPath default = %q, want data/emails.db", cfg.DBPath)
	}
	if cfg.AttachmentsDir != "data/attachments" {
		t.Errorf("AttachmentsDir default = %q, want data/attachments", cfg.AttachmentsDir)
	}
	if cfg.AIBackend != "antigravity" {
		t.Errorf("AIBackend default = %q, want antigravity", cfg.AIBackend)
	}
	if cfg.AntigravityBin != "agy" {
		t.Errorf("AntigravityBin default = %q, want agy", cfg.AntigravityBin)
	}
	if cfg.SkillsDir != "skills" {
		t.Errorf("SkillsDir default = %q, want skills", cfg.SkillsDir)
	}
}

func TestParseConfigAPIKeyMode(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "secret123")
	os.Setenv("AI_BACKEND", "gemini")
	os.Setenv("GEMINI_API_KEY", "AIzaSyTestKey123")
	os.Unsetenv("GCP_PROJECT_ID")
	os.Setenv("ALLOWED_SENDERS", "alice@domain.com")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GMAIL_APP_PASSWORD")
		os.Unsetenv("AI_BACKEND")
		os.Unsetenv("GEMINI_API_KEY")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	cfg, err := ParseConfig()
	if err != nil {
		t.Fatalf("ParseConfig API key mode failed: %v", err)
	}

	if cfg.GeminiAPIKey != "AIzaSyTestKey123" {
		t.Errorf("GeminiAPIKey = %q, want AIzaSyTestKey123", cfg.GeminiAPIKey)
	}
	if cfg.AIBackend != "gemini" {
		t.Errorf("AIBackend = %q, want gemini", cfg.AIBackend)
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

	_, err := ParseConfig()
	if err == nil {
		t.Fatal("ParseConfig expected error when GMAIL_APP_PASSWORD missing, got nil")
	}
	if !strings.Contains(err.Error(), "GMAIL_APP_PASSWORD is required") {
		t.Errorf("expected error message to mention GMAIL_APP_PASSWORD, got: %v", err)
	}
}

func TestParseConfigMissingBothAuthWhenGemini(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "secret123")
	os.Setenv("AI_BACKEND", "gemini")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GCP_PROJECT_ID")
	os.Setenv("ALLOWED_SENDERS", "user@test.com")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GMAIL_APP_PASSWORD")
		os.Unsetenv("AI_BACKEND")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	_, err := ParseConfig()
	if err == nil {
		t.Fatal("ParseConfig expected error when AI_BACKEND=gemini and both keys missing, got nil")
	}
	if !strings.Contains(err.Error(), "either GEMINI_API_KEY or GCP_PROJECT_ID is required") {
		t.Errorf("expected error message to mention either auth key, got: %v", err)
	}
}

func TestParseConfigAntigravityModeWithoutGeminiKeys(t *testing.T) {
	os.Setenv("GMAIL_ADDRESS", "agent@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "secret123")
	os.Setenv("AI_BACKEND", "antigravity")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GCP_PROJECT_ID")
	os.Setenv("ALLOWED_SENDERS", "user@test.com")
	defer func() {
		os.Unsetenv("GMAIL_ADDRESS")
		os.Unsetenv("GMAIL_APP_PASSWORD")
		os.Unsetenv("AI_BACKEND")
		os.Unsetenv("ALLOWED_SENDERS")
	}()

	cfg, err := ParseConfig()
	if err != nil {
		t.Fatalf("ParseConfig in antigravity mode should not require Gemini keys, got error: %v", err)
	}
	if cfg.AIBackend != "antigravity" {
		t.Errorf("AIBackend = %q, want antigravity", cfg.AIBackend)
	}
}

func TestIsAllowedSender(t *testing.T) {
	cfg := &Config{
		AllowedSenders: []string{"alice@test.com", "bob@test.com"},
	}

	if !cfg.IsAllowedSender("alice@test.com") {
		t.Error("expected exact match to return true")
	}
	if !cfg.IsAllowedSender(" ALICE@TEST.COM ") {
		t.Error("expected trimmed uppercase match to return true")
	}
	if cfg.IsAllowedSender("mallory@evil.com") {
		t.Error("expected unauthorized sender to return false")
	}
}
