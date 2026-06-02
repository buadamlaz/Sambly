package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, "sambly.db")
	conn, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			username            TEXT UNIQUE NOT NULL,
			password            TEXT NOT NULL,
			created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
			password_changed_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL,
			username   TEXT NOT NULL,
			csrf_token TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS audit_log (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			actor      TEXT NOT NULL,
			action     TEXT NOT NULL,
			detail     TEXT,
			ip         TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return err
	}
	// Idempotent migration for existing DBs without the column
	d.conn.Exec(`ALTER TABLE users ADD COLUMN password_changed_at DATETIME`)
	return nil
}

// --- Admin user ---

func (d *DB) AdminUserExists() (bool, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0, err
}

func (d *DB) CreateAdminUser(username, hashedPassword string) error {
	_, err := d.conn.Exec(
		"INSERT INTO users (username, password) VALUES (?, ?)",
		username, hashedPassword,
	)
	return err
}

func (d *DB) GetAdminUser(username string) (id int64, hash string, err error) {
	err = d.conn.QueryRow(
		"SELECT id, password FROM users WHERE username = ?", username,
	).Scan(&id, &hash)
	return
}

func (d *DB) ChangeAdminPassword(userID int64, newHash string) error {
	_, err := d.conn.Exec("UPDATE users SET password = ? WHERE id = ?", newHash, userID)
	return err
}

// IsPasswordChanged returns true when the user has changed their default password at least once.
func (d *DB) IsPasswordChanged(userID int64) (bool, error) {
	var t sql.NullTime
	err := d.conn.QueryRow("SELECT password_changed_at FROM users WHERE id = ?", userID).Scan(&t)
	return t.Valid, err
}

// SetPasswordChanged records the timestamp of a password change.
func (d *DB) SetPasswordChanged(userID int64) error {
	_, err := d.conn.Exec("UPDATE users SET password_changed_at = ? WHERE id = ?", time.Now(), userID)
	return err
}

// --- Sessions ---

func (d *DB) CreateSession(token, username, csrfToken string, userID int64, ttl time.Duration) error {
	expires := time.Now().Add(ttl)
	_, err := d.conn.Exec(
		"INSERT INTO sessions (token, user_id, username, csrf_token, expires_at) VALUES (?, ?, ?, ?, ?)",
		token, userID, username, csrfToken, expires,
	)
	return err
}

type SessionRow struct {
	UserID    int64
	Username  string
	CSRFToken string
	ExpiresAt time.Time
}

func (d *DB) GetSession(token string) (*SessionRow, error) {
	row := &SessionRow{}
	err := d.conn.QueryRow(
		"SELECT user_id, username, csrf_token, expires_at FROM sessions WHERE token = ?", token,
	).Scan(&row.UserID, &row.Username, &row.CSRFToken, &row.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (d *DB) DeleteSession(token string) error {
	_, err := d.conn.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (d *DB) CleanExpiredSessions() {
	d.conn.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now())
}

// --- Audit log ---

func (d *DB) AddAuditLog(actor, action, detail, ip string) {
	d.conn.Exec(
		"INSERT INTO audit_log (actor, action, detail, ip) VALUES (?, ?, ?, ?)",
		actor, action, detail, ip,
	)
}

type AuditEntry struct {
	ID        int64
	Actor     string
	Action    string
	Detail    string
	IP        string
	CreatedAt time.Time
}

func (d *DB) GetAuditLogs(limit int) ([]AuditEntry, error) {
	rows, err := d.conn.Query(
		"SELECT id, actor, action, detail, ip, created_at FROM audit_log ORDER BY id DESC LIMIT ?", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Detail, &e.IP, &e.CreatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (d *DB) GetAuditLogsPaged(limit, offset int) ([]AuditEntry, error) {
	rows, err := d.conn.Query(
		"SELECT id, actor, action, detail, ip, created_at FROM audit_log ORDER BY id DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Detail, &e.IP, &e.CreatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (d *DB) GetAuditLogsCount() (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&count)
	return count, err
}
