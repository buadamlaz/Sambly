package handlers

import (
	"net/http"

	"github.com/buadamlaz/Sambly/internal/auth"
	"github.com/buadamlaz/Sambly/internal/samba"
	"github.com/buadamlaz/Sambly/internal/security"
)

func (h *Handler) handleGroups(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}

	groups, err := samba.ListGroups()
	pd := PageData{
		Title:      "Groups — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "groups",
		Data:       groups,
	}
	if err != nil {
		pd.FlashErr = "Failed to list groups: " + err.Error()
	}
	if f := r.URL.Query().Get("flash"); f != "" {
		pd.Flash = f
	}

	h.render(w, "groups.html", pd)
}

func (h *Handler) handleGroupsAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	name := r.FormValue("name")
	if !security.ValidateGroupName(name) {
		h.groupsError(w, sess, "Invalid group name. Only letters, numbers, underscore, hyphen. Max 32 chars.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.CreateGroup(name); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_ADD_FAIL", name+": "+err.Error(), ip)
		h.groupsError(w, sess, "Failed to create group: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "GROUP_ADD", "Created group: "+name, ip)
	http.Redirect(w, r, "/groups?flash=Group+created+successfully", http.StatusSeeOther)
}

func (h *Handler) handleGroupsDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	name := r.FormValue("name")
	if !security.ValidateGroupName(name) {
		h.groupsError(w, sess, "Invalid group name.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.DeleteGroup(name); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_DELETE_FAIL", name+": "+err.Error(), ip)
		h.groupsError(w, sess, "Failed to delete group: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "GROUP_DELETE", "Deleted group: "+name, ip)
	http.Redirect(w, r, "/groups?flash=Group+deleted+successfully", http.StatusSeeOther)
}

func (h *Handler) handleGroupsAssign(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	username := r.FormValue("username")
	groupName := r.FormValue("group")

	if !security.ValidateUsername(username) {
		h.groupsError(w, sess, "Invalid username.")
		return
	}
	if !security.ValidateGroupName(groupName) {
		h.groupsError(w, sess, "Invalid group name.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.AddUserToGroup(username, groupName); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_ASSIGN_FAIL", username+"->"+groupName+": "+err.Error(), ip)
		h.groupsError(w, sess, "Failed to assign user to group: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "GROUP_ASSIGN", username+" -> "+groupName, ip)
	http.Redirect(w, r, "/groups?flash=User+assigned+to+group", http.StatusSeeOther)
}

func (h *Handler) handleGroupsRemoveMember(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	username := r.FormValue("username")
	groupName := r.FormValue("group")

	if !security.ValidateUsername(username) || !security.ValidateGroupName(groupName) {
		h.groupsError(w, sess, "Invalid input.")
		return
	}

	ip := security.RealIP(r)
	if err := samba.RemoveUserFromGroup(username, groupName); err != nil {
		h.db.AddAuditLog(sess.Username, "GROUP_REMOVE_FAIL", username+"<-"+groupName+": "+err.Error(), ip)
		h.groupsError(w, sess, "Failed to remove user from group: "+err.Error())
		return
	}

	h.db.AddAuditLog(sess.Username, "GROUP_REMOVE", username+" removed from "+groupName, ip)
	http.Redirect(w, r, "/groups?flash=User+removed+from+group", http.StatusSeeOther)
}

func (h *Handler) groupsError(w http.ResponseWriter, sess *auth.Session, errMsg string) {
	groups, _ := samba.ListGroups()
	h.render(w, "groups.html", PageData{
		Title:      "Groups — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "groups",
		FlashErr:   errMsg,
		Data:       groups,
	})
}
