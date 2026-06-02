package handlers

import (
	"net/http"
	"time"

	"github.com/buadamlaz/sambly/internal/auth"
	"github.com/buadamlaz/sambly/internal/security"
)

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Already logged in → redirect
	if _, ok := h.auth.GetSession(r); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		flash := r.URL.Query().Get("flash")
		h.renderLogin(w, "", flash)
		return
	}

	ip := security.RealIP(r)
	allowed, wait := h.rl.Allow(ip)
	if !allowed {
		mins := int(wait.Minutes()) + 1
		h.renderLogin(w, "Too many failed attempts. Try again in "+itoa(mins)+" minute(s).", "")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	_, err := h.auth.LoginWithCookie(w, username, password)
	if err != nil {
		h.db.AddAuditLog(username, "LOGIN_FAIL", "Bad credentials", ip)
		h.renderLogin(w, "Invalid username or password.", "")
		return
	}

	h.rl.Reset(ip)
	h.db.AddAuditLog(username, "LOGIN", "Successful login", ip)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.auth.GetSession(r)
	if ok {
		h.db.AddAuditLog(sess.Username, "LOGOUT", "", security.RealIP(r))
		h.auth.Logout(r)
	}
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login?flash=You+have+been+logged+out.", http.StatusSeeOther)
}

func (h *Handler) renderLogin(w http.ResponseWriter, errMsg, flash string) {
	type loginData struct {
		Error string
		Flash string
		Year  int
	}
	h.render(w, "login", loginData{Error: errMsg, Flash: flash, Year: time.Now().Year()})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
