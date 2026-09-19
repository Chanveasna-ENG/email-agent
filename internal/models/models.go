package models

import "time"

// Attachment represents a file attached to an email.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
	Path        string
	Size        int64
}

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
	Attachments []Attachment
}

// ConversationTurn represents a single turn in a multi-turn chat context.
type ConversationTurn struct {
	Role        string // "user" or "model"
	Content     string
	Attachments []Attachment
}
