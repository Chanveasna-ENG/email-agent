package main

import "time"

// EmailMessage represents a parsed incoming or stored email message.
type EmailMessage struct {
	UID         uint32
	MessageID   string
	InReplyTo   string
	References  []string
	Subject     string
	Sender      string
	SenderEmail string
	BodyText    string
	Date        time.Time
}

// ConversationTurn represents a single turn in a multi-turn chat context.
type ConversationTurn struct {
	Role    string // "user" or "model"
	Content string
}
