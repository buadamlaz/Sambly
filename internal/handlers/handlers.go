package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/buadamlaz/Sambly/internal/auth"
	"github.com/buadamlaz/Sambly/internal/db"
	"github.com/buadamlaz/Sambly/internal/security"
)

// Handler is the root handler that holds all dependencies.
type Handler struct {
	db          *db.DB
	auth        *auth.Manager
	sec         *security.Manager
	tmpl        *template.Template
	templateDir string
}

// New creates a new Handler.
func New(database *db.DB, authMgr *auth.Manager, secMgr *security.Manager, templateDir string, dataDir string) (*Handler, error) {
	tmpl, err := loadTemplates(templateDir)
	if err != nil {
		return nil, err
	}
	return &Handler{
		db:          database,
		auth:        authMgr,
		sec:         secMgr,
		tmpl:        tmpl,
		templateDir: templateDir,
	}, nil
}

func loadTemplates(dir string) (*template.Template, error) {
	pattern := filepath.Join(dir, "*.html")
	tmpl, err := template.New("").Funcs(templateFuncs()).ParseGlob(pattern)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"yesno": func(s string) string {
			switch s {
			case "yes", "true", "1":
				return "Yes"
			case "no", "false", "0":
				return "No"
			default:
				if s == "" {
					return "-"
				}
				return s
			}
		},
		"activeIf": func(page, current string) string {
			if page == current {
				return "active"
			}
			return ""
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
	}
}

// PageData is the common template data passed to all pages.
type PageData struct {
	Title      string
	Username   string
	CSRF       string
	Flash      string
	FlashErr   string
	ActivePage string // dashboard | users | groups | shares | settings | logs
	Data       interface{}
}

func (h *Handler) render(w http.ResponseWriter, name string, data PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("[ERROR] render template %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) requireSession(r *http.Request) (*auth.Session, bool) {
	sess, err := h.auth.GetSession(r)
	return sess, err == nil
}

func (h *Handler) redirectLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// requireAuth validates session and CSRF for POST handlers.
func (h *Handler) requireAuth(w http.ResponseWriter, r *http.Request) (*auth.Session, bool) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return nil, false
	}
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return nil, false
	}
	if !h.auth.ValidateCSRF(r, sess) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return nil, false
	}
	return sess, true
}

// RegisterRoutes wires up all HTTP routes to the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Static assets
	mux.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("web/static"))))

	// Auth
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/logout", h.handleLogout)

	// App routes
	mux.HandleFunc("/", h.handleRoot)
	mux.HandleFunc("/dashboard", h.handleDashboard)

	mux.HandleFunc("/users", h.handleUsers)
	mux.HandleFunc("/users/add", h.handleUsersAdd)
	mux.HandleFunc("/users/delete", h.handleUsersDelete)
	mux.HandleFunc("/users/password", h.handleUsersPassword)
	mux.HandleFunc("/users/toggle", h.handleUsersToggle)

	mux.HandleFunc("/groups", h.handleGroups)
	mux.HandleFunc("/groups/add", h.handleGroupsAdd)
	mux.HandleFunc("/groups/delete", h.handleGroupsDelete)
	mux.HandleFunc("/groups/assign", h.handleGroupsAssign)
	mux.HandleFunc("/groups/remove-member", h.handleGroupsRemoveMember)

	mux.HandleFunc("/shares", h.handleShares)
	mux.HandleFunc("/shares/add", h.handleSharesAdd)
	mux.HandleFunc("/shares/edit", h.handleSharesEdit)
	mux.HandleFunc("/shares/delete", h.handleSharesDelete)

	mux.HandleFunc("/settings", h.handleSettings)
	mux.HandleFunc("/settings/service", h.handleServiceAction)
	mux.HandleFunc("/settings/password", h.handleChangePassword)

	mux.HandleFunc("/logs", h.handleLogs)
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	_, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
