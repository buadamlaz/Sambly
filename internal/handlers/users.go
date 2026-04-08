package handlers

import (
	"net/http"

	"github.com/buadamlaz/Sambly/internal/auth"
	"github.com/buadamlaz/Sambly/internal/samba"
	"github.com/buadamlaz/Sambly/internal/security"
)

// usersError re-renders the users page with an error message.
// Uses *auth.Session to populate username and CSRF token.

func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}

	users, err := samba.ListUsers()
	pd := PageData{
		Title:      "Users — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "users",
		Data:       users,
	}
	if err != nil {
		pd.FlashErr = "Failed to list users: " + err.Error()
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}

	h.render(w, "users.html", pd)
}

func (h *Handler) handleUsersAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if !security.ValidateUsername(username) {
		h.usersError(w, sess, "Invalid username. Only letters, numbers, underscore, hyphen, dot. Max 32 chars.")
		return
	}
	if len(password) < 8 {
		h.usersError(w, sess, "Password must be at least 8 characters.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.AddUser(username, password); err != nil {
		h.db.AddAuditLog(sess.Username, "USER_ADD_FAIL", username+": "+err.Error(), ip)
		h.usersError(w, sess, "Failed to add user: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "USER_ADD", "Added samba user: "+username, ip)
	http.Redirect(w, r, "/users?flash=User+added+successfully", http.StatusSeeOther)
}

func (h *Handler) handleUsersDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	username := r.FormValue("username")
	if !security.ValidateUsername(username) {
		h.usersError(w, sess, "Invalid username.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.DeleteUser(username); err != nil {
		h.db.AddAuditLog(sess.Username, "USER_DELETE_FAIL", username+": "+err.Error(), ip)
		h.usersError(w, sess, "Failed to delete user: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "USER_DELETE", "Deleted samba user: "+username, ip)
	http.Redirect(w, r, "/users?flash=User+deleted+successfully", http.StatusSeeOther)
}

func (h *Handler) handleUsersPassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if !security.ValidateUsername(username) {
		h.usersError(w, sess, "Invalid username.")
		return
	}
	if len(password) < 8 {
		h.usersError(w, sess, "Password must be at least 8 characters.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.SetPassword(username, password); err != nil {
		h.db.AddAuditLog(sess.Username, "USER_PASSWD_FAIL", username+": "+err.Error(), ip)
		h.usersError(w, sess, "Failed to set password: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "USER_PASSWD", "Changed password for: "+username, ip)
	http.Redirect(w, r, "/users?flash=Password+changed+successfully", http.StatusSeeOther)
}

func (h *Handler) handleUsersToggle(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	username := r.FormValue("username")
	action := r.FormValue("action") // "enable" or "disable"

	if !security.ValidateUsername(username) {
		h.usersError(w, sess, "Invalid username.")
		return
	}
	if action != "enable" && action != "disable" {
		h.usersError(w, sess, "Invalid action.")
		return
	}

	ip := security.RealIP(r)
	var err error
	if action == "enable" {
		err = samba.EnableUser(username)
	} else {
		err = samba.DisableUser(username)
	}

	if err != nil {
		h.db.AddAuditLog(sess.Username, "USER_TOGGLE_FAIL", username+": "+err.Error(), ip)
		h.usersError(w, sess, "Failed to toggle user: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "USER_TOGGLE", action+" user: "+username, ip)
	http.Redirect(w, r, "/users?flash=User+updated+successfully", http.StatusSeeOther)
}

func (h *Handler) usersError(w http.ResponseWriter, sess *auth.Session, errMsg string) {
	users, _ := samba.ListUsers()
	h.render(w, "users.html", PageData{
		Title:      "Users — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "users",
		FlashErr:   errMsg,
		Data:       users,
	})
}
