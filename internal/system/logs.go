package system

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SambaLogEntry represents a parsed Samba log line.
type SambaLogEntry struct {
	Time    string
	Level   string // "0"=error, "1"=warn, "2"=info, "3"=debug
	Source  string // file:line(func) part
	Message string
	Raw     string
}

var (
	// Samba log header: [2024/01/15 10:30:45.123456,  0] ../../source3/smbd/server.c:123(main)
	reHeader = regexp.MustCompile(`^\[(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d+),\s*(\d+)\]\s*(.+)$`)
)

// SambaLogPaths returns candidate log file paths to try, in order.
func SambaLogPaths() []string {
	return []string{
		"/var/log/samba/log.smbd",
		"/var/log/samba/log.samba",
		"/var/log/samba/smbd.log",
		"/var/log/samba/log",
	}
}

// FindSambaLog returns the first existing Samba log file.
func FindSambaLog() string {
	for _, p := range SambaLogPaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Try any file in /var/log/samba/
	entries, err := filepath.Glob("/var/log/samba/log.*")
	if err == nil && len(entries) > 0 {
		// prefer log.smbd if present, else first
		for _, e := range entries {
			if strings.HasSuffix(e, "smbd") {
				return e
			}
		}
		return entries[0]
	}
	return ""
}

// ReadSambaLog reads the last `limit` log entries from the Samba log file.
func ReadSambaLog(path string, limit int) ([]SambaLogEntry, error) {
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
			// Save previous entry
			if current != nil {
				entries = append(entries, *current)
			}

			ts := formatSambaTime(m[1])
			level := m[2]
			source := shortSource(m[3])

			current = &SambaLogEntry{
				Time:   ts,
				Level:  level,
				Source: source,
				Raw:    line,
			}
		} else if current != nil {
			// Continuation line — append to message
			msg := strings.TrimSpace(line)
			if msg != "" {
				if current.Message == "" {
					current.Message = msg
				} else {
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

	// Return last `limit` entries (most recent last → reverse for display)
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	// Reverse so newest is first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}

// formatSambaTime converts "2024/01/15 10:30:45.123456" to "2024-01-15 10:30:45".
func formatSambaTime(raw string) string {
	t, err := time.Parse("2006/01/02 15:04:05.999999", raw)
	if err != nil {
		return raw
	}
	return t.Format("2006-01-02 15:04:05")
}

// shortSource trims the long source path to just "filename:line(func)".
func shortSource(s string) string {
	s = strings.TrimSpace(s)
	// ../../source3/smbd/server.c:123(main) → server.c:123(main)
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

// LevelClass returns a CSS class name for a Samba log level.
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

// LevelLabel returns a human label for a Samba log level.
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
