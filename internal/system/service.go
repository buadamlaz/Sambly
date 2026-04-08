package system

import (
	"fmt"
	"os/exec"
	"strings"
)

// ServiceStatus represents the state of the Samba service.
type ServiceStatus struct {
	Active  bool
	Status  string
	Enabled bool
}

// GetSambaStatus returns the current systemd status of smbd.
func GetSambaStatus() ServiceStatus {
	out, err := exec.Command("systemctl", "is-active", "smbd").Output()
	active := err == nil && strings.TrimSpace(string(out)) == "active"

	statusOut, _ := exec.Command("systemctl", "status", "smbd", "--no-pager", "-l").Output()
	statusStr := extractActiveLine(string(statusOut))

	enabledOut, _ := exec.Command("systemctl", "is-enabled", "smbd").Output()
	enabled := strings.TrimSpace(string(enabledOut)) == "enabled"

	return ServiceStatus{
		Active:  active,
		Status:  statusStr,
		Enabled: enabled,
	}
}

// StartSamba starts the smbd service via sudo.
func StartSamba() error {
	return sudoSystemctl("start", "smbd")
}

// StopSamba stops the smbd service via sudo.
func StopSamba() error {
	return sudoSystemctl("stop", "smbd")
}

// RestartSamba restarts the smbd service via sudo.
func RestartSamba() error {
	return sudoSystemctl("restart", "smbd")
}

// ReloadSamba reloads smbd config via sudo.
func ReloadSamba() error {
	return sudoSystemctl("reload", "smbd")
}

// IsSambaInstalled checks if samba binaries are available.
func IsSambaInstalled() bool {
	_, err := exec.LookPath("smbd")
	return err == nil
}

// ValidateConf runs testparm to validate smb.conf syntax.
func ValidateConf() (string, error) {
	out, err := exec.Command("testparm", "-s", "--suppress-prompt").CombinedOutput()
	return string(out), err
}

// sudoSystemctl runs: sudo systemctl <action> <unit>
func sudoSystemctl(action, unit string) error {
	out, err := exec.Command("sudo", "systemctl", action, unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", action, unit, strings.TrimSpace(string(out)))
	}
	return nil
}

func extractActiveLine(statusOutput string) string {
	for _, line := range strings.Split(statusOutput, "\n") {
		if strings.Contains(line, "Active:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Active:"))
		}
	}
	return "unknown"
}
