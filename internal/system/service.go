package system

import (
	"fmt"
	"os/exec"
	"strings"
)

type ServiceStatus struct {
	Active  bool
	Status  string
	Enabled bool
	Version string
}

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
		Version: GetSambaVersion(),
	}
}

func IsSambaInstalled() bool {
	_, err := exec.LookPath("smbd")
	return err == nil
}

func GetSambaVersion() string {
	out, err := exec.Command("smbd", "--version").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func StartSamba() error   { return systemctl("start", "smbd") }
func StopSamba() error    { return systemctl("stop", "smbd") }
func RestartSamba() error { return systemctl("restart", "smbd") }
func ReloadSamba() error  { return systemctl("reload", "smbd") }

func ValidateConf() (string, error) {
	out, err := exec.Command("testparm", "-s", "--suppress-prompt").CombinedOutput()
	return string(out), err
}

func systemctl(action, unit string) error {
	out, err := exec.Command("systemctl", action, unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", action, unit, strings.TrimSpace(string(out)))
	}
	return nil
}

func extractActiveLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "Active:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Active:"))
		}
	}
	return "unknown"
}
