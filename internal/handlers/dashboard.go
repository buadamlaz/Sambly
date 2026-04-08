package handlers

import (
	"net/http"

	"github.com/buadamlaz/Sambly/internal/samba"
	"github.com/buadamlaz/Sambly/internal/system"
)

type dashboardData struct {
	SambaInstalled bool
	ServiceStatus  system.ServiceStatus
	UserCount      int
	ShareCount     int
	GroupCount     int
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}

	users, _ := samba.ListUsers()
	shares, _ := samba.ListShares()
	groups, _ := samba.ListGroups()

	h.render(w, "dashboard.html", PageData{
		Title:      "Dashboard — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "dashboard",
		Data: dashboardData{
			SambaInstalled: system.IsSambaInstalled(),
			ServiceStatus:  system.GetSambaStatus(),
			UserCount:      len(users),
			ShareCount:     len(shares),
			GroupCount:     len(groups),
		},
	})
}
