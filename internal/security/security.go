package security

import (
	"net/http"
	"strings"
	"time"

	"github.com/buadamlaz/Sambly/internal/db"
)

const (
	MaxFailedAttempts = 5
	BanDuration       = 15 * time.Minute
)

type Manager struct {
	db *db.DB
}

func NewManager(database *db.DB) *Manager {
	return &Manager{db: database}
}

// RealIP extracts the real IP from a request, respecting X-Forwarded-For only
// for trusted proxies. Since Sambly binds to localhost, we just use RemoteAddr.
func RealIP(r *http.Request) string {
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// Strip brackets from IPv6
	ip = strings.TrimPrefix(ip, "[")
	ip = strings.TrimSuffix(ip, "]")
	return ip
}

// IsBanned checks if an IP is currently banned.
func (m *Manager) IsBanned(ip string) bool {
	_, bannedUntil, err := m.db.GetIPBan(ip)
	if err != nil {
		return false // no record = not banned
	}
	if bannedUntil == "" {
		return false
	}
	t, err := time.Parse("2006-01-02 15:04:05", bannedUntil)
	if err != nil {
		return false
	}
	return time.Now().UTC().Before(t)
}

// RecordFailure records a failed login attempt and bans the IP if threshold exceeded.
// Returns whether the IP is now banned.
func (m *Manager) RecordFailure(ip string) bool {
	attempts, err := m.db.RecordFailedAttempt(ip)
	if err != nil {
		return false
	}
	if attempts >= MaxFailedAttempts {
		until := time.Now().UTC().Add(BanDuration).Format("2006-01-02 15:04:05")
		m.db.BanIP(ip, until)
		return true
	}
	return false
}

// RecordSuccess clears failed attempts after a successful login.
func (m *Manager) RecordSuccess(ip string) {
	m.db.ResetIPAttempts(ip)
}

// ValidateUsername ensures usernames only contain safe characters.
// Samba usernames: alphanumeric, underscore, hyphen, dot. Max 32 chars.
func ValidateUsername(username string) bool {
	if username == "" || len(username) > 32 {
		return false
	}
	for _, c := range username {
		if !isAlphaNum(c) && c != '_' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

// ValidateShareName ensures share names are safe.
func ValidateShareName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !isAlphaNum(c) && c != '_' && c != '-' && c != ' ' {
			return false
		}
	}
	return true
}

// ValidatePath ensures a path looks like an absolute Linux path with no injection.
func ValidatePath(path string) bool {
	if path == "" || len(path) > 255 {
		return false
	}
	if !strings.HasPrefix(path, "/") {
		return false
	}
	// Reject shell metacharacters
	dangerous := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "!", "\\", "\n", "\r", "\t"}
	for _, d := range dangerous {
		if strings.Contains(path, d) {
			return false
		}
	}
	return true
}

// ValidateGroupName ensures group names are safe.
func ValidateGroupName(name string) bool {
	if name == "" || len(name) > 32 {
		return false
	}
	for _, c := range name {
		if !isAlphaNum(c) && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func isAlphaNum(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// SecurityHeaders sets secure HTTP headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}
