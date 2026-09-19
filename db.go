package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	`

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite schema: %w", err)
	}

	return &EmailDB{db: db}, nil
}

// SaveEmail indexes an email and its attachment records into SQLite.
func (e *EmailDB) SaveEmail(msg *EmailMessage) error {
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
func (e *EmailDB) GetEmailByMessageID(messageID string) (*EmailMessage, error) {
	cleanID := strings.TrimSpace(messageID)
	if cleanID == "" {
		return nil, nil
	}

	row := e.db.QueryRow(`
		SELECT message_id, in_reply_to, subject, sender, sender_email, body_text, date
		FROM emails
		WHERE message_id = ?;
	`, cleanID)

	var msg EmailMessage
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
func (e *EmailDB) SearchEmails(query string, limit int) ([]*EmailMessage, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	// Attempt FTS5 MATCH query first
	ftsQuery := `
		SELECT e.message_id, e.in_reply_to, e.subject, e.sender, e.sender_email, e.body_text, e.date
		FROM emails e
		JOIN emails_fts fts ON e.message_id = fts.message_id
		WHERE emails_fts MATCH ?
		ORDER BY rank
		LIMIT ?;
	`

	// Clean query for FTS syntax
	sanitized := strings.ReplaceAll(cleanQuery, `"`, `""`)
	rows, err := e.db.Query(ftsQuery, `"`+sanitized+`"`, limit)
	if err != nil {
		// Fallback to LIKE if FTS expression parsing fails
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

	var results []*EmailMessage
	for rows.Next() {
		var msg EmailMessage
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

// Close closes the underlying SQLite database connection.
func (e *EmailDB) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}
