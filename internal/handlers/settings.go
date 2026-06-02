package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/buadamlaz/sambly/internal/auth"
	"github.com/buadamlaz/sambly/internal/samba"
	"github.com/buadamlaz/sambly/internal/security"
	"github.com/buadamlaz/sambly/internal/system"
)

const smbConfPath = "/etc/samba/smb.conf"

type settingsData struct {
	ServiceStatus  system.ServiceStatus
	SambaInstalled bool
	SmbConf        string
	Global         samba.GlobalSettings
	PrinterSharing bool
}

func buildSettingsData() settingsData {
	return settingsData{
		ServiceStatus:  system.GetSambaStatus(),
		SambaInstalled: system.IsSambaInstalled(),
		SmbConf:        readSmbConf(),
		Global:         samba.GetGlobalSettings(),
		PrinterSharing: samba.IsPrinterSharingEnabled(),
	}
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
		Data:       buildSettingsData(),
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}
	if e := r.URL.Query().Get("error"); e != "" {
		pd.FlashErr = e
	}
	h.render(w, "settings", pd)
}

// ─── Service actions ─────────────────────────────────────────────────────────

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
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.db.AddAuditLog(sess.Username, "SERVICE_"+action+"_FAIL", err.Error(), ip)
		http.Redirect(w, r, "/settings?error="+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}
	h.db.AddAuditLog(sess.Username, "SERVICE_"+action, "Samba service "+action, ip)
	http.Redirect(w, r, "/settings?flash=Service+"+action+"ed+successfully.", http.StatusSeeOther)
}

func (h *Handler) handleValidateConf(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireSession(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	output, _ := system.ValidateConf()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(output))
}

