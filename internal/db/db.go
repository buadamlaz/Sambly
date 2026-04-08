package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS admin_users (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	username     TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	id          TEXT PRIMARY KEY,
	user_id     INTEGER NOT NULL,
	csrf_token  TEXT NOT NULL,
	ip_address  TEXT,
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
	expires_at  DATETIME NOT NULL,
	FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ip_bans (
	ip           TEXT PRIMARY KEY,
	attempts     INTEGER DEFAULT 0,
	banned_until DATETIME,
	last_attempt DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_log (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	username   TEXT,
	action     TEXT NOT NULL,
	details    TEXT,
	ip_address TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dsn := filepath.Join(dataDir, "sambly.db")
	sqldb, err := sql.Open("sqlite", dsn+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqldb.SetMaxOpenConns(1)

	if err := sqldb.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if _, err := sqldb.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	log.Printf("[INFO] Database opened: %s", dsn)
	return &DB{sqldb}, nil
}

func (d *DB) AdminUserExists() (bool, error) {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM admin_users").Scan(&count)
	return count > 0, err
}

func (d *DB) CreateAdminUser(username, hash string) error {
	_, err := d.Exec(
		"INSERT INTO admin_users (username, password_hash) VALUES (?, ?)",
		username, hash,
	)
	return err
}

func (d *DB) GetAdminUser(username string) (id int64, hash string, err error) {
	row := d.QueryRow(
		"SELECT id, password_hash FROM admin_users WHERE username = ?",
		username,
	)
	err = row.Scan(&id, &hash)
	return
}

func (d *DB) CreateSession(id string, userID int64, csrfToken, ip string, expiresAt string) error {
	_, err := d.Exec(
		`INSERT INTO sessions (id, user_id, csrf_token, ip_address, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, userID, csrfToken, ip, expiresAt,
	)
	return err
}

func (d *DB) GetSession(id string) (userID int64, csrfToken string, username string, err error) {
	row := d.QueryRow(
		`SELECT s.user_id, s.csrf_token, u.username
		 FROM sessions s
		 JOIN admin_users u ON u.id = s.user_id
		 WHERE s.id = ? AND s.expires_at > datetime('now')`,
		id,
	)
	err = row.Scan(&userID, &csrfToken, &username)
	return
}

func (d *DB) DeleteSession(id string) error {
	_, err := d.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

func (d *DB) CleanExpiredSessions() {
	d.Exec("DELETE FROM sessions WHERE expires_at <= datetime('now')")
}

func (d *DB) GetIPBan(ip string) (attempts int, bannedUntil string, err error) {
	row := d.QueryRow(
		"SELECT attempts, COALESCE(banned_until,'') FROM ip_bans WHERE ip = ?", ip,
	)
	err = row.Scan(&attempts, &bannedUntil)
	return
}

func (d *DB) RecordFailedAttempt(ip string) (int, error) {
	_, err := d.Exec(`
		INSERT INTO ip_bans (ip, attempts, last_attempt)
		VALUES (?, 1, datetime('now'))
		ON CONFLICT(ip) DO UPDATE SET
			attempts     = attempts + 1,
			last_attempt = datetime('now')
	`, ip)
	if err != nil {
		return 0, err
	}
	var attempts int
	d.QueryRow("SELECT attempts FROM ip_bans WHERE ip = ?", ip).Scan(&attempts)
	return attempts, nil
}

func (d *DB) BanIP(ip string, until string) error {
	_, err := d.Exec(
		"UPDATE ip_bans SET banned_until = ? WHERE ip = ?", until, ip,
	)
	return err
}

func (d *DB) ResetIPAttempts(ip string) error {
	_, err := d.Exec("DELETE FROM ip_bans WHERE ip = ?", ip)
	return err
}

func (d *DB) AddAuditLog(username, action, details, ip string) {
	_, err := d.Exec(
		`INSERT INTO audit_log (username, action, details, ip_address)
		 VALUES (?, ?, ?, ?)`,
		username, action, details, ip,
	)
	if err != nil {
		log.Printf("[WARN] audit log: %v", err)
	}
}

type AuditEntry struct {
	ID        int64
	Username  string
	Action    string
	Details   string
	IPAddress string
	CreatedAt string
}

func (d *DB) GetAuditLog(limit int) ([]AuditEntry, error) {
	rows, err := d.Query(
		`SELECT id, COALESCE(username,'system'), action, COALESCE(details,''),
		        COALESCE(ip_address,''), created_at
		 FROM audit_log ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Username, &e.Action, &e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) ChangeAdminPassword(userID int64, newHash string) error {
	_, err := d.Exec(
		"UPDATE admin_users SET password_hash = ? WHERE id = ?",
		newHash, userID,
	)
	return err
}
