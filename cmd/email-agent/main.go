package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"email-agent/internal/antigravity"
	"email-agent/internal/config"
	"email-agent/internal/db"
	"email-agent/internal/gemini"
	"email-agent/internal/models"
	"email-agent/internal/parser"
	"email-agent/internal/skills"
	"email-agent/internal/transport"
)

func main() {
	log.Println("[INFO] Starting Email AI Assistant daemon...")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Configuration error: %v", err)
	}

	// Ensure persistent directories exist
	if dbDir := filepath.Dir(cfg.DBPath); dbDir != "" && dbDir != "." {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Printf("[WARN] Could not create database directory %s: %v", dbDir, err)
		}
	}
	if cfg.AttachmentsDir != "" {
		if err := os.MkdirAll(cfg.AttachmentsDir, 0755); err != nil {
			log.Printf("[WARN] Could not create attachments directory %s: %v", cfg.AttachmentsDir, err)
		}
	}
	if cfg.WorkspaceDir != "" {
		if err := os.MkdirAll(cfg.WorkspaceDir, 0755); err != nil {
			log.Printf("[WARN] Could not create workspace directory %s: %v", cfg.WorkspaceDir, err)
		}
	}

	systemPrompt := ""
	if promptBytes, err := os.ReadFile(cfg.SystemPromptPath); err == nil {
		systemPrompt = string(promptBytes)
		log.Printf("[INFO] Loaded default prompt from %s (%d bytes)", cfg.SystemPromptPath, len(systemPrompt))
	} else {
		log.Printf("[WARN] Could not read %s, using internal fallback prompt", cfg.SystemPromptPath)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize local SQLite cache
	emailDB, err := db.NewEmailDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize SQLite email cache (%s): %v", cfg.DBPath, err)
	}
	defer emailDB.Close()
	log.Printf("[INFO] Local SQLite cache initialized at %s", cfg.DBPath)

	var geminiClient *gemini.GeminiClient
	var agyRunner *antigravity.Runner

	if cfg.AIBackend == "antigravity" {
		agyRunner = antigravity.NewRunner(cfg.AntigravityBin).WithWorkspace(cfg.WorkspaceDir)
		log.Printf("[INFO] AI backend: Antigravity CLI (%s) in workspace %s with Pro subscription", cfg.AntigravityBin, cfg.WorkspaceDir)
	} else {
		var err error
		geminiClient, err = gemini.NewGeminiClient(ctx, cfg, systemPrompt)
		if err != nil {
			log.Fatalf("[FATAL] Failed to initialize Gemini client: %v", err)
		}
		if cfg.GeminiAPIKey != "" {
			log.Printf("[INFO] AI backend: Google AI Studio API Key (model: %s)", cfg.GeminiModel)
		} else {
			log.Printf("[INFO] AI backend: Vertex AI (project: %s, model: %s)", cfg.GCPProjectID, cfg.GeminiModel)
		}
	}

	emailTransport := transport.NewTransport(cfg)
	if err := emailTransport.ConnectIMAP(); err != nil {
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

		messages, err := emailTransport.FetchUnseen(ctx)
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
			if parser.IsLoopOrAutoReply(msg.SenderEmail, cfg.GmailAddress, "", "") {
				log.Printf("[WARN] Skipping message UID=%d: matches self address or auto-reply header", msg.UID)
				_ = emailTransport.MarkSeen(ctx, msg.UID)
				processedUIDs[msg.UID] = true
				continue
			}

			// 2. Sender whitelist verification
			if !cfg.IsAllowedSender(msg.SenderEmail) {
				log.Printf("[SECURITY] Unauthorized sender %q (UID=%d). Dropping message.", msg.SenderEmail, msg.UID)
				_ = emailTransport.MarkSeen(ctx, msg.UID)
				processedUIDs[msg.UID] = true
				continue
			}

			// Save inbound message into local SQLite cache
			if err := emailDB.SaveEmail(msg); err != nil {
				log.Printf("[WARN] Failed indexing message UID=%d into SQLite: %v", msg.UID, err)
			}

			// 3. Resolve active skill instruction
			activePrompt, skillName := skills.ResolveSkill(cfg.SkillsDir, msg.Subject, msg.BodyText, systemPrompt)
			log.Printf("[INFO] Using skill %q for UID=%d", skillName, msg.UID)

			// 4. Reconstruct conversation thread history (check local cache first, fallback to IMAP)
			fetcher := func(msgID string) (*models.EmailMessage, error) {
				if localMsg, err := emailDB.GetEmailByMessageID(msgID); err == nil && localMsg != nil {
					return localMsg, nil
				}
				return emailTransport.FetchByMessageID(ctx, msgID)
			}

			turns, err := parser.AssembleThread(fetcher, msg.References, msg, cfg.GmailAddress)
			if err != nil {
				log.Printf("[ERROR] Failed assembling thread history for UID=%d: %v", msg.UID, err)
				continue
			}
			log.Printf("[INFO] Assembled conversation thread with %d turn(s) for UID=%d", len(turns), msg.UID)

			var replyMarkdown string
			if cfg.AIBackend == "antigravity" {
				var promptBuilder strings.Builder
				promptBuilder.WriteString(fmt.Sprintf("Inbound email from: %s\nSubject: %s\nDate: %s\n",
					msg.Sender, msg.Subject, msg.Date.Format(time.RFC1123Z)))
				if len(msg.Attachments) > 0 {
					promptBuilder.WriteString("Attachments:\n")
					for _, att := range msg.Attachments {
						promptBuilder.WriteString(fmt.Sprintf("- %s (%s, path: %s)\n", att.Filename, att.ContentType, att.Path))
					}
				}
				if activePrompt != "" {
					promptBuilder.WriteString(fmt.Sprintf("\n[Active Skill - %s]:\n%s\n", skillName, activePrompt))
				}

				if len(turns) > 1 {
					promptBuilder.WriteString("\n[Conversation Thread History]:\n")
					for i, t := range turns {
						roleLabel := "User"
						if t.Role == "model" {
							roleLabel = "Assistant"
						}
						promptBuilder.WriteString(fmt.Sprintf("Turn %d (%s):\n%s\n\n", i+1, roleLabel, strings.TrimSpace(t.Content)))
					}
				} else {
					promptBuilder.WriteString(fmt.Sprintf("\nEmail Content:\n%s\n", msg.BodyText))
				}

				// Automated keyword search match injection
				lowerBody := strings.ToLower(msg.Subject + " " + msg.BodyText)
				if strings.Contains(lowerBody, "discuss") ||
					strings.Contains(lowerBody, "last time") ||
					strings.Contains(lowerBody, "previous") ||
					strings.Contains(lowerBody, "earlier") ||
					strings.Contains(lowerBody, "remember") ||
					strings.Contains(lowerBody, "search") {
					words := strings.Fields(msg.Subject + " " + msg.BodyText)
					for _, w := range words {
						cleanW := strings.Trim(w, "?.,!\"'()[]")
						if len(cleanW) > 3 {
							matches, _ := emailDB.SearchEmails(cleanW, 3)
							var matchItems []string
							for _, m := range matches {
								if m.MessageID == msg.MessageID {
									continue
								}
								snip := strings.TrimSpace(m.BodyText)
								if len(snip) > 200 {
									snip = snip[:200] + "..."
								}
								matchItems = append(matchItems, fmt.Sprintf("- [%s] From: %s | Subject: %q | %s",
									m.Date.Format("2006-01-02 15:04"), m.Sender, m.Subject, snip))
							}
							if len(matchItems) > 0 {
								promptBuilder.WriteString(fmt.Sprintf("\n[Relevant Past Email Archive Matches for %q]:\n%s\n", cleanW, strings.Join(matchItems, "\n")))
								break
							}
						}
					}
				}

				promptBuilder.WriteString("\nPlease write a concise, professional reply to the sender.")

				log.Printf("[INFO] Dispatching to Antigravity CLI (stateless execution)...")
				replyMarkdown, err = agyRunner.Execute(ctx, promptBuilder.String())
				if err != nil {
					log.Printf("[ERROR] Antigravity execution failed for UID=%d: %v", msg.UID, err)
					continue
				}
			} else {
				activeGemini := geminiClient.WithSystemPrompt(activePrompt)

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
				replyMarkdown, err = activeGemini.GenerateReply(ctx, turns, searchCallback)
				if err != nil {
					log.Printf("[ERROR] Gemini generation failed for UID=%d: %v", msg.UID, err)
					continue
				}
			}

			// 7. Format minimal HTML and plain text
			htmlBody, textBody, err := parser.FormatReply(replyMarkdown)
			if err != nil {
				log.Printf("[WARN] Formatter fallback to raw text: %v", err)
				htmlBody = "<p>" + replyMarkdown + "</p>"
				textBody = replyMarkdown
			}

			// 8. Send threaded SMTP reply
			replySubject := parser.NormalizeSubject(msg.Subject)
			err = emailTransport.SendReply(msg.Sender, replySubject, msg.MessageID, msg.References, htmlBody, textBody)
			if err != nil {
				log.Printf("[ERROR] Failed sending SMTP reply for UID=%d: %v", msg.UID, err)
				continue
			}
			log.Printf("[INFO] Successfully sent reply to %q for subject %q", msg.SenderEmail, replySubject)

			// Cache agent's outgoing reply in SQLite so future turns and searches can recall it
			replyMsg := &models.EmailMessage{
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
			if err := emailTransport.MarkSeen(ctx, msg.UID); err != nil {
				log.Printf("[WARN] Failed marking UID=%d as seen: %v", msg.UID, err)
			}
			processedUIDs[msg.UID] = true
		}

		// Wait for next email event via IMAP IDLE or fallback poll timeout
		if err := emailTransport.WaitForMail(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[WARN] WaitForMail returned: %v", err)
			time.Sleep(3 * time.Second)
		}
	}
}
