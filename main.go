package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	log.Println("[INFO] Starting Email AI Assistant daemon (Phase 2)...")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Configuration error: %v", err)
	}

	systemPrompt := ""
	if promptBytes, err := os.ReadFile(cfg.SystemPromptPath); err == nil {
		systemPrompt = string(promptBytes)
		log.Printf("[INFO] Loaded default system prompt from %s (%d bytes)", cfg.SystemPromptPath, len(systemPrompt))
	} else {
		log.Printf("[WARN] Could not read %s, using internal fallback prompt", cfg.SystemPromptPath)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize local SQLite cache
	emailDB, err := NewEmailDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize SQLite email cache (%s): %v", cfg.DBPath, err)
	}
	defer emailDB.Close()
	log.Printf("[INFO] Local SQLite cache initialized at %s", cfg.DBPath)

	geminiClient, err := NewGeminiClient(ctx, cfg, systemPrompt)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize Gemini client: %v", err)
	}
	if cfg.GeminiAPIKey != "" {
		log.Printf("[INFO] Gemini client ready via Google AI Studio API Key (model: %s)", cfg.GeminiModel)
	} else {
		log.Printf("[INFO] Gemini client ready via Vertex AI (project: %s, model: %s)", cfg.GCPProjectID, cfg.GeminiModel)
	}

	transport := NewTransport(cfg)
	if err := transport.ConnectIMAP(); err != nil {
		log.Fatalf("[FATAL] Failed initial IMAP connection: %v", err)
	}
	log.Printf("[INFO] Connected to Gmail IMAP as %s", cfg.GmailAddress)
	log.Printf("[INFO] Allowed senders whitelist: %v", cfg.AllowedSenders)

	processedUIDs := make(map[uint32]bool)

	log.Println("[INFO] Entering main listener loop...")

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Shutdown signal received, terminating...")
			return
		default:
		}

		messages, err := transport.FetchUnseen(ctx)
		if err != nil {
			log.Printf("[ERROR] Error fetching unseen messages: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range messages {
			if processedUIDs[msg.UID] {
				continue
			}

			log.Printf("[INFO] Inbound message: UID=%d, From=%q, Subject=%q, Attachments=%d",
				msg.UID, msg.Sender, msg.Subject, len(msg.Attachments))

			// 1. Loop and auto-reply prevention
			if IsLoopOrAutoReply(msg.SenderEmail, cfg.GmailAddress, "", "") {
				log.Printf("[WARN] Skipping message UID=%d: matches self address or auto-reply header", msg.UID)
				_ = transport.MarkSeen(ctx, msg.UID)
				processedUIDs[msg.UID] = true
				continue
			}

			// 2. Sender whitelist verification
			if !sliceContains(cfg.AllowedSenders, msg.SenderEmail) {
				log.Printf("[SECURITY] Unauthorized sender %q (UID=%d). Dropping message.", msg.SenderEmail, msg.UID)
				_ = transport.MarkSeen(ctx, msg.UID)
				processedUIDs[msg.UID] = true
				continue
			}

			// Save inbound message into local SQLite cache
			if err := emailDB.SaveEmail(msg); err != nil {
				log.Printf("[WARN] Failed indexing message UID=%d into SQLite: %v", msg.UID, err)
			}

			// 3. Resolve persona instruction
			activePrompt, personaName := ResolvePersona(cfg.PersonasDir, msg.Subject, msg.BodyText, systemPrompt)
			log.Printf("[INFO] Using persona %q for UID=%d", personaName, msg.UID)
			activeGemini := geminiClient.WithSystemPrompt(activePrompt)

			// 4. Reconstruct zero-DB thread history (check local cache first, fallback to IMAP)
			fetcher := func(msgID string) (*EmailMessage, error) {
				if localMsg, err := emailDB.GetEmailByMessageID(msgID); err == nil && localMsg != nil {
					return localMsg, nil
				}
				return transport.FetchByMessageID(ctx, msgID)
			}

			turns, err := AssembleThread(fetcher, msg.References, msg, cfg.GmailAddress)
			if err != nil {
				log.Printf("[ERROR] Failed assembling thread history for UID=%d: %v", msg.UID, err)
				continue
			}
			log.Printf("[INFO] Assembled conversation thread with %d turn(s) for UID=%d", len(turns), msg.UID)

			// 5. Define personal email search callback for Gemini tool use
			searchCallback := func(query string) (string, error) {
				log.Printf("[TOOL] Executing search_past_emails: %q", query)
				results, err := emailDB.SearchEmails(query, 5)
				if err != nil {
					return "", err
				}
				if len(results) == 0 {
					return "No matching past emails found in user's inbox for this query.", nil
				}
				var sb strings.Builder
				for i, r := range results {
					sb.WriteString(fmt.Sprintf("[%d] Date: %s | From: %s | Subject: %s\nSnippet: %s\n\n",
						i+1, r.Date.Format("2006-01-02 15:04"), r.Sender, r.Subject, r.BodyText))
				}
				return sb.String(), nil
			}

			// 6. Generate LLM response via Gemini (with tools + multimodal attachments)
			replyMarkdown, err := activeGemini.GenerateReply(ctx, turns, searchCallback)
			if err != nil {
				log.Printf("[ERROR] Gemini generation failed for UID=%d: %v", msg.UID, err)
				continue
			}

			// 7. Format minimal HTML and plain text
			htmlBody, textBody, err := FormatReply(replyMarkdown)
			if err != nil {
				log.Printf("[WARN] Formatter fallback to raw text: %v", err)
				htmlBody = "<p>" + replyMarkdown + "</p>"
				textBody = replyMarkdown
			}

			// 8. Send threaded SMTP reply
			replySubject := NormalizeSubject(msg.Subject)
			err = transport.SendReply(msg.Sender, replySubject, msg.MessageID, msg.References, htmlBody, textBody)
			if err != nil {
				log.Printf("[ERROR] Failed sending SMTP reply for UID=%d: %v", msg.UID, err)
				continue
			}
			log.Printf("[INFO] Successfully sent reply to %q for subject %q", msg.SenderEmail, replySubject)

			// Cache agent's outgoing reply in SQLite so future turns and searches can recall it
			replyMsg := &EmailMessage{
				MessageID:   fmt.Sprintf("<agent_%d@%s>", time.Now().UnixNano(), "local"),
				InReplyTo:   msg.MessageID,
				References:  append(msg.References, msg.MessageID),
				Subject:     replySubject,
				Sender:      cfg.GmailAddress,
				SenderEmail: cfg.GmailAddress,
				BodyText:    textBody,
				Date:        time.Now(),
			}
			_ = emailDB.SaveEmail(replyMsg)

			// 9. Mark as seen in IMAP
			if err := transport.MarkSeen(ctx, msg.UID); err != nil {
				log.Printf("[WARN] Failed marking UID=%d as seen: %v", msg.UID, err)
			}
			processedUIDs[msg.UID] = true
		}

		// Wait for next email event via IMAP IDLE or fallback poll timeout
		if err := transport.WaitForMail(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[WARN] WaitForMail returned: %v", err)
			time.Sleep(3 * time.Second)
		}
	}
}
