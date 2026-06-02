package samba

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type SambaUser struct {
	Username string
	UID      string
	FullName string
	Disabled bool
}

// ListUsers returns Samba users via pdbedit.
// Falls back to /etc/passwd only when pdbedit itself fails (not installed, etc.).
func ListUsers() ([]SambaUser, error) {
	out, err := exec.Command("pdbedit", "-L", "-v").Output()
	if err == nil {
		users := parsePdbedit(string(out))
		fillMissingUIDs(users)
		return users, nil
	}
	return listFromPasswd()
}

// fillMissingUIDs reads /etc/passwd to fill UID fields that pdbedit didn't return.
func fillMissingUIDs(users []SambaUser) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return
	}
	uidMap := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 {
			uidMap[parts[0]] = parts[2]
		}
	}
	for i := range users {
		if users[i].UID == "" {
			users[i].UID = uidMap[users[i].Username]
		}
	}
}

// ListUsernames returns plain usernames (UID >= 1000) for dropdowns.
func ListUsernames() []string {
	users, _ := listFromPasswd()
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Username)
	}
	return names
}

func listFromPasswd() ([]SambaUser, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	var users []SambaUser
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		uid, err := strconv.Atoi(parts[2])
		if err != nil || uid < 1000 || uid >= 65534 {
			continue
		}
		users = append(users, SambaUser{Username: parts[0], UID: parts[2]})
	}
	return users, nil
}

func parsePdbedit(output string) []SambaUser {
	var users []SambaUser
	var cur *SambaUser
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if cur != nil {
				users = append(users, *cur)
				cur = nil
			}
			continue
		}
		if strings.HasPrefix(line, "Unix username:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "Unix username:"))
			cur = &SambaUser{Username: name}
		} else if cur != nil {
			switch {
			case strings.HasPrefix(line, "Unix UID:"):
				cur.UID = strings.TrimSpace(strings.TrimPrefix(line, "Unix UID:"))
			case strings.HasPrefix(line, "Full Name:"):
				cur.FullName = strings.TrimSpace(strings.TrimPrefix(line, "Full Name:"))
			case strings.HasPrefix(line, "Account Flags:"):
				flags := strings.TrimSpace(strings.TrimPrefix(line, "Account Flags:"))
				cur.Disabled = strings.Contains(flags, "D")
			}
		}
	}
	if cur != nil {
		users = append(users, *cur)
	}
	return users
}

// SetFullName sets the GECOS/display name for a Linux user.
func SetFullName(username, fullName string) error {
	out, err := exec.Command("usermod", "--comment", fullName, username).CombinedOutput()
	if err != nil {
		return fmt.Errorf("usermod: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func AddUser(username, password string) error {
	out, err := exec.Command("useradd",
		"--no-create-home", "--shell", "/usr/sbin/nologin", username,
	).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "already exists") {
		return fmt.Errorf("useradd: %s", strings.TrimSpace(string(out)))
	}
	return setSambaPassword(username, password)
}

func DeleteUser(username string) error {
	out, err := exec.Command("smbpasswd", "-x", username).CombinedOutput()
	if err != nil {
		s := string(out)
		if !strings.Contains(s, "Failed to find entry") && !strings.Contains(s, "no such user") {
			return fmt.Errorf("smbpasswd -x: %s", strings.TrimSpace(s))
		}
	}
	exec.Command("userdel", "-r", username).Run()
	return nil
}

func SetPassword(username, password string) error {
	return setSambaPassword(username, password)
}

func EnableUser(username string) error {
	out, err := exec.Command("smbpasswd", "-e", username).CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd -e: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func DisableUser(username string) error {
	out, err := exec.Command("smbpasswd", "-d", username).CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd -d: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func setSambaPassword(username, password string) error {
	cmd := exec.Command("smbpasswd", "-a", "-s", username)
	cmd.Stdin = bytes.NewBufferString(password + "\n" + password + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
