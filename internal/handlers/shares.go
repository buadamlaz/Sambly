package handlers

import (
	"fmt"
	"net/http"

	"github.com/buadamlaz/Sambly/internal/auth"
	"github.com/buadamlaz/Sambly/internal/samba"
	"github.com/buadamlaz/Sambly/internal/security"
)

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
		Data:       shares,
	}
	if err != nil {
		pd.FlashErr = "Failed to read smb.conf: " + err.Error()
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}

	h.render(w, "shares.html", pd)
}

func (h *Handler) handleSharesAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	share, err := shareFromForm(r)
	if err != nil {
		h.sharesError(w, sess, err.Error())
		return
	}

	ip := security.RealIP(r)
	if err := samba.AddShare(share); err != nil {
		h.db.AddAuditLog(sess.Username, "SHARE_ADD_FAIL", share.Name+": "+err.Error(), ip)
		h.sharesError(w, sess, "Failed to add share: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "SHARE_ADD", "Added share: "+share.Name+" path="+share.Path, ip)
	http.Redirect(w, r, "/shares?flash=Share+added.+Restart+Samba+to+apply+changes.", http.StatusSeeOther)
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
			http.Error(w, "Invalid share name", http.StatusBadRequest)
			return
		}
		share, err := samba.GetShare(name)
		if err != nil {
			h.sharesError(w, sess, "Share not found: "+err.Error())
			return
		}
		h.render(w, "shares_edit.html", PageData{
			Title:    "Edit Share — Sambly",
			Username: sess.Username,
			CSRF:     sess.CSRFToken,
			Data:     share,
		})
		return
	}

	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	originalName := r.FormValue("original_name")
	if !security.ValidateShareName(originalName) {
		h.sharesError(w, sess, "Invalid original share name.")
		return
	}

	share, err := shareFromForm(r)
	if err != nil {
		h.sharesError(w, sess, err.Error())
		return
	}

	ip := security.RealIP(r)
	if err := samba.EditShare(originalName, share); err != nil {
		h.db.AddAuditLog(sess.Username, "SHARE_EDIT_FAIL", share.Name+": "+err.Error(), ip)
		h.sharesError(w, sess, "Failed to edit share: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "SHARE_EDIT", "Edited share: "+originalName+" -> "+share.Name, ip)
	http.Redirect(w, r, "/shares?flash=Share+updated.+Restart+Samba+to+apply+changes.", http.StatusSeeOther)
}

func (h *Handler) handleSharesDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	name := r.FormValue("name")
	if !security.ValidateShareName(name) {
		h.sharesError(w, sess, "Invalid share name.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.DeleteShare(name); err != nil {
		h.db.AddAuditLog(sess.Username, "SHARE_DELETE_FAIL", name+": "+err.Error(), ip)
		h.sharesError(w, sess, "Failed to delete share: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "SHARE_DELETE", "Deleted share: "+name, ip)
	http.Redirect(w, r, "/shares?flash=Share+deleted.+Restart+Samba+to+apply+changes.", http.StatusSeeOther)
}

func shareFromForm(r *http.Request) (samba.Share, error) {
	name := r.FormValue("name")
	path := r.FormValue("path")

	if !security.ValidateShareName(name) {
		return samba.Share{}, fmt.Errorf("invalid share name: only letters, numbers, spaces, underscore, hyphen (max 64 chars)")
	}
	if !security.ValidatePath(path) {
		return samba.Share{}, fmt.Errorf("invalid path: must be an absolute path with no special characters")
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
	}, nil
}

func sanitizeText(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	safe := ""
	for _, c := range s {
		if c != '\n' && c != '\r' && c != '\t' && c != ';' && c != '`' && c != '$' {
			safe += string(c)
		}
	}
	return safe
}

func sanitizeUserList(s string) string {
	result := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '.' || c == '@' || c == ',' || c == ' ' {
			result += string(c)
		}
	}
	return result
}

func sanitizeBool(s string) string {
	if s == "yes" || s == "no" {
		return s
	}
	return "no"
}

func sanitizeMask(s string) string {
	if len(s) == 0 || len(s) > 5 {
		return ""
	}
	for _, c := range s {
		if c < '0' || c > '7' {
			return ""
		}
	}
	return s
}

func (h *Handler) sharesError(w http.ResponseWriter, sess *auth.Session, errMsg string) {
	shares, _ := samba.ListShares()
	h.render(w, "shares.html", PageData{
		Title:      "Shares — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "shares",
		FlashErr:   errMsg,
		Data:       shares,
	})
}
