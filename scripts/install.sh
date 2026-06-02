#!/usr/bin/env bash
# Sambly install / uninstall script
# Usage: sudo bash install.sh [--uninstall]
# https://github.com/buadamlaz/Sambly

set -euo pipefail

REPO="https://github.com/buadamlaz/Sambly"
BINARY_URL="https://github.com/buadamlaz/Sambly/releases/latest/download/sambly-linux-amd64"
BINARY_PATH="/usr/local/bin/sambly"
DATA_DIR="/var/lib/sambly"
SERVICE_FILE="/etc/systemd/system/sambly.service"
GO_VERSION="1.22.5"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

info()    { echo -e "${CYAN}[*]${NC} $*"; }
success() { echo -e "${GREEN}[✓]${NC} $*"; }
warn()    { echo -e "${YELLOW}[!]${NC} $*"; }
die()     { echo -e "${RED}[✗]${NC} $*" >&2; exit 1; }

generate_password() {
    if command -v openssl &>/dev/null; then
        openssl rand -base64 32 | tr -dc 'a-zA-Z0-9!@#%^&*' | head -c 20
    else
        tr -dc 'a-zA-Z0-9!@#%^&*' < /dev/urandom | head -c 20
    fi
}

detect_pkg_manager() {
    if command -v apt-get &>/dev/null; then echo "apt"
    elif command -v dnf &>/dev/null; then echo "dnf"
    elif command -v yum &>/dev/null; then echo "yum"
    elif command -v pacman &>/dev/null; then echo "pacman"
    else echo "unknown"
    fi
}

# --- Root check ---
[ "$EUID" -eq 0 ] || die "This script must be run as root. Try: sudo bash install.sh"

# --- Uninstall ---
if [ "${1:-}" = "--uninstall" ]; then
    echo ""
    echo "  Sambly Uninstaller"
    echo "  ────────────────────────────────────────"
    echo ""

    # Helper: ask yes/no, default Y
    ask_yn() {
        local prompt="$1"
        local answer
        read -r -p "  $prompt [Y/n]: " answer </dev/tty || answer=""
        answer="${answer:-Y}"
        [[ "$answer" =~ ^[Yy]$ ]]
    }

    ask_yn "Remove Sambly binary, service and data?" || { info "Uninstall cancelled."; exit 0; }

    info "Stopping and removing Sambly service..."
    systemctl stop sambly    2>/dev/null || true
    systemctl disable sambly 2>/dev/null || true
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload  2>/dev/null || true
    rm -f "$BINARY_PATH"
    rm -rf "$DATA_DIR"
    success "Sambly removed."

    # Ask about Samba
    if command -v smbd &>/dev/null; then
        echo ""
        if ask_yn "Remove Samba (smbd/nmbd) packages?"; then
            PKG=$(detect_pkg_manager)
            case "$PKG" in
                apt)    apt-get remove -y samba samba-common ;;
                dnf)    dnf remove -y samba samba-common ;;
                yum)    yum remove -y samba samba-common ;;
                pacman) pacman -R --noconfirm samba ;;
                *)      warn "Cannot auto-remove Samba — unsupported package manager." ;;
            esac
            success "Samba packages removed."
        else
            info "Samba packages kept."
        fi
    fi

    # Ask about smb.conf
    if [ -f /etc/samba/smb.conf ]; then
        echo ""
        if ask_yn "Remove Samba configuration (/etc/samba/smb.conf and backups)?"; then
            rm -f /etc/samba/smb.conf /etc/samba/smb.conf.bak /etc/samba/smb.conf.tmp
            success "Samba configuration removed."
        else
            info "Samba configuration kept."
        fi
    fi

    # Ask about Go
    if [ -d /usr/local/go ]; then
        echo ""
        if ask_yn "Remove Go installation (/usr/local/go)?"; then
            rm -rf /usr/local/go
            success "Go removed."
        else
            info "Go installation kept."
        fi
    fi

    echo ""
    success "Uninstall complete."
    exit 0
fi

# --- Banner ---
echo ""
echo " ____                 _     _       "
echo "/ ___|  __ _ _ __ ___|_|__ | |_   _ "
echo "\___ \ / _' | '_ ' _ \| '_ \| | | | |"
echo " ___) | (_| | | | | | | |_) | | |_| |"
echo "|____/ \__,_|_| |_| |_|_.__/|_|\__, |"
echo "                                 |___/ "
echo "  Samba Web Manager — Installer"
echo ""

# --- Interactive setup ---
# When piped through curl, stdin is not a terminal so read returns empty → defaults apply.

echo -e "${BOLD}Configure Sambly:${NC}"
echo ""

read -r -p "  Listen port     [8090]  : " INPUT_PORT </dev/tty || INPUT_PORT=""
INPUT_PORT="${INPUT_PORT:-8090}"
if ! echo "$INPUT_PORT" | grep -qE '^[0-9]+$' || [ "$INPUT_PORT" -lt 1 ] || [ "$INPUT_PORT" -gt 65535 ]; then
    warn "Invalid port, using default: 8090"
    INPUT_PORT="8090"
fi

read -r -p "  Admin username  [admin]  : " INPUT_USER </dev/tty || INPUT_USER=""
INPUT_USER="${INPUT_USER:-admin}"
if ! echo "$INPUT_USER" | grep -qE '^[a-zA-Z0-9_\-\.]{1,32}$'; then
    warn "Invalid username, using default: admin"
    INPUT_USER="admin"
