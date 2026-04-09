<div align="center">

<img src="assets/logo.png" alt="Sambly Logo" width="300" />

# Sambly — Samba Management, Simplified

[![GitHub](https://img.shields.io/github/license/buadamlaz/Sambly)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)

</div>

---

Sambly is an open-source, production-grade web GUI for managing a Samba server on Linux.
Clean, minimal, dark-mode, and secure by default.

## Features

- **User Management** — Add, delete, enable/disable Samba users. Change passwords securely (via stdin, never command arguments).
- **Group Management** — Create Unix groups, assign/remove users.
- **Share Management** — List, add, edit, delete shares in `smb.conf`. Automatic backup before every change.
- **Service Control** — Start, stop, restart, reload `smbd` via systemd.
- **Audit Log** — Every action is logged (who did what, from which IP).
- **Security-first** — Rate limiting, IP bans, CSRF protection, bcrypt passwords, secure session cookies.
- **Restart Warnings** — Prompts to restart Samba when configuration changes.
- **Auto-setup** — Generates secure admin credentials on first run.

> **⚠ IMPORTANT SECURITY WARNING**
>
> **THIS PROJECT IS NOT DESIGNED TO BE EXPOSED TO THE INTERNET.**
> Sambly is intended exclusively for **local network** or **private server** use.
> Running Sambly on a public-facing interface is a serious security risk.

---

## Screenshots

> _Screenshots coming soon_

## Installation

### Requirements

- Debian 12 / Ubuntu 22.04+ (other distros may work)
- Root access
- Internet access (for Go and Samba download)

### Quick Install

```bash
# Clone the repository
git clone https://github.com/buadamlaz/Sambly.git
cd Sambly

# Run the installer as root
sudo bash scripts/install.sh
```

The script will:

1. Check OS compatibility
2. Install Samba, SQLite, and Go (if not present)
3. Build the Sambly binary
4. Create a dedicated `sambly` system user
5. Install to `/opt/sambly`
6. Register and start `sambly.service` via systemd
7. Print access credentials

### First Login

After installation, retrieve your credentials:

```bash
journalctl -u sambly --no-pager | grep -A6 "CREDENTIALS"
```

Then open your browser:

```
http://<server-ip>:8090
```

> Change the default password immediately after first login in **Settings → Change Password**.

### Manual Build

```bash
# Install Go 1.21+
# https://go.dev/dl/

# Clone
git clone https://github.com/buadamlaz/Sambly.git
cd Sambly

# Download dependencies
go mod download

# Build
go build -o sambly ./cmd/server

# Run
./sambly -addr 0.0.0.0:8090 -data ./data -web ./web
```

## Security

Sambly is built with security as the top priority:

| Feature | Implementation |
|---------|---------------|
| Authentication | bcrypt (cost 12) password hashing |
| Sessions | Random 64-char hex ID, HTTPOnly + SameSite=Strict cookies |
| CSRF Protection | Per-session token validated on all POST requests |
| Rate Limiting | 5 failed login attempts → 15-minute IP ban |
| Command Injection | All shell commands use `exec.Command` with explicit args. **No `sh -c` with user input.** |
| Input Validation | Usernames, paths, share names, group names all validated with allowlists |
| Config Backups | smb.conf is backed up to `/var/lib/sambly/backups/` before every modification |
| Security Headers | X-Frame-Options, X-Content-Type-Options, CSP, Referrer-Policy |
| Bind Address | Default: `0.0.0.0:8090` (all interfaces) |

### ⚠ Security Warnings

- **Do not expose port 8090** to the public internet without a firewall rule.
- This tool manages Samba server configuration — treat access to it as equivalent to root access to Samba.
- Always use a strong, unique password for the admin account.
- Consider restricting access via firewall: `ufw allow from 192.168.1.0/24 to any port 8090`

## Usage Guide

### Adding a Samba User

1. Navigate to **Users**
2. Click **+ Add User**
3. Enter username and password (min 8 characters)
4. Click **Add User**

Note: A corresponding Linux system user is created automatically (with no login shell).

### Creating a Share

1. Navigate to **Shares**
2. Click **+ Add Share**
3. Fill in the share name and path (must exist on the filesystem)
4. Configure permissions (Valid Users, Write List, Read Only, etc.)
5. Click **Add Share**
6. When prompted, click **Restart Now** to apply changes to Samba

### Managing Groups

1. Navigate to **Groups**
2. Create a group with **+ Add Group**
3. Use **Assign User** to add users to groups
4. Reference groups in share permissions with `@groupname`

### Service Control

Navigate to **Settings** to start, stop, restart, or reload the Samba service.

## Project Structure

```
sambly/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── auth/auth.go            # Authentication, sessions, bcrypt
│   ├── db/db.go                # SQLite database layer
│   ├── handlers/               # HTTP route handlers
│   │   ├── handlers.go         # Router, middleware, PageData
│   │   ├── auth.go             # Login/logout
│   │   ├── dashboard.go        # Dashboard
│   │   ├── users.go            # Samba user management
│   │   ├── groups.go           # Group management
│   │   ├── shares.go           # Share management
│   │   ├── settings.go         # Settings & service control
│   │   └── logs.go             # Audit log view
│   ├── samba/
│   │   ├── users.go            # pdbedit, smbpasswd wrappers
│   │   ├── shares.go           # smb.conf parser & writer
│   │   └── groups.go           # groupadd, usermod wrappers
│   ├── security/security.go    # Rate limiting, CSRF, validation
│   └── system/service.go       # systemctl wrappers
├── web/
│   ├── templates/              # HTML templates (Go html/template)
│   └── static/                 # CSS, JS (no frameworks)
├── assets/                     # Logo and images
├── scripts/install.sh          # Installation script
├── sambly.service              # systemd service unit
├── go.mod
└── README.md
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go (standard library + `golang.org/x/crypto`) |
| Database | SQLite (`modernc.org/sqlite` — pure Go, no CGO) |
| Templates | `html/template` (server-side rendering) |
| Frontend | Vanilla CSS + minimal JS (no frameworks) |
| System | systemd, smbpasswd, pdbedit, useradd |

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Follow existing code style (gofmt, minimal dependencies)
4. Add security consideration notes for any new system interactions
5. Submit a pull request

### Development Setup

```bash
git clone https://github.com/buadamlaz/Sambly.git
cd Sambly
go mod download
go build ./...

# Run
./sambly -addr 0.0.0.0:8090 -data ./data -web ./web
```

### Reporting Issues

Please report security issues **privately** via GitHub Security Advisories, not public issues.

For bugs and feature requests, open a GitHub issue.

## Uninstallation

To remove Sambly from your system:

```bash
cd Sambly
sudo bash scripts/install.sh --uninstall
```

The uninstall process will:

1. Stop and disable the `sambly` systemd service
2. Remove the installed binary and files from `/opt/sambly`
3. Remove application data from `/var/lib/sambly`
4. Remove the `sambly` system user and group
5. Remove the sudoers rules from `/etc/sudoers.d/sambly`
6. Optionally remove Samba (`smbd`, `nmbd`) if you choose
7. Optionally restore or remove `smb.conf`

> Data removed during uninstall cannot be recovered. smb.conf backups are stored in `/var/lib/sambly/backups/` — copy them before uninstalling if needed.

## License

MIT License — see [LICENSE](LICENSE) for details.
