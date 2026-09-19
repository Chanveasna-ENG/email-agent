package parser

import (
	"strings"

	"email-agent/internal/models"
)

// MessageFetcher is an abstraction for querying prior emails by Message-ID.
type MessageFetcher func(messageID string) (*models.EmailMessage, error)

// AssembleThread reconstructs chronological conversation turns from references and the current message.
func AssembleThread(fetcher MessageFetcher, references []string, currentMsg *models.EmailMessage, agentEmail string) ([]models.ConversationTurn, error) {
	var rawTurns []models.ConversationTurn
	cleanAgentEmail := strings.ToLower(strings.TrimSpace(agentEmail))

	// Fetch referenced prior messages in chronological order
	for _, refID := range references {
		cleanRefID := strings.TrimSpace(refID)
		if cleanRefID == "" {
			continue
		}

		if fetcher != nil {
			msg, err := fetcher(cleanRefID)
			if err != nil || msg == nil {
				continue
			}

			cleanSender := strings.ToLower(strings.TrimSpace(msg.SenderEmail))
			if cleanSender == cleanAgentEmail {
				rawTurns = append(rawTurns, models.ConversationTurn{
					Role:        "model",
					Content:     strings.TrimSpace(msg.BodyText),
					Attachments: msg.Attachments,
				})
			} else {
				userText := StripQuotedReply(msg.BodyText)
				if userText != "" || len(msg.Attachments) > 0 {
					rawTurns = append(rawTurns, models.ConversationTurn{
						Role:        "user",
						Content:     userText,
						Attachments: msg.Attachments,
					})
				}
			}
		}
	}

	// Append current message as the latest user turn
	currentUserText := StripQuotedReply(currentMsg.BodyText)
	if currentUserText == "" {
		currentUserText = strings.TrimSpace(currentMsg.BodyText)
	}
	rawTurns = append(rawTurns, models.ConversationTurn{
		Role:        "user",
		Content:     currentUserText,
		Attachments: currentMsg.Attachments,
	})

	// Consolidate consecutive turns with the same role to conform to Gemini multi-turn API
	var turns []models.ConversationTurn
	for _, t := range rawTurns {
		if t.Content == "" && len(t.Attachments) == 0 {
			continue
		}
		if len(turns) > 0 && turns[len(turns)-1].Role == t.Role {
			if t.Content != "" {
				if turns[len(turns)-1].Content != "" {
					turns[len(turns)-1].Content += "\n\n" + t.Content
				} else {
					turns[len(turns)-1].Content = t.Content
				}
			}
			turns[len(turns)-1].Attachments = append(turns[len(turns)-1].Attachments, t.Attachments...)
		} else {
			turns = append(turns, t)
		}
	}

	return turns, nil
}
