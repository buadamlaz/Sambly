package handlers

import (
	"net/http"

	"github.com/buadamlaz/sambly/internal/auth"
	"github.com/buadamlaz/sambly/internal/samba"
	"github.com/buadamlaz/sambly/internal/security"
)

type userEntry struct {
	User     samba.SambaUser
	MemberOf []string
}

type usersPageData struct {
	Users     []userEntry
	AllGroups []samba.Group
}

func buildUsersPageData() usersPageData {
	users, _ := samba.ListUsers()
	groups, _ := samba.ListGroups()

	// Build username → group names map
	membership := make(map[string][]string)
	for _, g := range groups {
		for _, m := range g.Members {
			membership[m] = append(membership[m], g.Name)
		}
	}

	entries := make([]userEntry, len(users))
	for i, u := range users {
		entries[i] = userEntry{User: u, MemberOf: membership[u.Username]}
	}
	return usersPageData{Users: entries, AllGroups: groups}
}

func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}
	pd := PageData{
		Title:      "Users — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "users",
		Data:       buildUsersPageData(),
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}
	h.render(w, "users", pd)
}

func (h *Handler) handleUsersAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")
	fullName := sanitizeFullName(r.FormValue("full_name"))

	if !security.ValidateUsername(username) {
		h.usersErr(w, sess, "Invalid username. Only letters, numbers, underscore, hyphen, dot. Max 32 chars.")
		return
	}
	if len(password) < 6 {
		h.usersErr(w, sess, "Password must be at least 6 characters.")
		return
	}
	if password != confirm {
		h.usersErr(w, sess, "Passwords do not match.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.AddUser(username, password); err != nil {
		h.db.AddAuditLog(sess.Username, "USER_ADD_FAIL", username+": "+err.Error(), ip)
		h.usersErr(w, sess, "Failed to add user: "+err.Error())
		return
	}
	if fullName != "" {
		samba.SetFullName(username, fullName)
	}
	h.db.AddAuditLog(sess.Username, "USER_ADD", "Added samba user: "+username, ip)
	http.Redirect(w, r, "/users?flash=User+added+successfully.", http.StatusSeeOther)
}

func (h *Handler) handleUsersDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	username := r.FormValue("username")
	if !security.ValidateUsername(username) {
		h.usersErr(w, sess, "Invalid username.")
		return
	}
	ip := security.RealIP(r)
	if err := samba.DeleteUser(username); err != nil {
		h.db.AddAuditLog(sess.Username, "USER_DELETE_FAIL", username+": "+err.Error(), ip)
		h.usersErr(w, sess, "Failed to delete user: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "USER_DELETE", "Deleted samba user: "+username, ip)
	http.Redirect(w, r, "/users?flash=User+deleted+successfully.", http.StatusSeeOther)
}

func (h *Handler) handleUsersPassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if !security.ValidateUsername(username) {
		h.usersErr(w, sess, "Invalid username.")
		return
	}
	if len(password) < 6 {
		h.usersErr(w, sess, "Password must be at least 6 characters.")
		return
	}
	ip := security.RealIP(r)
	if err := samba.SetPassword(username, password); err != nil {
		h.db.AddAuditLog(sess.Username, "USER_PASSWD_FAIL", username+": "+err.Error(), ip)
		h.usersErr(w, sess, "Failed to set password: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "USER_PASSWD", "Changed password for: "+username, ip)
	http.Redirect(w, r, "/users?flash=Password+changed+successfully.", http.StatusSeeOther)
}

func (h *Handler) handleUsersToggle(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	username := r.FormValue("username")
	action := r.FormValue("action")

	if !security.ValidateUsername(username) {
		h.usersErr(w, sess, "Invalid username.")
		return
	}
	if action != "enable" && action != "disable" {
		h.usersErr(w, sess, "Invalid action.")
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
		h.usersErr(w, sess, "Failed to update user: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "USER_TOGGLE", action+" user: "+username, ip)
	http.Redirect(w, r, "/users?flash=User+updated+successfully.", http.StatusSeeOther)
}

func sanitizeFullName(s string) string {
	if len(s) > 64 {
		s = s[:64]
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != ':' && c != ',' && c != '\n' && c != '\r' {
			out = append(out, c)
		}
	}
	return string(out)
}

func (h *Handler) usersErr(w http.ResponseWriter, sess *auth.Session, msg string) {
	h.render(w, "users", PageData{
		Title:      "Users — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "users",
		FlashErr:   msg,
		Data:       buildUsersPageData(),
	})
}
