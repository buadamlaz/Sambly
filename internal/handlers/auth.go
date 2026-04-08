package handlers

import (
	"net/http"

	"github.com/buadamlaz/Sambly/internal/auth"
	"github.com/buadamlaz/Sambly/internal/security"
)

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Already logged in?
	if _, ok := h.requireSession(r); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		h.render(w, "login.html", PageData{Title: "Login — Sambly"})
		return
	}

	// POST: process login
	ip := security.RealIP(r)

	// Check IP ban
	if h.sec.IsBanned(ip) {
		h.render(w, "login.html", PageData{
			Title:    "Login — Sambly",
			FlashErr: "Too many failed attempts. Please wait 15 minutes before trying again.",
		})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	sessionID, err := h.auth.Login(username, password, ip)
	if err != nil {
		banned := h.sec.RecordFailure(ip)
		h.db.AddAuditLog(username, "LOGIN_FAILED", "Invalid credentials", ip)

		msg := "Invalid username or password."
		if banned {
			msg = "Too many failed attempts. Your IP has been temporarily blocked for 15 minutes."
		}
		h.render(w, "login.html", PageData{
			Title:    "Login — Sambly",
			FlashErr: msg,
		})
		return
	}

	h.sec.RecordSuccess(ip)
	h.db.AddAuditLog(username, "LOGIN_SUCCESS", "", ip)

	auth.SetSessionCookie(w, sessionID, false) // false = not HTTPS (local network use)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := h.requireSession(r); ok {
		ip := security.RealIP(r)
		h.db.AddAuditLog(sess.Username, "LOGOUT", "", ip)
	}
	h.auth.Logout(r)
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
