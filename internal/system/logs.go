package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SambaLogEntry represents a parsed Samba log line.
type SambaLogEntry struct {
	Time    string
	Level   string // "0"=error, "1"=warn, "2"=info, "3"=debug
	Source  string
	Message string
}

var reHeader = regexp.MustCompile(`^\[(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d+),\s*(\d+)\]\s*(.+)$`)

// SambaLogPaths returns candidate log file paths to check.
func SambaLogPaths() []string {
	return []string{
		"/var/log/samba/log.smbd",
		"/var/log/samba/log.samba",
		"/var/log/samba/smbd.log",
		"/var/log/samba/log",
	}
}

// FindSambaLog returns the first readable Samba log file path.
func FindSambaLog() string {
	for _, p := range SambaLogPaths() {
		if f, err := os.Open(p); err == nil {
			f.Close()
			return p
		}
	}
	// Try any log.* file in /var/log/samba/
	entries, err := filepath.Glob("/var/log/samba/log.*")
	if err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e, "smbd") {
				if f, err := os.Open(e); err == nil {
					f.Close()
					return e
				}
			}
		}
		for _, e := range entries {
			if f, err := os.Open(e); err == nil {
				f.Close()
				return e
			}
		}
	}
	return ""
}

// ReadSambaLog reads last `limit` entries from the given file.
// Falls back to journalctl if the file cannot be opened.
func ReadSambaLog(path string, limit int) ([]SambaLogEntry, string, error) {
	// Try reading the file directly first
	if path != "" {
		entries, err := readFromFile(path, limit)
		if err == nil {
			return entries, fmt.Sprintf("file: %s", path), nil
		}
	}

	// Fallback: journalctl -u smbd
	entries, err := readFromJournal(limit)
	if err == nil && len(entries) > 0 {
		return entries, "journalctl: smbd", nil
	}

	// Fallback: journalctl -u samba
	entries, err = readFromJournalUnit("samba", limit)
	if err == nil && len(entries) > 0 {
		return entries, "journalctl: samba", nil
	}

	return nil, "", fmt.Errorf("no readable Samba log source found (tried file and journalctl)")
}

func readFromFile(path string, limit int) ([]SambaLogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []SambaLogEntry
	var current *SambaLogEntry

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if m := reHeader.FindStringSubmatch(line); m != nil {
			if current != nil {
				entries = append(entries, *current)
			}
			current = &SambaLogEntry{
				Time:   formatSambaTime(m[1]),
				Level:  m[2],
				Source: shortSource(m[3]),
			}
		} else if current != nil {
			msg := strings.TrimSpace(line)
			if msg != "" {
				if current.Message == "" {
					current.Message = msg
				} else if !strings.Contains(current.Message, msg) {
					current.Message += " | " + msg
				}
			}
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	if err := scanner.Err(); err != nil {
		return entries, err
	}

	return tailAndReverse(entries, limit), nil
}

func readFromJournal(limit int) ([]SambaLogEntry, error) {
	return readFromJournalUnit("smbd", limit)
}

func readFromJournalUnit(unit string, limit int) ([]SambaLogEntry, error) {
	cmd := exec.Command("journalctl",
		"-u", unit,
		"--no-pager",
		"-n", strconv.Itoa(limit),
		"--output=short",
		"--no-hostname",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var entries []SambaLogEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-- ") {
			continue
		}
		// Journal format: "Apr 08 17:11:14 host smbd[pid]: message"
		entry := parseJournalLine(line)
		if entry != nil {
			entries = append(entries, *entry)
		}
	}
	return entries, nil
}

var reJournal = regexp.MustCompile(`^(\w+\s+\d+\s+\d{2}:\d{2}:\d{2})\s+\S+\s+\S+\[?\d*\]?:\s+(.*)$`)

func parseJournalLine(line string) *SambaLogEntry {
	m := reJournal.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	msg := strings.TrimSpace(m[2])
	level := "2" // default info
	lmsg := strings.ToLower(msg)
	if strings.Contains(lmsg, "error") || strings.Contains(lmsg, "failed") || strings.Contains(lmsg, "fail") {
		level = "0"
	} else if strings.Contains(lmsg, "warn") {
		level = "1"
	} else if strings.Contains(lmsg, "debug") {
		level = "3"
	}

	// Parse journal time "Apr 08 17:11:14" → current year assumed
	ts := m[1]
	if t, err := time.Parse("Jan 02 15:04:05", ts); err == nil {
		ts = time.Date(time.Now().Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second(), 0, time.Local).
			Format("2006-01-02 15:04:05")
	}

	return &SambaLogEntry{
		Time:    ts,
		Level:   level,
		Source:  "journal",
		Message: msg,
	}
}

func tailAndReverse(entries []SambaLogEntry, limit int) []SambaLogEntry {
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

func formatSambaTime(raw string) string {
	t, err := time.Parse("2006/01/02 15:04:05.999999", raw)
	if err != nil {
		return raw
	}
	return t.Format("2006-01-02 15:04:05")
}

func shortSource(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// LevelClass returns a CSS class for a log level number.
func LevelClass(level string) string {
	switch level {
	case "0":
		return "log-error"
	case "1":
		return "log-warn"
	case "2":
		return "log-info"
	default:
		return "log-debug"
	}
}

// LevelLabel returns a display label for a log level number.
func LevelLabel(level string) string {
	switch level {
	case "0":
		return "ERROR"
	case "1":
		return "WARN"
	case "2":
		return "INFO"
	default:
		return "DEBUG"
	}
}
