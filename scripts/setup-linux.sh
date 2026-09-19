#!/usr/bin/env bash
# scripts/setup-linux.sh - Automated Linux host provisioning for email-agent daemon
set -euo pipefail

if [ "${EUID}" -ne 0 ]; then
  echo "[ERROR] This script must be run as root (use sudo)." >&2
  exit 1
fi

echo "==> Setting up Email AI Assistant on Linux..."

# 1. Check dependencies
for cmd in curl python3; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[WARN] Optional or required tool '$cmd' not found in PATH." >&2
  fi
done

# 2. Check Antigravity CLI
if ! command -v agy >/dev/null 2>&1; then
  echo "[INFO] 'agy' CLI not found in root PATH."
  echo "      Ensure Antigravity is installed for the user: curl -fsSL https://antigravity.google/cli/install.sh | bash"
fi

# 3. Compile static binary if missing
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [ ! -f "${SCRIPT_DIR}/bin/email-agent" ]; then
  echo "==> Compiling static Linux binary..."
  if command -v go >/dev/null 2>&1; then
    (cd "${SCRIPT_DIR}" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/email-agent ./cmd/email-agent)
  else
    echo "[ERROR] 'bin/email-agent' not found and 'go' is not installed. Compile on your dev machine first (make build-linux)." >&2
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

# 5. Provision directories
echo "==> Provisioning directories in /opt/email-agent and /etc/email-agent..."
mkdir -p /opt/email-agent/data
mkdir -p /opt/email-agent/workspace
mkdir -p /etc/email-agent

cp "${SCRIPT_DIR}/bin/email-agent" /opt/email-agent/email-agent
chmod 0755 /opt/email-agent/email-agent

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

# 6. Install hardened systemd service unit
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
EnvironmentFile=/etc/email-agent/.env

# Linux namespace hardening & sandboxing
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ProtectKernelTunables=true
ProtectControlGroups=true
ReadWritePaths=/opt/email-agent/data /opt/email-agent/workspace /home/emailagent

[Install]
WantedBy=multi-user.target
EOF

# 7. Reload systemd
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
