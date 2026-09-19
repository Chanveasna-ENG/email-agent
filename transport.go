package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
)

// EmailTransport manages IMAP receiving and SMTP sending.
type EmailTransport struct {
	cfg         *Config
	imapMu      sync.Mutex
	imapClient  *imapclient.Client
	newMailChan chan struct{}
}

// NewTransport initializes a transport instance.
func NewTransport(cfg *Config) *EmailTransport {
	return &EmailTransport{
		cfg:         cfg,
		newMailChan: make(chan struct{}, 10),
	}
}

// ConnectIMAP establishes and logs in an IMAP connection to Gmail.
func (t *EmailTransport) ConnectIMAP() error {
	t.imapMu.Lock()
	defer t.imapMu.Unlock()

	if t.imapClient != nil {
		_ = t.imapClient.Close()
	}

	opts := &imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					select {
					case t.newMailChan <- struct{}{}:
					default:
					}
				}
			},
		},
	}

	client, err := imapclient.DialTLS("imap.gmail.com:993", opts)
	if err != nil {
		return fmt.Errorf("dial imap: %w", err)
	}

	if err := client.Login(t.cfg.GmailAddress, t.cfg.GmailAppPassword).Wait(); err != nil {
		_ = client.Close()
		return fmt.Errorf("imap login failed: %w", err)
	}

	if _, err := client.Select("INBOX", nil).Wait(); err != nil {
		_ = client.Close()
		return fmt.Errorf("select INBOX failed: %w", err)
	}

	t.imapClient = client
	return nil
}

// ensureIMAPConnected verifies connection or reconnects if closed.
func (t *EmailTransport) ensureIMAPConnected() (*imapclient.Client, error) {
	t.imapMu.Lock()
	defer t.imapMu.Unlock()

	if t.imapClient == nil || t.imapClient.State() < imap.ConnStateAuthenticated {
		opts := &imapclient.Options{
			UnilateralDataHandler: &imapclient.UnilateralDataHandler{
				Mailbox: func(data *imapclient.UnilateralDataMailbox) {
					if data.NumMessages != nil {
						select {
						case t.newMailChan <- struct{}{}:
						default:
						}
					}
				},
			},
		}
		client, err := imapclient.DialTLS("imap.gmail.com:993", opts)
		if err != nil {
			return nil, fmt.Errorf("reconnect imap: %w", err)
		}
		if err := client.Login(t.cfg.GmailAddress, t.cfg.GmailAppPassword).Wait(); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("imap login: %w", err)
		}
		if _, err := client.Select("INBOX", nil).Wait(); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("select INBOX: %w", err)
		}
		t.imapClient = client
	}
	return t.imapClient, nil
}

// FetchUnseen retrieves all unseen email messages from INBOX.
func (t *EmailTransport) FetchUnseen(ctx context.Context) ([]*EmailMessage, error) {
	client, err := t.ensureIMAPConnected()
	if err != nil {
		return nil, err
	}

	criteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}

	uidsData, err := client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("search unseen messages: %w", err)
	}

	uids := uidsData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	var messages []*EmailMessage
	for _, uid := range uids {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg, err := t.fetchMessageByUID(client, uid)
		if err != nil {
			continue
		}
		if msg != nil {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// FetchByMessageID searches INBOX for a specific message by its Message-ID header.
func (t *EmailTransport) FetchByMessageID(ctx context.Context, messageID string) (*EmailMessage, error) {
	cleanID := strings.TrimSpace(messageID)
	if cleanID == "" {
		return nil, nil
	}

	client, err := t.ensureIMAPConnected()
	if err != nil {
		return nil, err
	}

	criteria := &imap.SearchCriteria{
		Header: []imap.SearchCriteriaHeaderField{
			{Key: "Message-ID", Value: cleanID},
		},
	}

	uidsData, err := client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("search by message id: %w", err)
	}

	uids := uidsData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	// Fetch the most recent match for this Message-ID
	return t.fetchMessageByUID(client, uids[len(uids)-1])
}

// fetchMessageByUID retrieves message body and headers for a given UID.
func (t *EmailTransport) fetchMessageByUID(client *imapclient.Client, uid imap.UID) (*EmailMessage, error) {
	bodySection := &imap.FetchItemBodySection{}
	fetchOpts := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}

	uidSet := imap.UIDSetNum(uid)
	cmd := client.Fetch(uidSet, fetchOpts)
	messages, err := cmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch uid %d: %w", uid, err)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	buf := messages[0]
	rawBody := buf.FindBodySection(bodySection)
	if len(rawBody) == 0 {
		return nil, nil
	}

	return parseRawEmail(uint32(buf.UID), rawBody, t.cfg.AttachmentsDir)
}

// parseRawEmail decodes MIME headers, plain text body, and extracts attachments.
func parseRawEmail(uid uint32, rawBytes []byte, attachmentsDir string) (*EmailMessage, error) {
	mr, err := mail.CreateReader(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("create mail reader: %w", err)
	}

	header := mr.Header
	subject, _ := header.Subject()
	date, _ := header.Date()
	fromList, _ := header.AddressList("From")
	senderRaw := ""
	if len(fromList) > 0 {
		senderRaw = fromList[0].String()
	}
	senderEmail := ExtractEmailAddress(senderRaw)

	messageID, _ := header.MessageID()
	inReplyTo := header.Get("In-Reply-To")
	rawReferences := header.Get("References")
	references := ParseReferences(rawReferences)

	bodyText, attachments, _ := ExtractEmailParts(mr)

	// Fallback if multipart had no plain text
	if bodyText == "" {
		bodyText = string(rawBytes)
	}

	if err := saveAttachmentsToDisk(attachmentsDir, messageID, attachments); err != nil {
		// Log or continue; do not abort email receipt on disk write error
	}

	return &EmailMessage{
		UID:         uid,
		MessageID:   messageID,
		InReplyTo:   inReplyTo,
		References:  references,
		Subject:     subject,
		Sender:      senderRaw,
		SenderEmail: senderEmail,
		BodyText:    bodyText,
		Date:        date,
		Attachments: attachments,
	}, nil
}

