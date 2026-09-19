# Email AI Assistant (Go)

A personal AI assistant communicating directly through your email client interface. Inbound emails are detected in real-time via IMAP IDLE, threaded conversation context is reconstructed directly from Gmail headers and local SQLite cache, answers are generated via **Google Antigravity CLI** (`agy` using your Pro subscription) or direct **Google Gemini** (API Key or GCP Vertex AI), and replies are dispatched via authenticated SMTP with clean, minimal HTML formatting.

## Architecture & Project Structure

```
.
├── cmd/
│   └── email-agent/
│       └── main.go              # Main daemon entrypoint
├── internal/
│   ├── antigravity/             # Antigravity CLI (agy) runner with conversation mapping
│   ├── config/                  # Configuration loader & validation
│   ├── db/                      # SQLite FTS5 caching, search & thread-to-conversation mapping
│   ├── gemini/                  # Google GenAI SDK integration (tools, multimodal, search)
│   ├── models/                  # Shared domain types (EmailMessage, Attachment, ConversationTurn)
│   ├── parser/                  # MIME parsing, HTML formatting, loop detection, thread reconstruction
│   ├── skills/                  # Skills loader with YAML frontmatter parsing and tag resolution
│   └── transport/               # Gmail IMAP IDLE listener & SMTP sender
├── data/                        # Persistent storage (emails.db, attachments/)
├── skills/                      # Antigravity & Claude compatible skills
│   ├── default/SKILL.md         # General assistant skill
│   ├── coder/SKILL.md           # Senior software architect skill
│   └── writer/SKILL.md          # Copywriting & editing skill
├── Dockerfile                   # Multi-stage distroless container build (Gemini backend)
├── docker-compose.yml           # Linux deployment config (always restart, non-root)
├── Makefile                     # Build & docker commands
└── build.ps1                    # PowerShell build & test script
```

## Features

- **Email Client Interface**: Text with your AI agent as naturally as emailing a colleague.
- **Hybrid AI Engine (`AI_BACKEND`)**:
  - **Antigravity CLI (Default)**: Uses local `agy` CLI logged in with your Google Antigravity account. Leverages your Pro subscription quota with zero per-token cost, autonomous tool execution (`--dangerously-skip-permissions`), and Antigravity conversation continuity (`--conversation`).
  - **Google Gemini**: Direct cloud API calls via Google AI Studio API Key or Vertex AI.
- **Antigravity / Claude Compatible Skills**: Drop custom skills into `skills/<name>/SKILL.md` with YAML frontmatter (`name`, `description`). Invoke dynamically via `[skill-name]` or `/skill-name` in email subject/body.
- **Thread-to-Conversation Persistence**: Maps RFC 5322 thread root IDs to Antigravity conversation IDs in SQLite, giving your agent continuous multi-turn memory across email exchanges.
- **SQLite FTS5 Email Search**: Indexes inbound and outbound emails for fast personal email search tool calls (`search_past_emails`).
- **Multimodal Attachment Support**: Extracts and saves inbound attachments (`data/attachments/`), passing images and PDFs directly to the AI engine.
- **Strict Sender Whitelist**: Protects against unauthorized email triggers and token cost.
- **Self-Loop & Auto-Reply Protection**: Ignores agent's own address and automated bounce messages.
- **Minimal HTML Formatting**: Converts Markdown output into clean, unbloated email HTML with plain-text fallback.

## Prerequisites

- **Go**: 1.22+ installed (`go version`).
- **Gmail Account**:
  - 2-Step Verification enabled.
  - 16-character [Google App Password](https://myaccount.google.com/apppasswords).
- **AI Backend** (choose either):
  - **Option 1: Antigravity CLI (Recommended)**:
    - Install `agy` on your machine (`agy --help`).
    - Log in once with your Google account.
  - **Option 2: Gemini API**:
    - Google AI Studio API Key (`GEMINI_API_KEY`), OR
    - GCP Vertex AI Service Account key.

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

   # AI Backend: "antigravity" (default) or "gemini"
   AI_BACKEND=antigravity
   AGY_BIN_PATH=agy

   # Skills directory
   SKILLS_DIR=skills

   # Storage
   DB_PATH=data/emails.db
   ATTACHMENTS_DIR=data/attachments
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

## Homeserver Deployment (Linux)

### 1. Automated Host Setup with Sandboxing (Recommended)
The automated script creates a dedicated unprivileged user `emailagent`, isolates the scratch `workspace/`, strips secrets from `agy`'s memory, shields `.env` in `/etc/email-agent/`, and configures hardened systemd namespace security:

1. Cross-compile static Linux binary on your machine (or let the script build it):
   ```bash
   make build-linux
   ```
2. Copy project folder to your Linux homeserver.
3. Run the setup script with `sudo`:
   ```bash
   sudo bash scripts/setup-linux.sh
   ```
4. Verify your credentials in `/etc/email-agent/.env` (chmod `0640`, unreadable by the agent's web browsing scripts):
   ```bash
   sudo nano /etc/email-agent/.env
   ```
5. Start and enable the service:
   ```bash
   sudo systemctl enable --now email-agent
   sudo journalctl -u email-agent -f
   ```

**Security & Sandboxing Architecture**:
- **Process Memory Isolation**: The Go daemon sanitizes `cmd.Env` before spawning `agy`, stripping `GMAIL_APP_PASSWORD` and API tokens. Python scripts executed by `agy` cannot inspect the process environment for secrets.
- **Dedicated Workspace**: `agy` executes strictly in `/opt/email-agent/workspace`. `.env` lives outside in `/etc/email-agent/` with root-restricted permissions. `cat .env` by prompt injection fails.
- **Unprivileged Execution**: Daemon runs as `emailagent` without `sudo` access. Packages are installed strictly in user space (`pip install --user`, `playwright install chromium` inside `/home/emailagent`).
- **Linux Namespace Hardening**: Systemd applies `ProtectSystem=strict` (read-only `/usr`, `/etc`), `ProtectHome=true` (hides user homes and SSH keys), `PrivateTmp=true`, and `NoNewPrivileges=true`.


### 2. Distroless Docker Deployment (Gemini Mode)
For headless `AI_BACKEND=gemini` deployment in a hardened container:
```bash
docker compose up -d --build
docker compose logs -f
```
