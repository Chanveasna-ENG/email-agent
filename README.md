# Email AI Assistant (Go)

A personal AI assistant communicating directly through your email client interface. Inbound emails are detected in real-time via IMAP IDLE, threaded conversation context is reconstructed directly from Gmail headers and local SQLite cache, answers are generated using Google Gemini (Google AI Studio API Key or GCP Vertex AI), and replies are dispatched via authenticated SMTP with clean, minimal HTML formatting.

## Architecture & Project Structure

```
.
├── cmd/
│   └── email-agent/
│       └── main.go              # Main daemon entrypoint
├── internal/
│   ├── config/                  # Configuration loader & validation
│   ├── db/                      # SQLite FTS5 caching & search (modernc.org/sqlite, pure Go)
│   ├── gemini/                  # Google GenAI SDK integration (tools, multimodal, search)
│   ├── models/                  # Shared domain types (EmailMessage, Attachment, ConversationTurn)
│   ├── parser/                  # MIME parsing, HTML formatting, loop detection, thread reconstruction
│   ├── personas/                # Persona dynamic resolution based on subject/body tags
│   └── transport/               # Gmail IMAP IDLE listener & SMTP sender
├── data/                        # Persistent storage (emails.db, attachments/)
├── personas/                    # Dynamic persona prompt files (default.txt, coder.txt, etc.)
├── Dockerfile                   # Multi-stage distroless container build
├── docker-compose.yml           # Linux deployment config (always restart, non-root)
├── Makefile                     # Build & docker commands
└── build.ps1                    # PowerShell build & test script
```

## Features

- **Email Client Interface**: Text with your AI agent as naturally as emailing a colleague.
- **Zero-DB & Cached Thread Reconstruction**: Reconstructs multi-turn dialogue from Gmail IMAP headers (`Message-ID`, `References`, `In-Reply-To`) with local SQLite fallback.
- **SQLite FTS5 Email Search**: Indexes inbound and outbound emails for fast personal email search tool calls (`search_past_emails`).
- **Multimodal Attachment Support**: Extracts and saves inbound attachments (`data/attachments/`), passing images and PDFs directly to Gemini.
- **Dual Gemini Authentication**: Supports Google AI Studio API keys (`GEMINI_API_KEY`) as well as GCP Vertex AI Service Accounts.
- **Dynamic Personas**: Select prompt profiles automatically using subject/body tags like `[coder]` or `[writer]`.
- **Strict Sender Whitelist**: Protects against unauthorized email triggers and token cost.
- **Self-Loop & Auto-Reply Protection**: Ignores agent's own address and automated bounce messages.
- **Minimal HTML Formatting**: Converts Markdown output into clean, unbloated email HTML with plain-text fallback.
- **Distroless Container Ready**: Pure Go SQLite (`modernc.org/sqlite`) enables 100% static, CGO-free binaries running on `gcr.io/distroless/static-debian12:nonroot`.

## Prerequisites

- **Go**: 1.22+ installed (`go version`).
- **Gmail Account**:
  - 2-Step Verification enabled.
  - 16-character [Google App Password](https://myaccount.google.com/apppasswords).
- **Gemini Auth** (choose either):
  - **Option A**: Google AI Studio API Key (recommended for personal use).
  - **Option B**: GCP Project with Vertex AI API enabled & Service Account JSON key.

## Configuration

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Fill in the required environment variables in `.env`:
   ```env
   # Gmail IMAP/SMTP
   GMAIL_ADDRESS=your-ai-agent@gmail.com
   GMAIL_APP_PASSWORD=xxxx xxxx xxxx xxxx
   ALLOWED_SENDERS=you@example.com

   # Option A: Google AI Studio (API Key)
   GEMINI_API_KEY=your-gemini-api-key

   # Option B: GCP Vertex AI
   # GCP_PROJECT_ID=your-gcp-project-id
   # GCP_LOCATION=us-central1
   # GOOGLE_APPLICATION_CREDENTIALS=credential/service-account.json

   GEMINI_MODEL=gemini-2.5-flash
   DB_PATH=data/emails.db
   ATTACHMENTS_DIR=data/attachments
   PERSONAS_DIR=personas
   ENABLE_GOOGLE_SEARCH=true
   ```

## Development & Quality Assurance

### Using PowerShell (Windows)
```powershell
# Format, analyze, test, and compile Windows binary
.\build.ps1

# Run tests only
.\build.ps1 -TestOnly

# Build Windows binary only
.\build.ps1 -BuildOnly

# Cross-compile static Linux binary (bin/email-agent)
.\build.ps1 -Linux
```

### Using Make / Standard Go CLI
```bash
# Format code
go fmt ./...

# Static analysis
go vet ./...

# Run unit test suite with coverage
go test -v -cover ./...

# Compile daemon binary for Windows
make build

# Compile static Linux binary (CGO_ENABLED=0)
make build-linux
```

## Running Locally

```bash
.\email-agent.exe
```
The service connects to Gmail IMAP, logs in, enters the IDLE loop, and awaits inbound messages from whitelisted senders.

## Linux Homeserver & Docker Deployment

### 1. Distroless Docker (Recommended)
The provided `Dockerfile` compiles a static binary (`CGO_ENABLED=0`) and runs in Google's minimal `gcr.io/distroless/static-debian12:nonroot` container for maximum security:

```bash
# Build and start in background with auto-restart
docker compose up -d --build

# View runtime logs
docker compose logs -f

# Stop container
docker compose down
```

**Docker Security Features**:
- Unprivileged user `nonroot:nonroot` (UID 65532).
- Root CA certificates included for Gmail TLS and Gemini HTTPS.
- `no-new-privileges:true` enabled.
- Persistent storage mapped to host `./data` (`emails.db` and attachments).
- `restart: always` ensures daemon resumes across server reboots.

### 2. Standalone Linux Binary (Systemd)
To run directly on a Linux server without Docker:

1. Cross-compile for Linux:
   ```bash
   make build-linux
   # Or: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/email-agent ./cmd/email-agent
   ```
2. Copy `bin/email-agent`, `.env`, `personas/`, and `data/` to your server (e.g. `/opt/email-agent/`).
3. Create a systemd service `/etc/systemd/system/email-agent.service`:
   ```ini
   [Unit]
   Description=Email AI Assistant Daemon
   After=network.target

   [Service]
   Type=simple
   User=emailagent
   WorkingDirectory=/opt/email-agent
   ExecStart=/opt/email-agent/email-agent
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```
4. Enable and start:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now email-agent
   ```
