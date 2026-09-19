#!/usr/bin/env bash
# scripts/setup-linux.sh - Automated Linux host provisioning for email-agent daemon
set -euo pipefail

if [ "${EUID}" -ne 0 ]; then
  echo "[ERROR] This script must be run as root (use sudo)." >&2
  exit 1
fi

REAL_USER="${SUDO_USER:-$USER}"
REAL_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6 || echo "/home/${REAL_USER}")

echo "==> Setting up Email AI Assistant on Linux for user '${REAL_USER}'..."

# 1. Check dependencies
for cmd in curl python3; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[WARN] Optional or required tool '$cmd' not found in PATH." >&2
  fi
done

# 2. Check Antigravity CLI and ensure system-wide copy in /usr/local/bin
AGY_FOUND=""
for candidate in \
  "/usr/local/bin/agy" \
  "/usr/bin/agy" \
  "${REAL_HOME}/.local/bin/agy" \
  "${REAL_HOME}/.antigravity/bin/agy" \
  "${REAL_HOME}/.gemini/bin/agy" \
  "${REAL_HOME}/bin/agy"; do
  if [ -f "$candidate" ] && [ -x "$candidate" ]; then
    AGY_FOUND="$candidate"
    break
  fi
done

if [ -z "$AGY_FOUND" ] && [ -n "${REAL_USER:-}" ]; then
  USER_BIN=$(su - "${REAL_USER}" -c "command -v agy" 2>/dev/null || true)
  if [ -n "$USER_BIN" ] && [ -x "$USER_BIN" ]; then
    AGY_FOUND="$USER_BIN"
  fi
fi

if [ -n "$AGY_FOUND" ]; then
  echo "==> Found 'agy' binary at: ${AGY_FOUND}"
  if [ "$AGY_FOUND" != "/usr/local/bin/agy" ]; then
    echo "==> Copying to /usr/local/bin/agy (safe for systemd ProtectHome sandbox)..."
    cp -p "$AGY_FOUND" /usr/local/bin/agy
    chmod 0755 /usr/local/bin/agy
  fi
else
  echo "[WARN] 'agy' CLI not found. Attempting install for ${REAL_USER}..."
  su - "${REAL_USER}" -c "curl -fsSL https://antigravity.google/cli/install.sh | bash" || true
  if [ -f "${REAL_HOME}/.local/bin/agy" ]; then
    cp -p "${REAL_HOME}/.local/bin/agy" /usr/local/bin/agy
    chmod 0755 /usr/local/bin/agy
    echo "==> Installed agy to /usr/local/bin/agy successfully."
  else
    echo "[WARN] Could not auto-install 'agy'. Ensure it is installed via:"
    echo "       curl -fsSL https://antigravity.google/cli/install.sh | bash"
    echo "       sudo cp ~/.local/bin/agy /usr/local/bin/agy"
  fi
fi

# 3. Check or compile static binary
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [ ! -f "${SCRIPT_DIR}/bin/email-agent" ]; then
  if ! command -v go >/dev/null 2>&1; then
    if command -v apt-get >/dev/null 2>&1; then
      echo "==> 'go' compiler not found. Installing golang-go via apt..."
      apt-get update -qq && apt-get install -y -qq golang-go
    elif command -v dnf >/dev/null 2>&1; then
      echo "==> 'go' compiler not found. Installing golang via dnf..."
      dnf install -y -q golang
    fi
  fi

  if command -v go >/dev/null 2>&1; then
    echo "==> Compiling static Linux binaries (email-agent & email-search)..."
    (cd "${SCRIPT_DIR}" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/email-agent ./cmd/email-agent)
    (cd "${SCRIPT_DIR}" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/email-search ./cmd/email-search)
  else
    echo "[ERROR] 'bin/email-agent' not found and 'go' could not be found." >&2
    echo "        Run 'sudo apt install -y golang-go' and re-run this script." >&2
    exit 1
  fi
fi

# 4. Create dedicated unprivileged system user
if ! id -u emailagent >/dev/null 2>&1; then
  echo "==> Creating system user 'emailagent'..."
  useradd -r -s /bin/bash -m -d /home/emailagent emailagent
