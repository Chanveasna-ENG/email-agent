package parser

import (
	"reflect"
	"testing"

	"email-agent/internal/models"
)

func TestAssembleThread(t *testing.T) {
	agentEmail := "agent@domain.com"

	currentMsg := &models.EmailMessage{
		MessageID:   "<current@mail>",
		SenderEmail: "user@domain.com",
		BodyText:    "What is the capital of France?\n\nOn Fri wrote:\n> past quote",
	}

	t.Run("No references", func(t *testing.T) {
		turns, err := AssembleThread(nil, nil, currentMsg, agentEmail)
		if err != nil {
			t.Fatalf("AssembleThread failed: %v", err)
		}
		expected := []models.ConversationTurn{
			{Role: "user", Content: "What is the capital of France?"},
		}
		if !reflect.DeepEqual(turns, expected) {
			t.Errorf("turns = %+v, want %+v", turns, expected)
		}
	})

	t.Run("With prior messages", func(t *testing.T) {
		db := map[string]*models.EmailMessage{
			"<msg1@mail>": {
				MessageID:   "<msg1@mail>",
				SenderEmail: "user@domain.com",
				BodyText:    "Hello agent",
			},
			"<msg2@mail>": {
				MessageID:   "<msg2@mail>",
				SenderEmail: "agent@domain.com",
				BodyText:    "Hello! How can I help you today?",
			},
		}

		mockFetcher := func(msgID string) (*models.EmailMessage, error) {
			return db[msgID], nil
		}

		refs := []string{"<msg1@mail>", "<msg2@mail>"}
		turns, err := AssembleThread(mockFetcher, refs, currentMsg, agentEmail)
		if err != nil {
			t.Fatalf("AssembleThread failed: %v", err)
		}

		expected := []models.ConversationTurn{
			{Role: "user", Content: "Hello agent"},
			{Role: "model", Content: "Hello! How can I help you today?"},
			{Role: "user", Content: "What is the capital of France?"},
		}
		if !reflect.DeepEqual(turns, expected) {
			t.Errorf("turns = %+v, want %+v", turns, expected)
		}
	})

	t.Run("Missing prior message skipped gracefully", func(t *testing.T) {
		mockFetcher := func(msgID string) (*models.EmailMessage, error) {
			return nil, nil // Not found
		}

		refs := []string{"<missing@mail>"}
		turns, err := AssembleThread(mockFetcher, refs, currentMsg, agentEmail)
		if err != nil {
			t.Fatalf("AssembleThread failed: %v", err)
		}

		expected := []models.ConversationTurn{
			{Role: "user", Content: "What is the capital of France?"},
		}
		if !reflect.DeepEqual(turns, expected) {
			t.Errorf("turns = %+v, want %+v", turns, expected)
		}
	})
}
