package handlers

import (
	"net/http"

	"github.com/buadamlaz/Sambly/internal/auth"
	"github.com/buadamlaz/Sambly/internal/security"
	"github.com/buadamlaz/Sambly/internal/system"
)

type settingsData struct {
	ServiceStatus system.ServiceStatus
	SambaInstalled bool
}

func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}

	pd := PageData{
		Title:      "Settings — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "settings",
		Data: settingsData{
			ServiceStatus:  system.GetSambaStatus(),
			SambaInstalled: system.IsSambaInstalled(),
		},
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}
	if e := r.URL.Query().Get("error"); e != "" {
		pd.FlashErr = e
	}

	h.render(w, "settings.html", pd)
}

func (h *Handler) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	action := r.FormValue("action")
	ip := security.RealIP(r)

	var err error
	switch action {
	case "start":
		err = system.StartSamba()
	case "stop":
		err = system.StopSamba()
	case "restart":
		err = system.RestartSamba()
	case "reload":
		err = system.ReloadSamba()
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	if err != nil {
		h.db.AddAuditLog(sess.Username, "SERVICE_"+action+"_FAIL", err.Error(), ip)
		http.Redirect(w, r, "/settings?error="+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	h.db.AddAuditLog(sess.Username, "SERVICE_"+action, "Samba service "+action, ip)
	http.Redirect(w, r, "/settings?flash=Service+"+action+"ed+successfully", http.StatusSeeOther)
}

func (h *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	current := r.FormValue("current_password")
	newPass := r.FormValue("new_password")
	confirm := r.FormValue("confirm_password")

	if newPass != confirm {
		h.settingsError(w, sess, "New passwords do not match.")
		return
	}
	if len(newPass) < 12 {
		h.settingsError(w, sess, "New password must be at least 12 characters.")
		return
	}

	// Verify current password
	_, hash, err := h.db.GetAdminUser(sess.Username)
	if err != nil || !auth.CheckPassword(hash, current) {
		h.settingsError(w, sess, "Current password is incorrect.")
		return
	}

	newHash, err := auth.HashPassword(newPass)
	if err != nil {
		h.settingsError(w, sess, "Failed to hash password: "+err.Error())
		return
	}

	if err := h.db.ChangeAdminPassword(sess.UserID, newHash); err != nil {
		h.settingsError(w, sess, "Failed to update password: "+err.Error())
		return
	}

	ip := security.RealIP(r)
	h.db.AddAuditLog(sess.Username, "ADMIN_PASSWD_CHANGE", "Admin password changed", ip)

	// Invalidate session and force re-login
	h.auth.Logout(r)
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login?flash=Password+changed.+Please+log+in+again.", http.StatusSeeOther)
}

func (h *Handler) settingsError(w http.ResponseWriter, sess *auth.Session, errMsg string) {
	h.render(w, "settings.html", PageData{
		Title:      "Settings — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "settings",
		FlashErr:   errMsg,
		Data: settingsData{
			ServiceStatus:  system.GetSambaStatus(),
			SambaInstalled: system.IsSambaInstalled(),
		},
	})
}

func urlEncode(s string) string {
	// Simple space-to-+ encoding for flash messages
	result := ""
	for _, c := range s {
		if c == ' ' {
			result += "+"
		} else {
			result += string(c)
		}
	}
	return result
}
