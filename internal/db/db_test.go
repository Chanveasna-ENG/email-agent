package db

import (
	"path/filepath"
	"testing"
	"time"

	"email-agent/internal/models"
)

func TestEmailDBSaveAndSearch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_emails.db")

	db, err := NewEmailDB(dbPath)
	if err != nil {
		t.Fatalf("NewEmailDB failed: %v", err)
	}
	defer db.Close()

	msg1 := &models.EmailMessage{
		MessageID:   "<msg101@test>",
		Subject:     "Project Phoenix Budget Plan",
		Sender:      "Boss <boss@test.com>",
		SenderEmail: "boss@test.com",
		BodyText:    "The budget approved for Q3 is $50,000.",
		Date:        time.Now(),
		Attachments: []models.Attachment{
			{Filename: "budget.pdf", ContentType: "application/pdf", Size: 1024},
		},
	}

	msg2 := &models.EmailMessage{
		MessageID:   "<msg102@test>",
		Subject:     "Lunch Catering",
		Sender:      "Colleague <colleague@test.com>",
		SenderEmail: "colleague@test.com",
		BodyText:    "We are ordering pizza for lunch today.",
		Date:        time.Now(),
	}

	if err := db.SaveEmail(msg1); err != nil {
		t.Fatalf("SaveEmail msg1 failed: %v", err)
	}
	if err := db.SaveEmail(msg2); err != nil {
		t.Fatalf("SaveEmail msg2 failed: %v", err)
	}

	// 1. Fetch by Message-ID
	fetched, err := db.GetEmailByMessageID("<msg101@test>")
	if err != nil {
		t.Fatalf("GetEmailByMessageID failed: %v", err)
	}
	if fetched == nil || fetched.Subject != "Project Phoenix Budget Plan" {
		t.Fatalf("expected subject 'Project Phoenix Budget Plan', got %v", fetched)
	}

	// 2. Search for Phoenix
	results, err := db.SearchEmails("Phoenix", 5)
	if err != nil {
		t.Fatalf("SearchEmails failed: %v", err)
	}
	if len(results) != 1 || results[0].MessageID != "<msg101@test>" {
		t.Fatalf("expected 1 result with msg101, got %d results", len(results))
	}

	// 3. Search for budget
	budgetResults, err := db.SearchEmails("budget", 5)
	if err != nil {
		t.Fatalf("SearchEmails failed: %v", err)
	}
	if len(budgetResults) != 1 {
		t.Fatalf("expected 1 result for budget, got %d", len(budgetResults))
	}

	// 4. Search with no matches
	noneResults, err := db.SearchEmails("astronaut", 5)
	if err != nil {
		t.Fatalf("SearchEmails failed: %v", err)
	}
	if len(noneResults) != 0 {
		t.Fatalf("expected 0 results for astronaut, got %d", len(noneResults))
	}
}
