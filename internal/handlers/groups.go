package handlers

import (
	"net/http"
	"strings"

	"github.com/buadamlaz/sambly/internal/auth"
	"github.com/buadamlaz/sambly/internal/samba"
	"github.com/buadamlaz/sambly/internal/security"
)

type groupsPageData struct {
	Groups []samba.Group
	Users  []string
}

func buildGroupsPageData() groupsPageData {
	groups, _ := samba.ListGroups()
	return groupsPageData{Groups: groups, Users: samba.ListUsernames()}
}

func (h *Handler) handleGroups(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}
	pd := PageData{
		Title:      "Groups — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "groups",
		Data:       buildGroupsPageData(),
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}
	h.render(w, "groups", pd)
}

func (h *Handler) handleGroupsAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	name := r.FormValue("name")
	if !security.ValidateGroupName(name) {
		h.groupsErr(w, sess, "Invalid group name. Only letters, numbers, underscore, hyphen. Max 32 chars.")
		return
	}
	ip := security.RealIP(r)
	if err := samba.CreateGroup(name); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_ADD_FAIL", name+": "+err.Error(), ip)
		h.groupsErr(w, sess, "Failed to create group: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "GROUP_ADD", "Created group: "+name, ip)
	http.Redirect(w, r, "/groups?flash=Group+created+successfully.", http.StatusSeeOther)
}

func (h *Handler) handleGroupsDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	name := r.FormValue("name")
	if !security.ValidateGroupName(name) {
		h.groupsErr(w, sess, "Invalid group name.")
		return
	}
	ip := security.RealIP(r)
	if err := samba.DeleteGroup(name); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_DELETE_FAIL", name+": "+err.Error(), ip)
		h.groupsErr(w, sess, "Failed to delete group: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "GROUP_DELETE", "Deleted group: "+name, ip)
	http.Redirect(w, r, "/groups?flash=Group+deleted+successfully.", http.StatusSeeOther)
}

// handleGroupsAssign adds a single user to a group (used from users page).
func (h *Handler) handleGroupsAssign(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	username := r.FormValue("username")
	groupName := r.FormValue("group")
	if !security.ValidateUsername(username) {
		h.groupsErr(w, sess, "Invalid username.")
		return
	}
	if !security.ValidateGroupName(groupName) {
		h.groupsErr(w, sess, "Invalid group name.")
		return
	}
	ip := security.RealIP(r)
	if err := samba.AddUserToGroup(username, groupName); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_ASSIGN_FAIL", username+"→"+groupName+": "+err.Error(), ip)
		h.groupsErr(w, sess, "Failed to assign user: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "GROUP_ASSIGN", username+" → "+groupName, ip)
	http.Redirect(w, r, "/users?flash=User+assigned+to+group.", http.StatusSeeOther)
}

// handleGroupsAssignMulti adds multiple users to a group.
// Accepts repeated "usernames" form fields (one per checkbox).
func (h *Handler) handleGroupsAssignMulti(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.groupsErr(w, sess, "Invalid form data.")
		return
	}

	groupName := r.FormValue("group")
	if !security.ValidateGroupName(groupName) {
		h.groupsErr(w, sess, "Invalid group name.")
		return
	}

	usernames := r.Form["usernames"]
	if len(usernames) == 0 {
		http.Redirect(w, r, "/groups?flash=No+users+selected.", http.StatusSeeOther)
		return
	}

	ip := security.RealIP(r)
	var added, failed []string
	for _, u := range usernames {
		if !security.ValidateUsername(u) {
			continue
		}
		if err := samba.AddUserToGroup(u, groupName); err != nil {
			failed = append(failed, u)
		} else {
			added = append(added, u)
		}
	}
	if len(added) > 0 {
		h.db.AddAuditLog(sess.Username, "GROUP_ASSIGN", strings.Join(added, ", ")+" → "+groupName, ip)
	}
	if len(failed) > 0 {
		h.groupsErr(w, sess, "Failed to add some users: "+strings.Join(failed, ", "))
		return
	}
	http.Redirect(w, r, "/groups?flash=Users+added+to+group.", http.StatusSeeOther)
}

func (h *Handler) handleGroupsRemoveMember(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	username := r.FormValue("username")
	groupName := r.FormValue("group")
	if !security.ValidateUsername(username) || !security.ValidateGroupName(groupName) {
		h.groupsErr(w, sess, "Invalid input.")
		return
	}
	ip := security.RealIP(r)
	if err := samba.RemoveUserFromGroup(username, groupName); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_REMOVE_FAIL", username+"←"+groupName+": "+err.Error(), ip)
		h.groupsErr(w, sess, "Failed to remove user: "+err.Error())
		return
	}
	h.db.AddAuditLog(sess.Username, "GROUP_REMOVE", username+" removed from "+groupName, ip)
	http.Redirect(w, r, "/groups?flash=User+removed+from+group.", http.StatusSeeOther)
}

func (h *Handler) groupsErr(w http.ResponseWriter, sess *auth.Session, msg string) {
	h.render(w, "groups", PageData{
		Title:      "Groups — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "groups",
		FlashErr:   msg,
		Data:       buildGroupsPageData(),
	})
}

func splitTrim(s string) []string {
	var result []string
	for _, p := range strings.Split(s, ",") {
		if v := strings.TrimSpace(p); v != "" {
			result = append(result, v)
		}
	}
	return result
}
