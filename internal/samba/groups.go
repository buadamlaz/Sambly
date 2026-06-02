package samba

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Group struct {
	Name    string
	GID     string
	Members []string
}

func ListGroups() ([]Group, error) {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return nil, fmt.Errorf("read /etc/group: %w", err)
	}
	var groups []Group
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		gid, err := strconv.Atoi(parts[2])
		if err != nil || gid < 1000 || gid >= 65534 {
			continue
		}
		var members []string
		if parts[3] != "" {
			members = strings.Split(parts[3], ",")
		}
		groups = append(groups, Group{Name: parts[0], GID: parts[2], Members: members})
	}
	return groups, nil
}

func CreateGroup(name string) error {
	out, err := exec.Command("groupadd", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupadd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func DeleteGroup(name string) error {
	out, err := exec.Command("groupdel", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupdel: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func AddUserToGroup(username, groupName string) error {
	out, err := exec.Command("usermod", "-aG", groupName, username).CombinedOutput()
	if err != nil {
		return fmt.Errorf("usermod: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func RemoveUserFromGroup(username, groupName string) error {
	out, err := exec.Command("gpasswd", "-d", username, groupName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpasswd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
