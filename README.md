<div align="center">

<img src="assets/logo.png" alt="Sambly Logo" width="300" />

# Sambly — Samba Management, Simplified!

[![GitHub License](https://img.shields.io/github/license/buadamlaz/Sambly)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/buadamlaz/Sambly)](https://github.com/buadamlaz/Sambly/releases)

</div>

---

Sambly is an open-source web GUI for managing a Samba/SMB server on Linux.
Single binary, zero runtime dependencies, installs in seconds.

## Features

**User Management**
- Add, delete, enable/disable SMB users with full name support
- Change passwords securely (passed via stdin, never as command-line arguments)
- View group membership per user directly in the users table
- Add users to groups from the Users page with one click

**Share Management**
- List, add, edit, delete shares directly in `smb.conf`
- Live directory browser — type a path and subdirectories appear instantly
- Multi-select dropdown for Valid Users and Write List (users + `@groups`)
- Visual permission picker — checkbox grid for Owner/Group/Public × Read/Write/Execute, shows octal and symbolic notation live
- Atomic writes with automatic `.bak` backup before every change
- One-click Restart Samba button on the success banner after any share change

**Group Management**
- Create and delete Unix groups
- Add multiple users at once via checkbox list
- Remove individual members directly from the groups table

**Settings & Configuration**
- **Server Identity** — Workgroup, server description, NetBIOS name
- **Access Control** — Security mode, map-to-guest, guest account, hosts allow/deny
- **Printer Sharing** — Toggle `[printers]` section on/off
- **Logging** — Log level and max log size
- **smb.conf Editor** — Raw editor with `testparm` validation and double-confirm protection; backup created automatically

**Service Control**
- Start, stop, restart, reload `smbd` via systemd
- Live service status badge in the sidebar
- Validate `smb.conf` with `testparm` directly from the UI

**Audit Log**
- Every action recorded: who did what, from which IP, and when
- Paginated (25 entries per page)

**Security**
- Rate limiting, CSRF protection, bcrypt password hashing, secure session cookies
- **Forced password change** — panel is locked until the auto-generated default password is changed
- All templates and assets embedded in the binary — no external files

---

> **⚠ IMPORTANT SECURITY WARNING**
>
> **THIS PROJECT IS NOT DESIGNED TO BE EXPOSED TO THE INTERNET.**
> Sambly is intended exclusively for **local network** or **private server** use.
> Running Sambly on a public-facing interface without a firewall is a serious security risk.

---

## Screenshots

> _Screenshots coming soon_

---

## Installation

### Requirements

- Linux (Debian/Ubuntu, RHEL/AlmaLinux, Arch)
- Root access
- Internet access (to download the binary)

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/buadamlaz/Sambly/main/scripts/install.sh | sudo bash
```

Or clone and run manually:

```bash
git clone https://github.com/buadamlaz/Sambly.git
cd Sambly
sudo bash scripts/install.sh
```

The installer will ask:

| Prompt | Default | Description |
|--------|---------|-------------|
| Listen port | `8090` | Port Sambly listens on |
| Admin username | `admin` | Panel login username |
| Admin password | *(random)* | Leave empty to auto-generate a secure password |

The script will then:

1. Detect your Linux distribution
2. Install Samba if not already present
3. Download the pre-built binary to `/usr/local/bin/sambly`
4. Register and start `sambly.service` via systemd
5. Print your access credentials

### First Login

Credentials are printed at the end of installation. You can also retrieve them:

```bash
cat /var/lib/sambly/initial-credentials.txt
```

Open your browser:

```
http://<server-ip>:8090
```

> **⚠ You will be forced to change the default password before accessing the panel.**
> After changing it, delete the credentials file:
> ```bash
> rm /var/lib/sambly/initial-credentials.txt
> ```

### Uninstall

```bash
sudo bash scripts/install.sh --uninstall
```

Stops and removes Sambly. Samba and its configuration are **not touched**.

### Manual Build

```bash
# Requires Go 1.22+  https://go.dev/dl/

git clone https://github.com/buadamlaz/Sambly.git
cd Sambly
go mod download
go build -o sambly ./cmd/server

# Must run as root (Samba system calls require it)
sudo ./sambly --addr 0.0.0.0:8090 --data ./data
```

---

## Security

| Feature | Implementation |
|---------|----------------|
| Authentication | bcrypt password hashing |
| Sessions | Cryptographically random token, HttpOnly + SameSite=Lax cookie, 24h TTL |
| CSRF Protection | Per-session token validated on every POST request |
| Rate Limiting | 5 failed login attempts → 15-minute IP block |
| Forced Password Change | Panel locked until default password is changed; timestamp recorded in DB |
| Command Injection | All system calls use `exec.Command` with explicit args — no `sh -c` with user input |
| Input Validation | Usernames, paths, share names, group names validated with strict allowlists |
| Config Backups | `smb.conf` backed up to `.bak` before every write |
| Security Headers | `X-Frame-Options`, `X-Content-Type-Options`, `CSP`, `Referrer-Policy` |

### ⚠ Security Warnings

- **Do not expose port 8090** to the public internet without a firewall rule.
- Sambly runs as root — treat access to it as equivalent to root access on the server.
- Always use a strong, unique password for the admin account.
- Restrict access by IP if possible:

```bash
ufw allow from 192.168.1.0/24 to any port 8090
```

---

## Usage Guide

### Users

1. Navigate to **Users**
2. Click **+ Add User**, enter username, optional full name, and password
3. The user is created as a Linux system account (`/usr/sbin/nologin`) and added to Samba
4. Use **+ Group** per row to assign the user to a group
5. Group membership is shown as badges in the Groups column

### Shares

1. Navigate to **Shares** → **+ Add Share**
2. Type a path — subdirectories appear as suggestions automatically
3. Select valid users/groups from the multi-select dropdown
4. Set file and directory permissions using the visual checkbox grid
5. After saving, click **↻ Restart Samba Now** in the success banner

### Groups

1. Navigate to **Groups** → **+ Add Group**
2. Click **+ Add Users** to open a checkbox list and assign multiple users at once
3. Use `@groupname` syntax in share Valid Users / Write List fields

### Settings

- **Server Identity** — set workgroup, server description, NetBIOS name
- **Access Control** — configure guest access policy and host restrictions
- **Printer Sharing** — enable or disable the `[printers]` section
- **Logging** — adjust verbosity and log file size
- **smb.conf Editor** — edit the raw config file; validated with `testparm` before saving
- **Service Control** — start, stop, restart, reload, validate config

### Account / Password

Click your **username** in the top-right corner to open the Change Password page.

---

## Project Structure

```
Sambly/
├── cmd/server/main.go          # Entry point, first-run setup, forced password check
├── internal/
│   ├── auth/auth.go            # Sessions, cookies, bcrypt
│   ├── db/db.go                # SQLite: users, sessions, audit log, password_changed_at
│   ├── security/security.go    # Rate limiting, CSRF, input validation, headers
│   ├── samba/
│   │   ├── users.go            # pdbedit, smbpasswd, useradd/userdel, full name
│   │   ├── shares.go           # smb.conf INI parser, global settings, printer sharing
│   │   └── groups.go           # groupadd, usermod, gpasswd wrappers
│   ├── system/service.go       # systemctl, testparm, version detection
│   └── handlers/
│       ├── handlers.go         # Router, go:embed, render, CSRF, ForcePasswordChange middleware
│       ├── api.go              # /api/dirs — live directory browser endpoint
│       ├── auth.go             # Login / logout
│       ├── dashboard.go        # Dashboard
│       ├── users.go            # SMB user management
│       ├── shares.go           # Share management
│       ├── groups.go           # Group management
│       ├── settings.go         # Settings, service control, smb.conf editor, account
│       ├── logs.go             # Audit log with pagination
│       └── tmpl/               # HTML templates (embedded in binary at compile time)
│           ├── base.html       # Layout, sidebar, topbar, footer partials + CSS
│           ├── login.html
│           ├── dashboard.html
│           ├── users.html
│           ├── shares.html / shares_edit.html
│           ├── groups.html
│           ├── settings.html
│           ├── logs.html
│           └── account.html    # Change password page
├── assets/                     # Logo (used in README)
├── scripts/install.sh          # Interactive install / uninstall
├── .github/workflows/
│   └── release.yml             # Auto-build binaries on version tag push
├── go.mod
└── README.md
```

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.22+ (standard library only, no frameworks) |
| Database | SQLite via `modernc.org/sqlite` — pure Go, no CGO |
| Templates | `html/template` + `go:embed` — single binary, no external files |
| Frontend | HTMX 2 + Alpine.js via CDN — no build step, no npm |
| System | systemd, `smbpasswd`, `pdbedit`, `useradd`, `groupadd`, `testparm` |

---

## Contributing

Contributions are welcome!

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Follow existing code style (`gofmt`, minimal dependencies, no frameworks)
4. Add security notes for any new system interactions
5. Submit a pull request

### Development Setup

```bash
git clone https://github.com/buadamlaz/Sambly.git
cd Sambly
go mod download
go build ./...

# Run locally on a Linux machine with Samba installed (root required)
sudo ./sambly --addr 0.0.0.0:8090 --data /tmp/sambly-dev
```

### Reporting Issues

Report security vulnerabilities **privately** via [GitHub Security Advisories](https://github.com/buadamlaz/Sambly/security/advisories/new).

For bugs and feature requests, open a [GitHub Issue](https://github.com/buadamlaz/Sambly/issues).

---

## License

MIT License — see [LICENSE](LICENSE) for details.