fi

read -r -s -p "  Admin password  [random] : " INPUT_PASS </dev/tty || INPUT_PASS=""
echo ""
if [ -z "$INPUT_PASS" ]; then
    INPUT_PASS=$(generate_password)
    info "No password entered — generated a random password."
fi

echo ""

# --- Install Samba if missing ---
PKG=$(detect_pkg_manager)

if ! command -v smbd &>/dev/null; then
    warn "Samba is not installed. Installing now..."
    case "$PKG" in
        apt)
            apt-get update -qq
            apt-get install -y samba
            ;;
        dnf)
            dnf install -y samba samba-common samba-client
            ;;
        yum)
            yum install -y samba samba-common samba-client
            ;;
        pacman)
            pacman -Sy --noconfirm samba
            ;;
        *)
            die "Unsupported package manager. Please install Samba manually and re-run this script."
            ;;
    esac
    systemctl enable smbd 2>/dev/null || true
    systemctl start  smbd 2>/dev/null || true
    success "Samba installed and started."
else
    success "Samba is already installed."
fi

# --- Download or build binary ---
info "Fetching Sambly binary..."

download_ok=false
if command -v curl &>/dev/null; then
    if curl -fsSL "$BINARY_URL" -o "$BINARY_PATH" 2>/dev/null; then
        download_ok=true
    fi
elif command -v wget &>/dev/null; then
    if wget -q "$BINARY_URL" -O "$BINARY_PATH" 2>/dev/null; then
        download_ok=true
    fi
fi

if $download_ok; then
    chmod +x "$BINARY_PATH"
    success "Binary downloaded to $BINARY_PATH"
else
    warn "No pre-built release found. Building from source..."

    if ! command -v go &>/dev/null && [ ! -x /usr/local/go/bin/go ]; then
        info "Installing Go ${GO_VERSION}..."
        ARCH=$(uname -m)
        case "$ARCH" in
            x86_64)  GOARCH="amd64" ;;
            aarch64) GOARCH="arm64" ;;
            armv6l)  GOARCH="armv6l" ;;
            *)       die "Unsupported architecture: $ARCH" ;;
        esac
        GO_TAR="go${GO_VERSION}.linux-${GOARCH}.tar.gz"
        TMP_DIR=$(mktemp -d)
        curl -fsSL "https://go.dev/dl/${GO_TAR}" -o "${TMP_DIR}/${GO_TAR}" || die "Failed to download Go."
        rm -rf /usr/local/go
        tar -C /usr/local -xzf "${TMP_DIR}/${GO_TAR}"
        rm -rf "$TMP_DIR"
        success "Go ${GO_VERSION} installed."
    fi

    export PATH="/usr/local/go/bin:$PATH"

    BUILD_DIR=$(mktemp -d)
    info "Cloning repository..."
    git clone --depth=1 "$REPO" "$BUILD_DIR" || die "Failed to clone repository."
    cd "$BUILD_DIR"
    info "Building Sambly (this may take a minute)..."
    go build -ldflags="-s -w" -o "$BINARY_PATH" ./cmd/server || die "Build failed."
    cd /
    rm -rf "$BUILD_DIR"
    chmod +x "$BINARY_PATH"
    success "Binary built and installed at $BINARY_PATH"
fi

# --- Create data directory and write setup config ---
mkdir -p "$DATA_DIR"
chmod 750 "$DATA_DIR"

cat > "${DATA_DIR}/setup.env" <<ENVEOF
ADMIN_USERNAME=${INPUT_USER}
ADMIN_PASSWORD=${INPUT_PASS}
ENVEOF
chmod 600 "${DATA_DIR}/setup.env"

# --- Create systemd service ---
info "Creating systemd service..."
cat > "$SERVICE_FILE" <<SVCEOF
[Unit]
Description=Sambly - Samba Web Manager
After=network.target smbd.service
Wants=smbd.service

[Service]
Type=simple
User=root
ExecStart=${BINARY_PATH} --addr=0.0.0.0:${INPUT_PORT} --data=${DATA_DIR}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF

systemctl daemon-reload
systemctl enable sambly
systemctl start sambly
success "Sambly service started."

# --- Wait for initialization ---
info "Waiting for service to initialize..."
for i in $(seq 1 15); do
    if [ -f "${DATA_DIR}/initial-credentials.txt" ]; then
        break
    fi
    sleep 1
done

# --- Display result ---
SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")

echo ""
echo "╔══════════════════════════════════════════════════╗"
echo "║         SAMBLY INSTALLED SUCCESSFULLY           ║"
echo "╠══════════════════════════════════════════════════╣"
printf  "║  URL      : http://%-30s║\n" "${SERVER_IP}:${INPUT_PORT}"
printf  "║  Username : %-37s║\n" "$INPUT_USER"
printf  "║  Password : %-37s║\n" "$INPUT_PASS"
echo "╠══════════════════════════════════════════════════╣"
echo "║  ⚠  Change your password after first login!    ║"
echo "╚══════════════════════════════════════════════════╝"
echo ""
warn "Credentials also saved at: ${DATA_DIR}/initial-credentials.txt"
warn "Delete this file after you have logged in."
echo ""
info "Manage : systemctl [start|stop|restart|status] sambly"
info "Logs   : journalctl -u sambly -f"
echo ""
