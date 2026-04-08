package system

import (
	"fmt"
	"os/exec"
	"strings"
)

// ServiceStatus represents the state of the Samba service.
type ServiceStatus struct {
	Active  bool
	Status  string // "active (running)", "inactive", etc.
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

// StartSamba starts the smbd service.
func StartSamba() error {
	out, err := exec.Command("systemctl", "start", "smbd").CombinedOutput()
	if err != nil {
		return fmt.Errorf("start smbd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// StopSamba stops the smbd service.
func StopSamba() error {
	out, err := exec.Command("systemctl", "stop", "smbd").CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop smbd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RestartSamba restarts the smbd service.
func RestartSamba() error {
	out, err := exec.Command("systemctl", "restart", "smbd").CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart smbd: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ReloadSamba reloads smbd config without a full restart.
func ReloadSamba() error {
	out, err := exec.Command("systemctl", "reload", "smbd").CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload smbd: %s", strings.TrimSpace(string(out)))
	}
	return nil
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

func extractActiveLine(statusOutput string) string {
	for _, line := range strings.Split(statusOutput, "\n") {
		if strings.Contains(line, "Active:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Active:"))
		}
	}
	return "unknown"
}
