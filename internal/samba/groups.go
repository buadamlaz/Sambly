package samba

import (
	"fmt"
	"os/exec"
	"strings"
)

// Group represents a Unix group (used for Samba share permissions).
type Group struct {
	Name    string
	GID     string
	Members []string
}

// ListGroups returns all Unix groups that have members or are used in smb.conf.
// We list all system groups and their members via getent.
func ListGroups() ([]Group, error) {
	out, err := exec.Command("getent", "group").Output()
	if err != nil {
		return nil, fmt.Errorf("getent group: %w", err)
	}

	var groups []Group
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: groupname:x:gid:member1,member2,...
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		name := parts[0]
		gid := parts[2]
		memberStr := parts[3]

		// Filter out system groups (GID < 1000) unless they have samba users
		// We show groups with members or gid >= 1000
		var members []string
		if memberStr != "" {
			members = strings.Split(memberStr, ",")
		}

		groups = append(groups, Group{
			Name:    name,
			GID:     gid,
			Members: members,
		})
	}

	return groups, nil
}

// CreateGroup creates a new Unix group.
func CreateGroup(name string) error {
	out, err := exec.Command("sudo", "groupadd", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupadd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteGroup removes a Unix group.
func DeleteGroup(name string) error {
	out, err := exec.Command("sudo", "groupdel", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupdel: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// AddUserToGroup adds a user to a Unix group.
func AddUserToGroup(username, group string) error {
	out, err := exec.Command("sudo", "usermod", "-aG", group, username).CombinedOutput()
	if err != nil {
		return fmt.Errorf("usermod: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveUserFromGroup removes a user from a group using gpasswd.
func RemoveUserFromGroup(username, group string) error {
	out, err := exec.Command("sudo", "gpasswd", "-d", username, group).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpasswd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
