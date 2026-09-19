package main

import (
	"strings"
)

// MessageFetcher is an abstraction for querying prior emails by Message-ID.
type MessageFetcher func(messageID string) (*EmailMessage, error)

// AssembleThread reconstructs chronological conversation turns from references and the current message.
func AssembleThread(fetcher MessageFetcher, references []string, currentMsg *EmailMessage, agentEmail string) ([]ConversationTurn, error) {
	var rawTurns []ConversationTurn
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
				// Prior message may be deleted or outside folder; skip gracefully
				continue
			}

			cleanSender := strings.ToLower(strings.TrimSpace(msg.SenderEmail))
			if cleanSender == cleanAgentEmail {
				rawTurns = append(rawTurns, ConversationTurn{
					Role:    "model",
					Content: strings.TrimSpace(msg.BodyText),
				})
			} else {
				userText := StripQuotedReply(msg.BodyText)
				if userText != "" {
					rawTurns = append(rawTurns, ConversationTurn{
						Role:    "user",
						Content: userText,
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
	rawTurns = append(rawTurns, ConversationTurn{
		Role:    "user",
		Content: currentUserText,
	})

	// Consolidate consecutive turns with the same role to conform to Gemini multi-turn API
	var turns []ConversationTurn
	for _, t := range rawTurns {
		if t.Content == "" {
			continue
		}
		if len(turns) > 0 && turns[len(turns)-1].Role == t.Role {
			turns[len(turns)-1].Content += "\n\n" + t.Content
		} else {
			turns = append(turns, t)
		}
	}

	return turns, nil
}
