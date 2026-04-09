package samba

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// SambaUser represents a Samba user from pdbedit.
type SambaUser struct {
	Username string
	UID      string
	Disabled bool
	FullName string
}

// ListUsers returns Samba users.
// Tries pdbedit first (most accurate); falls back to reading /etc/passwd
// for regular accounts (UID 1000-65533) — no sudo required for fallback.
func ListUsers() ([]SambaUser, error) {
	out, err := runCommand("sudo", "pdbedit", "-L", "-v")
	if err == nil {
		users := parsePdbedit(out)
		if len(users) > 0 {
			return users, nil
		}
	}
	return listUsersFromPasswd()
}

// listUsersFromPasswd reads /etc/passwd and returns accounts with UID >= 1000.
// This is always readable without sudo and covers all Samba users we create.
func listUsersFromPasswd() ([]SambaUser, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return []SambaUser{}, nil
	}
	var users []SambaUser
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		uid, err := strconv.Atoi(parts[2])
		if err != nil || uid < 1000 || uid >= 65534 {
			continue // skip system users and nobody
		}
		users = append(users, SambaUser{
			Username: parts[0],
			UID:      parts[2],
		})
	}
	return users, nil
}

func parsePdbedit(output string) []SambaUser {
	var users []SambaUser
	var current *SambaUser

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				users = append(users, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "Unix username:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "Unix username:"))
			current = &SambaUser{Username: name}
		} else if current != nil {
			if strings.HasPrefix(line, "Unix UID:") {
				current.UID = strings.TrimSpace(strings.TrimPrefix(line, "Unix UID:"))
			} else if strings.HasPrefix(line, "Full Name:") {
				current.FullName = strings.TrimSpace(strings.TrimPrefix(line, "Full Name:"))
			} else if strings.HasPrefix(line, "Account Flags:") {
				flags := strings.TrimSpace(strings.TrimPrefix(line, "Account Flags:"))
				current.Disabled = strings.Contains(flags, "D")
			}
		}
	}

	if current != nil {
		users = append(users, *current)
	}

	return users
}

// AddUser creates a Linux system user (no shell, no home dir) and adds to Samba.
// Password is passed via stdin — never as a command-line argument.
func AddUser(username, password string) error {
	out, err := exec.Command("sudo", "useradd",
		"--no-create-home",
		"--shell", "/usr/sbin/nologin",
		username,
	).CombinedOutput()
	if err != nil {
		if !strings.Contains(string(out), "already exists") {
			return fmt.Errorf("useradd: %s", strings.TrimSpace(string(out)))
		}
	}
	return setSambaPassword(username, password)
}

// DeleteUser removes a user from Samba and the system.
// smbpasswd -x is best-effort: if the user isn't in the Samba DB
// (e.g. was never fully added), we still remove the system account.
func DeleteUser(username string) error {
	out, err := runCommand("sudo", "smbpasswd", "-x", username)
	if err != nil {
		// Only fail if the user actually exists in the system but samba removal errored.
		// If the error is "Failed to find entry..." the user was never in Samba DB — continue.
		if !strings.Contains(out, "Failed to find entry") &&
			!strings.Contains(out, "no such user") {
			return fmt.Errorf("remove from samba: %s", strings.TrimSpace(out))
		}
	}
	// Remove system user (best-effort, ignore if already gone)
	exec.Command("sudo", "userdel", "-r", username).Run()
	return nil
}

// SetPassword changes a Samba user's password via stdin.
func SetPassword(username, password string) error {
	return setSambaPassword(username, password)
}

// EnableUser re-enables a disabled Samba account.
func EnableUser(username string) error {
	_, err := runCommand("sudo", "smbpasswd", "-e", username)
	return err
}

// DisableUser disables a Samba account.
func DisableUser(username string) error {
	_, err := runCommand("sudo", "smbpasswd", "-d", username)
	return err
}

func setSambaPassword(username, password string) error {
	cmd := exec.Command("sudo", "smbpasswd", "-a", "-s", username)
	passInput := password + "\n" + password + "\n"
	cmd.Stdin = bytes.NewBufferString(passInput)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func runCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}