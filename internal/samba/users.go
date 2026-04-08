package samba

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// SambaUser represents a Samba user from pdbedit.
type SambaUser struct {
	Username string
	UID      string
	Disabled bool
	FullName string
}

// ListUsers returns all Samba users via pdbedit.
func ListUsers() ([]SambaUser, error) {
	out, err := runCommand("sudo", "pdbedit", "-L", "-v")
	if err != nil {
		// pdbedit may fail if no users exist yet — return empty list
		return []SambaUser{}, nil
	}

	return parsePdbedit(out), nil
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
				// Disabled accounts have 'D' flag: [D          ]
				current.Disabled = strings.Contains(flags, "D")
			}
		}
	}

	if current != nil {
		users = append(users, *current)
	}

	return users
}

// AddUser creates a Linux system user and adds them to Samba.
// password is set via stdin to smbpasswd — never passed as a shell argument.
func AddUser(username, password string) error {
	out, err := exec.Command("sudo", "useradd",
		"--no-create-home",
		"--shell", "/usr/sbin/nologin",
		username,
	).CombinedOutput()
	if err != nil {
		// User might already exist — check
		if !strings.Contains(string(out), "already exists") {
			return fmt.Errorf("useradd: %s", strings.TrimSpace(string(out)))
		}
	}

	// Add to Samba and set password via stdin
	return setSambaPassword(username, password)
}

// DeleteUser removes a user from Samba (and optionally the system).
func DeleteUser(username string) error {
	// Remove from Samba database
	if _, err := runCommand("sudo", "smbpasswd", "-x", username); err != nil {
		return fmt.Errorf("remove from samba: %w", err)
	}
	// Remove from system (best-effort)
	exec.Command("sudo", "userdel", username).Run()
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

// setSambaPassword uses smbpasswd to set/change a password via stdin.
// This avoids passing the password as a command-line argument.
func setSambaPassword(username, password string) error {
	cmd := exec.Command("sudo", "smbpasswd", "-a", "-s", username)
	// smbpasswd -s reads password from stdin, expecting it twice
	passInput := password + "\n" + password + "\n"
	cmd.Stdin = bytes.NewBufferString(passInput)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// runCommand executes a command and returns combined output.
func runCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}
