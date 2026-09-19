package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Println("[INFO] Starting Email AI Assistant daemon...")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Configuration error: %v", err)
	}

	systemPrompt := ""
	if promptBytes, err := os.ReadFile(cfg.SystemPromptPath); err == nil {
		systemPrompt = string(promptBytes)
		log.Printf("[INFO] Loaded system prompt from %s (%d bytes)", cfg.SystemPromptPath, len(systemPrompt))
	} else {
		log.Printf("[WARN] Could not read %s, running with default prompt", cfg.SystemPromptPath)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	geminiClient, err := NewGeminiClient(ctx, cfg, systemPrompt)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize Vertex AI Gemini client: %v", err)
	}
	log.Printf("[INFO] Vertex AI client ready (project: %s, model: %s)", cfg.GCPProjectID, cfg.GeminiModel)

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

			log.Printf("[INFO] Inbound message: UID=%d, From=%q, Subject=%q", msg.UID, msg.Sender, msg.Subject)

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

			// 3. Reconstruct zero-DB thread history
			fetcher := func(msgID string) (*EmailMessage, error) {
				return transport.FetchByMessageID(ctx, msgID)
			}

			turns, err := AssembleThread(fetcher, msg.References, msg, cfg.GmailAddress)
			if err != nil {
				log.Printf("[ERROR] Failed assembling thread history for UID=%d: %v", msg.UID, err)
				continue
			}
			log.Printf("[INFO] Assembled conversation thread with %d turn(s) for UID=%d", len(turns), msg.UID)

			// 4. Generate LLM response via Vertex AI
			replyMarkdown, err := geminiClient.GenerateReply(ctx, turns)
			if err != nil {
				log.Printf("[ERROR] Gemini generation failed for UID=%d: %v", msg.UID, err)
				continue
			}

			// 5. Format minimal HTML and plain text
			htmlBody, textBody, err := FormatReply(replyMarkdown)
			if err != nil {
				log.Printf("[WARN] Formatter fallback to raw text: %v", err)
				htmlBody = "<p>" + replyMarkdown + "</p>"
				textBody = replyMarkdown
			}

			// 6. Send threaded SMTP reply
			replySubject := NormalizeSubject(msg.Subject)
			err = transport.SendReply(msg.Sender, replySubject, msg.MessageID, msg.References, htmlBody, textBody)
			if err != nil {
				log.Printf("[ERROR] Failed sending SMTP reply for UID=%d: %v", msg.UID, err)
				continue
			}
			log.Printf("[INFO] Successfully sent reply to %q for subject %q", msg.SenderEmail, replySubject)

			// 7. Mark as seen in IMAP
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
