package handlers

import (
	"embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/buadamlaz/sambly/internal/auth"
	"github.com/buadamlaz/sambly/internal/db"
	"github.com/buadamlaz/sambly/internal/security"
)

//go:embed tmpl/*.html
var tmplFS embed.FS

type Handler struct {
	db     *db.DB
	auth   *auth.Manager
	rl     *security.RateLimiter
	tmpls  map[string]*template.Template
}

type PageData struct {
	Title      string
	Username   string
	CSRF       string
	ActivePage string
	Flash      string
	FlashErr   string
	Data       any
}

func New(database *db.DB, authMgr *auth.Manager, rl *security.RateLimiter) (*Handler, error) {
	h := &Handler{
		db:    database,
		auth:  authMgr,
		rl:    rl,
		tmpls: make(map[string]*template.Template),
	}
	if err := h.loadTemplates(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Handler) loadTemplates() error {
	funcs := template.FuncMap{
		"join":     strings.Join,
		"contains": strings.Contains,
		"add":      func(a, b int) int { return a + b },
		"slice4": func(a, b, c, d string) []struct{ Name, Label, Class string } {
			return []struct{ Name, Label, Class string }{
				{a, "Start", "btn-success"},
				{b, "Stop", "btn-danger"},
				{c, "Restart", "btn-warn"},
				{d, "Reload Config", "btn-secondary"},
			}
		},
		"appVersion": func() string { return "1.0.0" },
		"logLevels": func() []struct{ Val, Label string } {
			return []struct{ Val, Label string }{
				{"", "— default —"},
				{"0", "0 — Errors only"},
				{"1", "1 — Warnings (recommended)"},
				{"2", "2 — Notice"},
				{"3", "3 — Info (verbose)"},
				{"5", "5 — Debug"},
				{"10", "10 — Everything"},
			}
		},
	}

	pages := []string{
		"login", "dashboard", "users", "shares", "shares_edit",
		"groups", "settings", "logs", "account",
	}

	for _, name := range pages {
		// ParseFS handles {{block}} overriding correctly across files —
		// unlike concatenating strings which causes "multiple definition" errors.
		t, err := template.New("").Funcs(funcs).ParseFS(
			tmplFS,
			"tmpl/base.html",
			"tmpl/"+name+".html",
		)
		if err != nil {
			return err
		}
		h.tmpls[name] = t
	}
	return nil
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	t, ok := h.tmpls[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) requireSession(r *http.Request) (*auth.Session, bool) {
	return h.auth.GetSession(r)
}

func (h *Handler) requireAuth(w http.ResponseWriter, r *http.Request) (*auth.Session, bool) {
	sess, ok := h.auth.GetSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return nil, false
	}
	if r.Method == http.MethodPost {
		csrf := r.FormValue("csrf_token")
		if csrf != sess.CSRFToken {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return nil, false
		}
	}
	return sess, true
}

func (h *Handler) redirectLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Auth
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/logout", h.handleLogout)

	// Pages
	mux.HandleFunc("/", h.handleRoot)
	mux.HandleFunc("/dashboard", h.handleDashboard)

	mux.HandleFunc("/users", h.handleUsers)
	mux.HandleFunc("/users/add", h.handleUsersAdd)
	mux.HandleFunc("/users/delete", h.handleUsersDelete)
	mux.HandleFunc("/users/password", h.handleUsersPassword)
	mux.HandleFunc("/users/toggle", h.handleUsersToggle)

	mux.HandleFunc("/shares", h.handleShares)
	mux.HandleFunc("/shares/add", h.handleSharesAdd)
	mux.HandleFunc("/shares/edit", h.handleSharesEdit)
	mux.HandleFunc("/shares/delete", h.handleSharesDelete)

	mux.HandleFunc("/groups", h.handleGroups)
	mux.HandleFunc("/groups/add", h.handleGroupsAdd)
	mux.HandleFunc("/groups/delete", h.handleGroupsDelete)
	mux.HandleFunc("/groups/assign", h.handleGroupsAssign)
	mux.HandleFunc("/groups/assign-multi", h.handleGroupsAssignMulti)
	mux.HandleFunc("/groups/remove", h.handleGroupsRemoveMember)

	mux.HandleFunc("/settings", h.handleSettings)
	mux.HandleFunc("/settings/service", h.handleServiceAction)
	mux.HandleFunc("/settings/password", h.handleChangePassword)
	mux.HandleFunc("/settings/validate", h.handleValidateConf)
	mux.HandleFunc("/settings/status", h.handleServiceStatus)
	mux.HandleFunc("/settings/smbconf", h.handleSmbConfSave)
	mux.HandleFunc("/settings/server", h.handleSettingsServer)
	mux.HandleFunc("/settings/access", h.handleSettingsAccess)
	mux.HandleFunc("/settings/logging", h.handleSettingsLogging)
	mux.HandleFunc("/settings/printers", h.handleSettingsPrinters)

	mux.HandleFunc("/api/dirs", h.handleAPIDirs)

	mux.HandleFunc("/logs", h.handleLogs)
	mux.HandleFunc("/account", h.handleAccount)
}

// ForcePasswordChange wraps a handler chain and redirects authenticated users
// who have never changed their default password to /account?force=1.
func (h *Handler) ForcePasswordChange(next http.Handler) http.Handler {
	// Paths that are always accessible regardless of password status
	allowed := map[string]bool{
		"/login":             true,
		"/logout":            true,
		"/account":           true,
		"/settings/password": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowed[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		sess, ok := h.auth.GetSession(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		changed, err := h.db.IsPasswordChanged(sess.UserID)
		if err == nil && !changed {
			http.Redirect(w, r, "/account?force=1", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
