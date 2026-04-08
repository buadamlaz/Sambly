package handlers

import (
	"net/http"

	"github.com/buadamlaz/Sambly/internal/db"
	"github.com/buadamlaz/Sambly/internal/system"
)

type logsData struct {
	Audit       []db.AuditEntry
	SambaLog    []system.SambaLogEntry
	SambaLogPath string
	SambaLogErr  string
	ActiveTab   string // "audit" | "samba"
}

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}

	tab := r.URL.Query().Get("tab")
	if tab != "samba" {
		tab = "audit"
	}

	// --- Sambly audit log ---
	audit, err := h.db.GetAuditLog(300)
	flashErr := ""
	if err != nil {
		flashErr = "Failed to load audit log: " + err.Error()
	}

	// --- Samba server log ---
	logPath := system.FindSambaLog()
	var sambaEntries []system.SambaLogEntry
	sambaLogErr := ""

	if logPath == "" {
		sambaLogErr = "Samba log file not found. Samba may not be installed or has not written any logs yet."
	} else {
		sambaEntries, err = system.ReadSambaLog(logPath, 300)
		if err != nil {
			sambaLogErr = "Failed to read Samba log (" + logPath + "): " + err.Error()
		}
	}

	h.render(w, "logs.html", PageData{
		Title:      "Logs — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "logs",
		FlashErr:   flashErr,
		Data: logsData{
			Audit:        audit,
			SambaLog:     sambaEntries,
			SambaLogPath: logPath,
			SambaLogErr:  sambaLogErr,
			ActiveTab:    tab,
		},
	})
}
