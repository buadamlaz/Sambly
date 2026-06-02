package handlers

import (
	"net/http"

	"github.com/buadamlaz/sambly/internal/auth"
	"github.com/buadamlaz/sambly/internal/samba"
	"github.com/buadamlaz/sambly/internal/security"
)

type sharesPageData struct {
	Shares []samba.Share
	Users  []string
	Groups []string // prefixed with @
}

func groupNames() []string {
	groups, _ := samba.ListGroups()
	// Skip groups that share a name with a user (Linux auto-creates a primary group per user).
	users := samba.ListUsernames()
	userSet := make(map[string]bool, len(users))
	for _, u := range users {
		userSet[u] = true
	}
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		if !userSet[g.Name] {
			names = append(names, "@"+g.Name)
		}
	}
	return names
}

func (h *Handler) handleShares(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}
	shares, err := samba.ListShares()
	pd := PageData{
		Title:      "Shares — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "shares",
		Data:       sharesPageData{Shares: shares, Users: samba.ListUsernames(), Groups: groupNames()},
	}
	if err != nil {
		pd.FlashErr = "Failed to read smb.conf: " + err.Error()
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}
	h.render(w, "shares", pd)
}

func (h *Handler) handleSharesAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	share, errMsg := shareFromForm(r)
	if errMsg != "" {
		h.sharesErr(w, sess, errMsg)
		return
	}
	ip := security.RealIP(r)
	if r.FormValue("create_dir") == "1" {
		owner := ""
		if share.ValidUsers != "" {
			// Use first valid user as owner
			for _, u := range splitUsers(share.ValidUsers) {
				owner = u
				break
			}
		}
		if err := samba.SetupShareDirectory(share.Path, owner); err != nil {
			h.db.AddAuditLog(sess.Username, "SHARE_MKDIR_FAIL", share.Path+": "+err.Error(), ip)
			h.sharesErr(w, sess, "Failed to create directory: "+err.Error())
			return
		}
		h.db.AddAuditLog(sess.Username, "SHARE_MKDIR", "Created dir: "+share.Path, ip)
	}
	if err := samba.AddShare(share); err != nil {
		h.db.AddAuditLog(sess.Username, "SHARE_ADD_FAIL", share.Name+": "+err.Error(), ip)
		h.sharesErr(w, sess, "Failed to add share: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "SHARE_ADD", "Added share: "+share.Name, ip)
	http.Redirect(w, r, "/shares?flash=Share+added.+Restart+Samba+to+apply.", http.StatusSeeOther)
}

func (h *Handler) handleSharesEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		sess, ok := h.requireSession(r)
		if !ok {
			h.redirectLogin(w, r)
			return
		}
		name := r.URL.Query().Get("name")
		if !security.ValidateShareName(name) {
			http.Error(w, "invalid share name", http.StatusBadRequest)
			return
		}
		share, err := samba.GetShare(name)
		if err != nil {
			h.sharesErr(w, sess, "Share not found: "+err.Error())
			return
		}
		h.render(w, "shares_edit", PageData{
			Title:      "Edit Share — Sambly",
			Username:   sess.Username,
			CSRF:       sess.CSRFToken,
			ActivePage: "shares",
			Data: struct {
				Share  *samba.Share
				Users  []string
				Groups []string
			}{share, samba.ListUsernames(), groupNames()},
		})
		return
	}

	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	originalName := r.FormValue("original_name")
	if !security.ValidateShareName(originalName) {
		h.sharesErr(w, sess, "Invalid original share name.")
		return
	}
	share, errMsg := shareFromForm(r)
	if errMsg != "" {
		h.sharesErr(w, sess, errMsg)
		return
	}
	ip := security.RealIP(r)
	if err := samba.EditShare(originalName, share); err != nil {
		h.db.AddAuditLog(sess.Username, "SHARE_EDIT_FAIL", share.Name+": "+err.Error(), ip)
		h.sharesErr(w, sess, "Failed to edit share: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "SHARE_EDIT", originalName+" → "+share.Name, ip)
	http.Redirect(w, r, "/shares?flash=Share+updated.+Restart+Samba+to+apply.", http.StatusSeeOther)
}

func (h *Handler) handleSharesDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	name := r.FormValue("name")
	if !security.ValidateShareName(name) {
		h.sharesErr(w, sess, "Invalid share name.")
		return
	}
	ip := security.RealIP(r)
	if err := samba.DeleteShare(name); err != nil {
		h.db.AddAuditLog(sess.Username, "SHARE_DELETE_FAIL", name+": "+err.Error(), ip)
		h.sharesErr(w, sess, "Failed to delete share: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "SHARE_DELETE", "Deleted share: "+name, ip)
	http.Redirect(w, r, "/shares?flash=Share+deleted.+Restart+Samba+to+apply.", http.StatusSeeOther)
}

func shareFromForm(r *http.Request) (samba.Share, string) {
	name := r.FormValue("name")
	path := r.FormValue("path")
	if !security.ValidateShareName(name) {
		return samba.Share{}, "Invalid share name: only letters, numbers, spaces, underscore, hyphen (max 64 chars)."
	}
	if !security.ValidatePath(path) {
		return samba.Share{}, "Invalid path: must be an absolute path without special characters."
	}
	return samba.Share{
		Name:          name,
		Path:          path,
		Comment:       sanitizeText(r.FormValue("comment"), 128),
		ValidUsers:    sanitizeUserList(r.FormValue("valid_users")),
		WriteList:     sanitizeUserList(r.FormValue("write_list")),
		ReadOnly:      sanitizeBool(r.FormValue("read_only")),
		Browseable:    sanitizeBool(r.FormValue("browseable")),
		GuestOK:       sanitizeBool(r.FormValue("guest_ok")),
		CreateMask:    sanitizeMask(r.FormValue("create_mask")),
		DirectoryMask: sanitizeMask(r.FormValue("directory_mask")),
		Raw:           make(map[string]string),
	}, ""
}

func (h *Handler) sharesErr(w http.ResponseWriter, sess *auth.Session, msg string) {
	shares, _ := samba.ListShares()
	h.render(w, "shares", PageData{
		Title:      "Shares — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "shares",
		FlashErr:   msg,
		Data:       sharesPageData{Shares: shares, Users: samba.ListUsernames(), Groups: groupNames()},
	})
}

func sanitizeText(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\n' && c != '\r' && c != ';' && c != '`' && c != '$' {
			out = append(out, c)
		}
	}
	return string(out)
}

func sanitizeUserList(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '.' || c == '@' || c == ',' || c == ' ' {
			out = append(out, c)
		}
	}
	return string(out)
}

func sanitizeBool(s string) string {
	if s == "yes" || s == "no" {
		return s
	}
	return "no"
}

func sanitizeMask(s string) string {
	if len(s) == 0 || len(s) > 4 {
		return ""
	}
	for _, c := range s {
		if c < '0' || c > '7' {
			return ""
		}
	}
	return s
}

func splitUsers(s string) []string {
	var result []string
	for _, u := range splitComma(s) {
		if u != "" {
			result = append(result, u)
		}
	}
	return result
}

func splitComma(s string) []string {
	var result []string
	cur := ""
	for _, c := range s {
		if c == ',' || c == ' ' {
			if cur != "" {
				result = append(result, cur)
				cur = ""
			}
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		result = append(result, cur)
	}
	return result
}
