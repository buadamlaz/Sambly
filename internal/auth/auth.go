package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/buadamlaz/Sambly/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "sambly_session"
	SessionDuration   = 8 * time.Hour
	BcryptCost        = 12
)

type Manager struct {
	db *db.DB
}

func NewManager(database *db.DB) *Manager {
	return &Manager{db: database}
}

// HashPassword returns a bcrypt hash of the plain-text password.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword validates plain against the stored hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// GenerateRandom returns n random hex bytes as a string.
func GenerateRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GeneratePassword creates a secure random password for the initial admin.
func GeneratePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Login validates credentials and creates a session. Returns session ID on success.
func (m *Manager) Login(username, password, ip string) (string, error) {
	userID, hash, err := m.db.GetAdminUser(username)
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	if !CheckPassword(hash, password) {
		return "", fmt.Errorf("invalid credentials")
	}

	sessionID, err := GenerateRandom(32)
	if err != nil {
		return "", err
	}
	csrfToken, err := GenerateRandom(32)
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().UTC().Add(SessionDuration).Format("2006-01-02 15:04:05")
	if err := m.db.CreateSession(sessionID, userID, csrfToken, ip, expiresAt); err != nil {
		return "", err
	}

	return sessionID, nil
}

// Logout deletes the session.
func (m *Manager) Logout(r *http.Request) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return
	}
	m.db.DeleteSession(cookie.Value)
}

// Session holds data about the current authenticated session.
type Session struct {
	UserID    int64
	Username  string
	CSRFToken string
}

// GetSession retrieves and validates the session from the request.
func (m *Manager) GetSession(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}

	userID, csrfToken, username, err := m.db.GetSession(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	return &Session{
		UserID:    userID,
		Username:  username,
		CSRFToken: csrfToken,
	}, nil
}

// ValidateCSRF checks that the CSRF token in the form matches the session token.
func (m *Manager) ValidateCSRF(r *http.Request, session *Session) bool {
	token := r.FormValue("csrf_token")
	return token != "" && token == session.CSRFToken
}

// SetSessionCookie sets the session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, sessionID string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(SessionDuration.Seconds()),
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
