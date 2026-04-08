package handlers

import "net/http"

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}

	entries, err := h.db.GetAuditLog(200)
	pd := PageData{
		Title:      "Audit Log — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "logs",
		Data:       entries,
	}
	if err != nil {
		pd.FlashErr = "Failed to load audit log: " + err.Error()
	}

	h.render(w, "logs.html", pd)
}
