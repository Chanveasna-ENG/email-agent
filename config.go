package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration for the email agent service.
type Config struct {
	GmailAddress     string
	GmailAppPassword string
	AllowedSenders   []string
	GCPProjectID     string
	GCPLocation      string
	GeminiModel      string
	CredentialsPath  string
	SystemPromptPath string
	IdleTimeout      time.Duration
	PollInterval     time.Duration
}

// LoadConfig reads configuration from environment variables and an optional .env file.
func LoadConfig() (*Config, error) {
	_ = godotenv.Load() // optional .env file

	gmailAddress := strings.TrimSpace(os.Getenv("GMAIL_ADDRESS"))
	gmailPassword := strings.TrimSpace(os.Getenv("GMAIL_APP_PASSWORD"))
	gcpProjectID := strings.TrimSpace(os.Getenv("GCP_PROJECT_ID"))
	gcpLocation := strings.TrimSpace(os.Getenv("GCP_LOCATION"))
	geminiModel := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	credsPath := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	promptPath := strings.TrimSpace(os.Getenv("SYSTEM_PROMPT_FILE"))
	rawSenders := strings.TrimSpace(os.Getenv("ALLOWED_SENDERS"))

	if gmailAddress == "" {
		return nil, fmt.Errorf("GMAIL_ADDRESS is required")
	}
	if gmailPassword == "" {
		return nil, fmt.Errorf("GMAIL_APP_PASSWORD is required")
	}
	if gcpProjectID == "" {
		return nil, fmt.Errorf("GCP_PROJECT_ID is required")
	}
	if rawSenders == "" {
		return nil, fmt.Errorf("ALLOWED_SENDERS is required (comma-separated email list)")
	}

	if gcpLocation == "" {
		gcpLocation = "us-central1"
	}
	if geminiModel == "" {
		geminiModel = "gemini-2.5-flash"
	}
	if promptPath == "" {
		promptPath = "system_prompt.txt"
	}

	idleTimeout := 29 * time.Minute
	if rawIdle := os.Getenv("IDLE_TIMEOUT_MINUTES"); rawIdle != "" {
		if mins, err := strconv.Atoi(rawIdle); err == nil && mins > 0 {
			idleTimeout = time.Duration(mins) * time.Minute
		}
	}

	pollInterval := 15 * time.Second
	if rawPoll := os.Getenv("POLL_INTERVAL_SECONDS"); rawPoll != "" {
		if secs, err := strconv.Atoi(rawPoll); err == nil && secs > 0 {
			pollInterval = time.Duration(secs) * time.Second
		}
	}

	var allowedSenders []string
	for _, s := range strings.Split(rawSenders, ",") {
		clean := strings.ToLower(strings.TrimSpace(s))
		if clean != "" {
			allowedSenders = append(allowedSenders, clean)
		}
	}

	return &Config{
		GmailAddress:     gmailAddress,
		GmailAppPassword: gmailPassword,
		AllowedSenders:   allowedSenders,
		GCPProjectID:     gcpProjectID,
		GCPLocation:      gcpLocation,
		GeminiModel:      geminiModel,
		CredentialsPath:  credsPath,
		SystemPromptPath: promptPath,
		IdleTimeout:      idleTimeout,
		PollInterval:     pollInterval,
	}, nil
}