// saveAttachmentsToDisk persists extracted attachments to attachments/<message_id>/<filename>.
func saveAttachmentsToDisk(baseDir, messageID string, attachments []Attachment) error {
	if len(attachments) == 0 || baseDir == "" {
		return nil
	}

	safeID := strings.Trim(messageID, "<>")
	safeID = strings.ReplaceAll(safeID, "/", "_")
	safeID = strings.ReplaceAll(safeID, "\\", "_")
	safeID = strings.ReplaceAll(safeID, ":", "_")
	if safeID == "" {
		safeID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}

	targetDir := filepath.Join(baseDir, safeID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create attachment directory: %w", err)
	}

	for i := range attachments {
		safeName := filepath.Base(attachments[i].Filename)
		if safeName == "." || safeName == "/" || safeName == "" {
			safeName = fmt.Sprintf("attachment_%d", i+1)
		}
		destPath := filepath.Join(targetDir, safeName)
		if err := os.WriteFile(destPath, attachments[i].Data, 0644); err == nil {
			attachments[i].Path = destPath
		}
	}

	return nil
}

// MarkSeen adds the \Seen flag to a message.
func (t *EmailTransport) MarkSeen(ctx context.Context, uid uint32) error {
	client, err := t.ensureIMAPConnected()
	if err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	storeFlags := &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagSeen},
		Silent: true,
	}

	return client.Store(uidSet, storeFlags, nil).Close()
}

// WaitForMail waits for incoming emails using IMAP IDLE and poll timeout.
func (t *EmailTransport) WaitForMail(ctx context.Context) error {
	client, err := t.ensureIMAPConnected()
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(t.cfg.PollInterval):
			return nil
		}
	}

	idleCmd, err := client.Idle()
	if err != nil {
		// Fallback to simple polling interval if IDLE fails
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(t.cfg.PollInterval):
			return nil
		}
	}

	select {
	case <-ctx.Done():
		_ = idleCmd.Close()
		return ctx.Err()
	case <-t.newMailChan:
		_ = idleCmd.Close()
		return nil
	case <-time.After(t.cfg.PollInterval):
		_ = idleCmd.Close()
		return nil
	}
}

// SendReply sends a threaded email reply via Gmail SMTP.
func (t *EmailTransport) SendReply(to, subject, inReplyTo string, references []string, htmlBody, textBody string) error {
	rawMime, err := buildMimeMessage(t.cfg.GmailAddress, to, subject, inReplyTo, references, htmlBody, textBody)
	if err != nil {
		return fmt.Errorf("build mime message: %w", err)
	}

	destAddr := ExtractEmailAddress(to)
	if destAddr == "" {
		return fmt.Errorf("invalid recipient email address: %q", to)
	}

	auth := smtp.PlainAuth("", t.cfg.GmailAddress, t.cfg.GmailAppPassword, "smtp.gmail.com")
	addr := "smtp.gmail.com:587"

	// Connect with TLS
	tlsconfig := &tls.Config{
		ServerName: "smtp.gmail.com",
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer client.Close()

	if err := client.StartTLS(tlsconfig); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}

	if err := client.Mail(t.cfg.GmailAddress); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}

	if err := client.Rcpt(destAddr); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	if _, err := w.Write(rawMime); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}

	return client.Quit()
}

// buildMimeMessage formats a multipart/alternative RFC 5322 email with threading headers.
func buildMimeMessage(from, to, subject, inReplyTo string, references []string, htmlBody, textBody string) ([]byte, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	boundary := fmt.Sprintf("boundary_%s", hex.EncodeToString(b))

	// Combine references: prior references + inReplyTo
	var allRefs []string
	for _, r := range references {
		clean := strings.TrimSpace(r)
		if clean != "" {
			allRefs = append(allRefs, clean)
		}
	}
	if inReplyTo != "" && !sliceContains(allRefs, inReplyTo) {
		allRefs = append(allRefs, inReplyTo)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))

	randomID := hex.EncodeToString(b[:8])
	domain := "gmail.com"
	if parts := strings.Split(from, "@"); len(parts) == 2 {
		domain = parts[1]
	}
	sb.WriteString(fmt.Sprintf("Message-ID: <%s@%s>\r\n", randomID, domain))

	if inReplyTo != "" {
		sb.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", inReplyTo))
	}
	if len(allRefs) > 0 {
		sb.WriteString(fmt.Sprintf("References: %s\r\n", strings.Join(allRefs, " ")))
	}

	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))

	// Plain text section
	sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	sb.WriteString(textBody)
	sb.WriteString("\r\n\r\n")

	// HTML section
	sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	sb.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	sb.WriteString(htmlBody)
	sb.WriteString("\r\n\r\n")

	sb.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return []byte(sb.String()), nil
}

func sliceContains(slice []string, val string) bool {
	cleanVal := strings.ToLower(strings.TrimSpace(val))
	for _, item := range slice {
		if strings.ToLower(strings.TrimSpace(item)) == cleanVal {
			return true
		}
	}
	return false
}
