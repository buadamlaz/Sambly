package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/buadamlaz/Sambly/internal/samba"
)

// handleAPIUsers returns a JSON array of Samba usernames.
// Used by the user-picker autocomplete in the shares form.
func (h *Handler) handleAPIUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireSession(r)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	users, _ := samba.ListUsers()
	names := make([]string, 0, len(users))
	for _, u := range users {
		// Escape quotes just in case (usernames are validated but be safe)
		names = append(names, `"`+strings.ReplaceAll(u.Username, `"`, `\"`)+`"`)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, "[%s]", strings.Join(names, ","))
}