else
  echo "==> System user 'emailagent' already exists."
fi

# 5. Sync Antigravity auth credentials from the invoking user to emailagent
for cred_dir in ".gemini" ".config/antigravity" ".config/agy" ".antigravity"; do
  if [ -d "${REAL_HOME}/${cred_dir}" ]; then
    echo "==> Syncing ${cred_dir} credentials from ${REAL_USER} to emailagent..."
    mkdir -p "/home/emailagent/${cred_dir}"
    cp -r "${REAL_HOME}/${cred_dir}/." "/home/emailagent/${cred_dir}/"
    chown -R emailagent:emailagent "/home/emailagent/${cred_dir}"
    chmod -R 0700 "/home/emailagent/${cred_dir}"
  fi
done

# 6. Provision directories
echo "==> Provisioning directories in /opt/email-agent and /etc/email-agent..."
mkdir -p /opt/email-agent/data
mkdir -p /opt/email-agent/workspace
mkdir -p /etc/email-agent

cp "${SCRIPT_DIR}/bin/email-agent" /opt/email-agent/email-agent
chmod 0755 /opt/email-agent/email-agent

if [ -f "${SCRIPT_DIR}/bin/email-search" ]; then
  cp "${SCRIPT_DIR}/bin/email-search" /opt/email-agent/workspace/email-search
  chmod 0755 /opt/email-agent/workspace/email-search
fi

if [ -d "${SCRIPT_DIR}/skills" ]; then
  cp -r "${SCRIPT_DIR}/skills" /opt/email-agent/
fi

# Provision credentials file outside workspace
if [ ! -f /etc/email-agent/.env ]; then
  if [ -f "${SCRIPT_DIR}/.env" ]; then
    echo "==> Copying local .env to /etc/email-agent/.env..."
    cp "${SCRIPT_DIR}/.env" /etc/email-agent/.env
  else
    echo "==> Copying .env.example to /etc/email-agent/.env..."
    cp "${SCRIPT_DIR}/.env.example" /etc/email-agent/.env
  fi
fi

# Shield .env: readable only by root and emailagent group, invisible to workspace
chown root:emailagent /etc/email-agent/.env
chmod 0640 /etc/email-agent/.env

# Permissions: code is root-owned (immutable by agent); data & workspace writable by agent
chown -R root:root /opt/email-agent
chown -R emailagent:emailagent /opt/email-agent/data /opt/email-agent/workspace /home/emailagent
chmod 0750 /opt/email-agent/data /opt/email-agent/workspace
chmod -R u+rwX /home/emailagent

# 7. Install hardened systemd service unit
echo "==> Installing hardened systemd service (/etc/systemd/system/email-agent.service)..."
cat <<'EOF' > /etc/systemd/system/email-agent.service
[Unit]
Description=Email AI Assistant Daemon
After=network.target

[Service]
Type=simple
User=emailagent
Group=emailagent
WorkingDirectory=/opt/email-agent
ExecStart=/opt/email-agent/email-agent
Restart=always
RestartSec=5
Environment="PATH=/usr/local/bin:/usr/bin:/bin"
Environment="AGY_BIN_PATH=/usr/local/bin/agy"
EnvironmentFile=/etc/email-agent/.env

# Linux namespace hardening & sandboxing
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=true
NoNewPrivileges=true
ProtectKernelTunables=true
ProtectControlGroups=true
ReadWritePaths=/opt/email-agent/data /opt/email-agent/workspace /home/emailagent

[Install]
WantedBy=multi-user.target
EOF

# 8. Reload systemd
systemctl daemon-reload

echo "============================================================"
echo " [SUCCESS] Email AI Assistant daemon setup complete!"
echo " Configuration: /etc/email-agent/.env (chmod 0640)"
echo " Workspace:     /opt/email-agent/workspace"
echo " Data:          /opt/email-agent/data"
echo ""
echo " To start daemon:"
echo "   sudo systemctl enable --now email-agent"
echo ""
echo " To view live logs:"
echo "   sudo journalctl -u email-agent -f"
echo "============================================================"