func (h *Handler) handleServiceStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireSession(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	st := system.GetSambaStatus()
	active := "false"
	if st.Active {
		active = "true"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"active":` + active + `}`))
}

// ─── Global settings saves ────────────────────────────────────────────────────

func (h *Handler) handleSettingsServer(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	updates := map[string]string{
		"workgroup":    sanitizeGlobalVal(r.FormValue("workgroup")),
		"server string": sanitizeGlobalVal(r.FormValue("server_string")),
		"netbios name": sanitizeGlobalVal(r.FormValue("netbios_name")),
	}
	if err := samba.UpdateGlobalSection(updates); err != nil {
		h.settingsErr(w, sess, "Failed to save server settings: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "SETTINGS_SERVER", "Server settings updated", security.RealIP(r))
	http.Redirect(w, r, "/settings?flash=Server+settings+saved.+Restart+Samba+to+apply.", http.StatusSeeOther)
}

func (h *Handler) handleSettingsAccess(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	updates := map[string]string{
		"security":     sanitizeGlobalVal(r.FormValue("security")),
		"map to guest": sanitizeGlobalVal(r.FormValue("map_to_guest")),
		"guest account": sanitizeGlobalVal(r.FormValue("guest_account")),
		"hosts allow":  sanitizeGlobalVal(r.FormValue("hosts_allow")),
		"hosts deny":   sanitizeGlobalVal(r.FormValue("hosts_deny")),
	}
	if err := samba.UpdateGlobalSection(updates); err != nil {
		h.settingsErr(w, sess, "Failed to save access settings: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "SETTINGS_ACCESS", "Access settings updated", security.RealIP(r))
	http.Redirect(w, r, "/settings?flash=Access+settings+saved.+Restart+Samba+to+apply.", http.StatusSeeOther)
}

func (h *Handler) handleSettingsLogging(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	updates := map[string]string{
		"log level":    sanitizeGlobalVal(r.FormValue("log_level")),
		"max log size": sanitizeGlobalVal(r.FormValue("max_log_size")),
	}
	if err := samba.UpdateGlobalSection(updates); err != nil {
		h.settingsErr(w, sess, "Failed to save logging settings: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "SETTINGS_LOGGING", "Logging settings updated", security.RealIP(r))
	http.Redirect(w, r, "/settings?flash=Logging+settings+saved.+Restart+Samba+to+apply.", http.StatusSeeOther)
}

func (h *Handler) handleSettingsPrinters(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	enable := r.FormValue("printer_sharing") == "yes"
	if err := samba.SetPrinterSharing(enable); err != nil {
		h.settingsErr(w, sess, "Failed to update printer sharing: "+err.Error())
		return
	}
	action := "disabled"
	if enable {
		action = "enabled"
	}
	h.db.AddAuditLog(sess.Username, "SETTINGS_PRINTERS", "Printer sharing "+action, security.RealIP(r))
	http.Redirect(w, r, "/settings?flash=Printer+sharing+"+action+".+Restart+Samba+to+apply.", http.StatusSeeOther)
}

// ─── Account page ─────────────────────────────────────────────────────────────

func (h *Handler) handleAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}
	pd := PageData{
		Title:    "Account — Sambly",
		Username: sess.Username,
		CSRF:     sess.CSRFToken,
	}
	if r.URL.Query().Get("force") == "1" {
		pd.Data = true // signals template to show the forced-change warning
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}
	if e := r.URL.Query().Get("error"); e != "" {
		pd.FlashErr = e
	}
	h.render(w, "account", pd)
}

// ─── Admin password ────────────────────────────────────────────────────────────

func (h *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	current := r.FormValue("current_password")
	newPass := r.FormValue("new_password")
	confirm := r.FormValue("confirm_password")

	if newPass != confirm {
		h.settingsErr(w, sess, "New passwords do not match.")
		return
	}
	if len(newPass) < 8 {
		h.settingsErr(w, sess, "New password must be at least 8 characters.")
		return
	}
	_, hash, err := h.db.GetAdminUser(sess.Username)
	if err != nil || !auth.CheckPassword(hash, current) {
		h.settingsErr(w, sess, "Current password is incorrect.")
		return
	}
	newHash, err := auth.HashPassword(newPass)
	if err != nil {
		h.settingsErr(w, sess, "Failed to hash password.")
		return
	}
	if err := h.db.ChangeAdminPassword(sess.UserID, newHash); err != nil {
		h.settingsErr(w, sess, "Failed to update password: "+err.Error())
		return
	}
	h.db.SetPasswordChanged(sess.UserID)
	h.db.AddAuditLog(sess.Username, "ADMIN_PASSWD_CHANGE", "Admin password changed", security.RealIP(r))
	h.auth.Logout(r)
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login?flash=Password+changed.+Please+log+in+again.", http.StatusSeeOther)
}

// ─── smb.conf editor ───────────────────────────────────────────────────────────

func (h *Handler) handleSmbConfSave(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	content := r.FormValue("smbconf")
	if strings.TrimSpace(content) == "" {
		h.settingsErr(w, sess, "smb.conf cannot be empty.")
		return
	}
	tmp, err := os.CreateTemp("", "smb.conf.validate.*")
	if err != nil {
		h.settingsErr(w, sess, "Failed to create temp file: "+err.Error())
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		h.settingsErr(w, sess, "Failed to write temp file: "+err.Error())
		return
	}
	tmp.Close()

	out, err := exec.Command("testparm", "-s", "--suppress-prompt", tmp.Name()).CombinedOutput()
	if err != nil {
		h.settingsErr(w, sess, fmt.Sprintf("smb.conf validation failed:\n%s", strings.TrimSpace(string(out))))
		return
	}
	existing, _ := os.ReadFile(smbConfPath)
	os.WriteFile(smbConfPath+".bak", existing, 0640)
	if err := os.WriteFile(smbConfPath, []byte(content), 0640); err != nil {
		h.settingsErr(w, sess, "Failed to save smb.conf: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "SMBCONF_EDIT", "smb.conf edited via web UI", security.RealIP(r))
	http.Redirect(w, r, "/settings?flash=smb.conf+saved.+Restart+Samba+to+apply.", http.StatusSeeOther)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func readSmbConf() string {
	data, err := os.ReadFile(smbConfPath)
	if err != nil {
		return "# Could not read " + smbConfPath + ": " + err.Error()
	}
	return string(data)
}

func (h *Handler) settingsErr(w http.ResponseWriter, sess *auth.Session, msg string) {
	h.render(w, "settings", PageData{
		Title:      "Settings — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "settings",
		FlashErr:   msg,
		Data:       buildSettingsData(),
	})
}

func sanitizeGlobalVal(s string) string {
	// Strip newlines and null bytes; trim whitespace
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\x00' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func urlEncode(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			out = append(out, '+')
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
}
