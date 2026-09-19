package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"email-agent/internal/models"

	_ "modernc.org/sqlite"
)

// EmailDB manages local caching and full-text search across whitelisted emails.
type EmailDB struct {
	db *sql.DB
}

// NewEmailDB initializes or opens the SQLite database and executes migrations.
func NewEmailDB(dbPath string) (*EmailDB, error) {
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT UNIQUE,
		in_reply_to TEXT,
		subject TEXT,
		sender TEXT,
		sender_email TEXT,
		body_text TEXT,
		date DATETIME
	);

	CREATE TABLE IF NOT EXISTS attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT,
		filename TEXT,
		content_type TEXT,
		file_path TEXT,
		size INTEGER
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS emails_fts USING fts5(
		message_id UNINDEXED,
		subject,
		body_text
	);

	CREATE TABLE IF NOT EXISTS thread_conversations (
		thread_id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite schema: %w", err)
	}

	return &EmailDB{db: db}, nil
}

// SaveEmail indexes an email and its attachment records into SQLite.
func (e *EmailDB) SaveEmail(msg *models.EmailMessage) error {
	if msg == nil || strings.TrimSpace(msg.MessageID) == "" {
		return nil
	}

	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
	INSERT INTO emails (message_id, in_reply_to, subject, sender, sender_email, body_text, date)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(message_id) DO UPDATE SET
		subject = excluded.subject,
		body_text = excluded.body_text,
		date = excluded.date;
	`

	dateVal := msg.Date
	if dateVal.IsZero() {
		dateVal = time.Now()
	}

	if _, err := tx.Exec(query, msg.MessageID, msg.InReplyTo, msg.Subject, msg.Sender, msg.SenderEmail, msg.BodyText, dateVal); err != nil {
		return fmt.Errorf("insert email: %w", err)
	}

	// Update FTS5 index
	_, _ = tx.Exec("DELETE FROM emails_fts WHERE message_id = ?", msg.MessageID)
	_, _ = tx.Exec("INSERT INTO emails_fts (message_id, subject, body_text) VALUES (?, ?, ?)", msg.MessageID, msg.Subject, msg.BodyText)

	// Save attachment metadata
	for _, att := range msg.Attachments {
		attQuery := `
		INSERT INTO attachments (message_id, filename, content_type, file_path, size)
		VALUES (?, ?, ?, ?, ?);
		`
		size := att.Size
		if size == 0 && len(att.Data) > 0 {
			size = int64(len(att.Data))
		}
		_, _ = tx.Exec(attQuery, msg.MessageID, att.Filename, att.ContentType, att.Path, size)
	}

	return tx.Commit()
}

// GetEmailByMessageID retrieves a cached message by its Message-ID header.
func (e *EmailDB) GetEmailByMessageID(messageID string) (*models.EmailMessage, error) {
	cleanID := strings.TrimSpace(messageID)
	if cleanID == "" {
		return nil, nil
	}

	row := e.db.QueryRow(`
		SELECT message_id, in_reply_to, subject, sender, sender_email, body_text, date
		FROM emails
		WHERE message_id = ?;
	`, cleanID)

	var msg models.EmailMessage
	var inReplyTo sql.NullString
	if err := row.Scan(&msg.MessageID, &inReplyTo, &msg.Subject, &msg.Sender, &msg.SenderEmail, &msg.BodyText, &msg.Date); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query email by message_id: %w", err)
	}
	if inReplyTo.Valid {
		msg.InReplyTo = inReplyTo.String
	}

	return &msg, nil
}

// SearchEmails searches cached whitelisted emails using FTS5 (with LIKE fallback).
func (e *EmailDB) SearchEmails(query string, limit int) ([]*models.EmailMessage, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	ftsQuery := `
		SELECT e.message_id, e.in_reply_to, e.subject, e.sender, e.sender_email, e.body_text, e.date
		FROM emails e
		JOIN emails_fts fts ON e.message_id = fts.message_id
		WHERE emails_fts MATCH ?
		ORDER BY rank
		LIMIT ?;
	`

	sanitized := strings.ReplaceAll(cleanQuery, `"`, `""`)
	rows, err := e.db.Query(ftsQuery, `"`+sanitized+`"`, limit)
	if err != nil {
		likePattern := "%" + cleanQuery + "%"
		fallbackQuery := `
			SELECT message_id, in_reply_to, subject, sender, sender_email, body_text, date
			FROM emails
			WHERE subject LIKE ? OR body_text LIKE ?
			ORDER BY date DESC
			LIMIT ?;
		`
		rows, err = e.db.Query(fallbackQuery, likePattern, likePattern, limit)
		if err != nil {
			return nil, fmt.Errorf("search emails fallback: %w", err)
		}
	}
	defer rows.Close()

	var results []*models.EmailMessage
	for rows.Next() {
		var msg models.EmailMessage
		var inReplyTo sql.NullString
		if err := rows.Scan(&msg.MessageID, &inReplyTo, &msg.Subject, &msg.Sender, &msg.SenderEmail, &msg.BodyText, &msg.Date); err != nil {
			continue
		}
		if inReplyTo.Valid {
			msg.InReplyTo = inReplyTo.String
		}
		results = append(results, &msg)
	}

	return results, nil
}

// GetRecentEmails retrieves the latest emails across all cached messages in chronological order.
func (e *EmailDB) GetRecentEmails(limit int) ([]*models.EmailMessage, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT message_id, in_reply_to, subject, sender, sender_email, body_text, date
		FROM emails
		ORDER BY date DESC
		LIMIT ?;
	`

	rows, err := e.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent emails: %w", err)
	}
	defer rows.Close()

	var results []*models.EmailMessage
	for rows.Next() {
		var msg models.EmailMessage
		var inReplyTo sql.NullString
		if err := rows.Scan(&msg.MessageID, &inReplyTo, &msg.Subject, &msg.Sender, &msg.SenderEmail, &msg.BodyText, &msg.Date); err != nil {
			continue
		}
		if inReplyTo.Valid {
			msg.InReplyTo = inReplyTo.String
		}
		results = append(results, &msg)
	}

	// Reverse to ascending order (oldest to newest)
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	return results, nil
}

// GetConversationID returns the mapped Antigravity conversation ID for a given thread root.
func (e *EmailDB) GetConversationID(threadID string) (string, error) {
	cleanID := strings.TrimSpace(threadID)
	if cleanID == "" {
		return "", nil
	}

	row := e.db.QueryRow("SELECT conversation_id FROM thread_conversations WHERE thread_id = ?", cleanID)
	var convID string
	if err := row.Scan(&convID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("query conversation_id: %w", err)
	}
	return convID, nil
}

// SaveConversationID stores or updates the Antigravity conversation ID for an email thread.
func (e *EmailDB) SaveConversationID(threadID, conversationID string) error {
	cleanThread := strings.TrimSpace(threadID)
	cleanConv := strings.TrimSpace(conversationID)
	if cleanThread == "" || cleanConv == "" {
		return nil
	}

	query := `
	INSERT INTO thread_conversations (thread_id, conversation_id, updated_at)
	VALUES (?, ?, ?)
	ON CONFLICT(thread_id) DO UPDATE SET
		conversation_id = excluded.conversation_id,
		updated_at = excluded.updated_at;
	`
	_, err := e.db.Exec(query, cleanThread, cleanConv, time.Now())
	if err != nil {
		return fmt.Errorf("save conversation_id: %w", err)
	}
	return nil
}

// Close closes the underlying SQLite database connection.
func (e *EmailDB) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}
