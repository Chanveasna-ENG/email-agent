package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"email-agent/internal/db"
	"email-agent/internal/models"
)

func TestRunSearch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "search_test.db")

	emailDB, err := db.NewEmailDB(dbPath)
	if err != nil {
		t.Fatalf("failed creating test db: %v", err)
	}

	testMsg := &models.EmailMessage{
		MessageID:   "<search1@test>",
		Subject:     "Quarterly Server Maintenance",
		Sender:      "Admin <admin@test.com>",
		SenderEmail: "admin@test.com",
		BodyText:    "Server reboot planned for Saturday 10 PM UTC.",
		Date:        time.Now(),
	}
	if err := emailDB.SaveEmail(testMsg); err != nil {
		t.Fatalf("failed saving test email: %v", err)
	}
	emailDB.Close()

	// 1. Successful keyword match
	out, err := runSearch(dbPath, "Maintenance")
	if err != nil {
		t.Fatalf("runSearch failed: %v", err)
	}
	if !strings.Contains(out, "Quarterly Server Maintenance") || !strings.Contains(out, "admin@test.com") {
		t.Errorf("expected search output to contain email details, got: %s", out)
	}

	// 2. Zero matches
	outNone, err := runSearch(dbPath, "NonExistentKeywordXYZ")
	if err != nil {
		t.Fatalf("runSearch with zero matches failed: %v", err)
	}
	if !strings.Contains(outNone, "No matching emails found") {
		t.Errorf("expected no matching emails message, got: %s", outNone)
	}

	// 3. Empty query returns error
	_, errEmpty := runSearch(dbPath, "   ")
	if errEmpty == nil {
		t.Errorf("expected error on empty query, got nil")
	}
}
