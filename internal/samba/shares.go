package samba

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const SmbConf = "/etc/samba/smb.conf"

// Share represents a section from smb.conf.
type Share struct {
	Name          string
	Path          string
	Comment       string
	ValidUsers    string
	WriteList     string
	ReadOnly      string
	Browseable    string
	GuestOK       string
	CreateMask    string
	DirectoryMask string
	Raw           map[string]string // all other keys
}

// ListShares parses smb.conf and returns configured shares (excluding [global] etc.).
func ListShares() ([]Share, error) {
	f, err := os.Open(SmbConf)
	if err != nil {
		return nil, fmt.Errorf("open smb.conf: %w", err)
	}
	defer f.Close()

	return parseConf(f)
}

func parseConf(r io.Reader) ([]Share, error) {
	var shares []Share
	var current *Share

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// Save previous section
			if current != nil && !isSpecialSection(current.Name) {
				shares = append(shares, *current)
			}

			name := line[1 : len(line)-1]
			current = &Share{
				Name: name,
				Raw:  make(map[string]string),
			}
			continue
		}

		if current == nil {
			continue
		}

		// Key = value
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip inline comments
		if ci := strings.Index(val, " #"); ci > 0 {
			val = strings.TrimSpace(val[:ci])
		}

		switch strings.ToLower(strings.ReplaceAll(key, " ", "")) {
		case "path":
			current.Path = val
		case "comment":
			current.Comment = val
		case "validusers":
			current.ValidUsers = val
		case "writelist":
			current.WriteList = val
		case "readonly":
			current.ReadOnly = val
		case "browseable", "browsable":
			current.Browseable = val
		case "guestok":
			current.GuestOK = val
		case "createmask", "createmmode":
			current.CreateMask = val
		case "directorymask", "directorymode":
			current.DirectoryMask = val
		default:
			current.Raw[key] = val
		}
	}

	if current != nil && !isSpecialSection(current.Name) {
		shares = append(shares, *current)
	}

	return shares, scanner.Err()
}

func isSpecialSection(name string) bool {
	lower := strings.ToLower(name)
	return lower == "global" || lower == "homes" || lower == "printers" || lower == "print$"
}

// GetShare returns a single share by name.
func GetShare(name string) (*Share, error) {
	shares, err := ListShares()
	if err != nil {
		return nil, err
	}
	for _, s := range shares {
		if strings.EqualFold(s.Name, name) {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("share not found: %s", name)
}

// AddShare appends a new share section to smb.conf via sudo tee -a.
func AddShare(s Share) error {
	if err := backupConf(); err != nil {
		return err
	}

	block := buildShareBlock(s)
	cmd := exec.Command("sudo", "tee", "-a", SmbConf)
	cmd.Stdin = strings.NewReader(block)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("append smb.conf: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteShare removes a share section from smb.conf via sudo tee.
func DeleteShare(name string) error {
	if err := backupConf(); err != nil {
		return err
	}

	content, err := os.ReadFile(SmbConf)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var out []string
	skip := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionName := trimmed[1 : len(trimmed)-1]
			skip = strings.EqualFold(sectionName, name)
		}
		if !skip {
			out = append(out, line)
		}
	}

	return writeConf(strings.Join(out, "\n"))
}

// EditShare replaces an existing share section via sudo tee.
func EditShare(originalName string, s Share) error {
	if err := backupConf(); err != nil {
		return err
	}

	content, err := os.ReadFile(SmbConf)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var out []string
	skip := false
	replaced := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionName := trimmed[1 : len(trimmed)-1]
			if strings.EqualFold(sectionName, originalName) {
				// Inject the new block here
				out = append(out, buildShareBlock(s))
				replaced = true
				skip = true
				continue
			}
			skip = false
		}
		if !skip {
			out = append(out, line)
		}
	}

	if !replaced {
		return fmt.Errorf("share '%s' not found in smb.conf", originalName)
	}

	return writeConf(strings.Join(out, "\n"))
}

// writeConf writes content to smb.conf via sudo tee (overwrites).
func writeConf(content string) error {
	cmd := exec.Command("sudo", "tee", SmbConf)
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write smb.conf: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func buildShareBlock(s Share) string {
	var sb strings.Builder
	sb.WriteString("\n[" + s.Name + "]\n")
	if s.Comment != "" {
		sb.WriteString("   comment = " + s.Comment + "\n")
	}
	sb.WriteString("   path = " + s.Path + "\n")

	boolStr := func(val, fallback string) string {
		if val == "" {
			return fallback
		}
		return val
	}

	sb.WriteString("   browseable = " + boolStr(s.Browseable, "yes") + "\n")
	sb.WriteString("   read only = " + boolStr(s.ReadOnly, "no") + "\n")
	sb.WriteString("   guest ok = " + boolStr(s.GuestOK, "no") + "\n")

	if s.ValidUsers != "" {
		sb.WriteString("   valid users = " + s.ValidUsers + "\n")
	}
	if s.WriteList != "" {
		sb.WriteString("   write list = " + s.WriteList + "\n")
	}
	if s.CreateMask != "" {
		sb.WriteString("   create mask = " + s.CreateMask + "\n")
	}
	if s.DirectoryMask != "" {
		sb.WriteString("   directory mask = " + s.DirectoryMask + "\n")
	}
	return sb.String()
}

// backupConf creates a timestamped backup of smb.conf.
func backupConf() error {
	src, err := os.ReadFile(SmbConf)
	if err != nil {
		return fmt.Errorf("read smb.conf: %w", err)
	}

	backupDir := "/var/lib/sambly/backups"
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	dst := filepath.Join(backupDir, "smb.conf."+ts)
	return os.WriteFile(dst, src, 0640)
}
