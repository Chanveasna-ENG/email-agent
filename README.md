# Email AI Assistant (Go)

A personal AI assistant communicating directly through your email client interface. Inbound emails are detected in real-time via IMAP IDLE, threaded conversation context is reconstructed directly from Gmail headers without an external database, answers are generated using Google Gemini on Vertex AI, and replies are dispatched via authenticated SMTP with clean, minimal HTML formatting.

## Architecture

```
Incoming Email (Gmail)
       │
       ▼
 [IMAP IDLE Listener]  ───>  [Loop & Auto-Reply Filter]
                                    │
                                    ▼
                         [Sender Whitelist Check]
                                    │
                                    ▼
                       [Zero-DB Thread Reconstructor]
                         (Fetches prior References)
                                    │
                                    ▼
                          [Vertex AI Gemini API]
                                    │
                                    ▼
                          [Minimal HTML Renderer]
                                    │
                                    ▼
                        [Authenticated SMTP Sender]
                      (In-Reply-To + References set)
                                    │
                                    ▼
                         [Mark IMAP Seen (\Seen)]
```

## Features

- **Email Client Interface**: Text with your AI agent as naturally as emailing a colleague.
- **Zero-DB Thread Reconstruction**: Reconstructs multi-turn dialogue from Gmail IMAP headers (`Message-ID`, `References`, `In-Reply-To`) without maintaining a separate database.
- **Strict Sender Whitelist**: Protects against unauthorized email triggers and token cost.
- **Self-Loop & Auto-Reply Protection**: Ignores agent's own address and automated bounce messages.
- **Minimal HTML Formatting**: Converts Markdown output into clean, unbloated email HTML with plain-text fallback.
- **Configurable Persona**: Define agent instructions via external `config/system_prompt.txt` or `system_prompt.txt`.

## Prerequisites

- **Go**: 1.22+ installed (`go version`).
- **Gmail Account**:
  - 2-Step Verification enabled.
  - 16-character [Google App Password](https://myaccount.google.com/apppasswords).
- **Google Cloud Platform**:
  - GCP Project with Vertex AI API enabled.
  - Service Account JSON key with `Vertex AI User` role.

## Configuration

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Fill in the required environment variables in `.env`:
   ```env
   GMAIL_ADDRESS=your-ai-agent@gmail.com
   GMAIL_APP_PASSWORD=xxxx xxxx xxxx xxxx
   ALLOWED_SENDERS=your-email@example.com
   GCP_PROJECT_ID=your-gcp-project-id
   GCP_LOCATION=us-central1
   GOOGLE_APPLICATION_CREDENTIALS=credential/service-account.json
   GEMINI_MODEL=gemini-2.5-flash
   SYSTEM_PROMPT_FILE=config/system_prompt.txt
   ```

## Development & Quality Assurance

### Using PowerShell (Windows)
```powershell
# Format, analyze, test, and compile
.\build.ps1

# Run tests only
.\build.ps1 -TestOnly

# Build binary only
.\build.ps1 -BuildOnly
```

### Using Make / Standard Go CLI
```bash
# Format code
go fmt ./...

# Static analysis
go vet ./...

# Run unit test suite with coverage
go test -v -cover ./...

# Compile daemon binary
go build -v -o email-agent.exe .
```

## Running the Daemon

```bash
.\email-agent.exe
```
The service connects to Gmail IMAP, logs in, enters the IDLE loop, and awaits inbound messages from whitelisted senders.
