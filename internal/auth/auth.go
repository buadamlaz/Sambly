package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/buadamlaz/sambly/internal/db"
)

const (
	sessionCookieName = "sambly_session"
	sessionTTL        = 24 * time.Hour
	tokenBytes        = 32
)

type Session struct {
	UserID    int64
	Username  string
	CSRFToken string
}

type Manager struct {
	db *db.DB
}

func NewManager(database *db.DB) *Manager {
	return &Manager{db: database}
}

func (m *Manager) Login(username, password string) (*Session, error) {
	id, hash, err := m.db.GetAdminUser(username)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, err
	}

	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, err
	}

	if err := m.db.CreateSession(token, username, csrf, id, sessionTTL); err != nil {
		return nil, err
	}
	return &Session{UserID: id, Username: username, CSRFToken: csrf}, nil
}

func (m *Manager) GetSession(r *http.Request) (*Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, false
	}
	row, err := m.db.GetSession(cookie.Value)
	if err != nil || time.Now().After(row.ExpiresAt) {
		return nil, false
	}
	return &Session{UserID: row.UserID, Username: row.Username, CSRFToken: row.CSRFToken}, true
}

func (m *Manager) Logout(r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	m.db.DeleteSession(cookie.Value)
}

func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GeneratePassword() (string, error) {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#%^&*"
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	result := make([]byte, 20)
	for i, v := range b {
		result[i] = chars[int(v)%len(chars)]
	}
	return string(result), nil
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// TokenForSession returns the raw cookie value for the current session.
// Used by Login handler to set the cookie after creating a session.
func (m *Manager) LoginWithCookie(w http.ResponseWriter, username, password string) (*Session, error) {
	id, hash, err := m.db.GetAdminUser(username)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, err
	}

	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, err
	}

	if err := m.db.CreateSession(token, username, csrf, id, sessionTTL); err != nil {
		return nil, err
	}

	SetSessionCookie(w, token)
	return &Session{UserID: id, Username: username, CSRFToken: csrf}, nil
}
