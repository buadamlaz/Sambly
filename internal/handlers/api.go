package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GET /api/dirs?path=/srv — returns subdirectory names as JSON array.
func (h *Handler) handleAPIDirs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireSession(r); !ok {
		jsonEmpty(w)
		return
	}

	raw := r.URL.Query().Get("path")
	if raw == "" {
		raw = "/"
	}

	cleaned := filepath.Clean(raw)
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/"
	}

	entries, err := os.ReadDir(cleaned)
	if err != nil {
		jsonEmpty(w)
		return
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	if dirs == nil {
		dirs = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dirs)
}

func jsonEmpty(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[]"))
}
