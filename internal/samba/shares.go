package samba

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const smbConf = "/etc/samba/smb.conf"

// hiddenSections are skipped when listing shares.
var hiddenSections = map[string]bool{
	"global":   true,
	"printers": true,
	"print$":   true,
	"homes":    true,
}

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
	// Raw holds any unrecognised key=value pairs so we don't lose them on rewrite.
	Raw map[string]string
}

// ListShares parses smb.conf and returns non-system shares.
func ListShares() ([]Share, error) {
	sections, err := parseSmbConf()
	if err != nil {
		return nil, err
	}
	var shares []Share
	for _, s := range sections {
		if hiddenSections[strings.ToLower(s.Name)] {
			continue
		}
		shares = append(shares, sectionToShare(s))
	}
	return shares, nil
}

// GetShare returns a single share by name.
func GetShare(name string) (*Share, error) {
	sections, err := parseSmbConf()
	if err != nil {
		return nil, err
	}
	for _, s := range sections {
		if strings.EqualFold(s.Name, name) {
			sh := sectionToShare(s)
			return &sh, nil
		}
	}
	return nil, fmt.Errorf("share %q not found", name)
}

// AddShare appends a new section to smb.conf.
func AddShare(share Share) error {
	sections, err := parseSmbConf()
	if err != nil {
		return err
	}
	for _, s := range sections {
		if strings.EqualFold(s.Name, share.Name) {
			return fmt.Errorf("share %q already exists", share.Name)
		}
	}
	sections = append(sections, shareToSection(share))
	return writeSmbConf(sections)
}

// EditShare replaces the section with oldName.
func EditShare(oldName string, share Share) error {
	sections, err := parseSmbConf()
	if err != nil {
		return err
	}
	found := false
	for i, s := range sections {
		if strings.EqualFold(s.Name, oldName) {
			sections[i] = shareToSection(share)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("share %q not found", oldName)
	}
	return writeSmbConf(sections)
}

// DeleteShare removes the named section from smb.conf.
func DeleteShare(name string) error {
	sections, err := parseSmbConf()
	if err != nil {
		return err
	}
	filtered := sections[:0]
	for _, s := range sections {
		if !strings.EqualFold(s.Name, name) {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == len(sections) {
		return fmt.Errorf("share %q not found", name)
	}
	return writeSmbConf(filtered)
}

// SetupShareDirectory creates the directory and sets ownership.
func SetupShareDirectory(path, owner string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if owner == "" {
		return nil
	}
	out, err := exec.Command("chown", owner+":"+owner, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("chown: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// --- internal INI parser ---

type iniSection struct {
	Name   string
	Lines  []string // raw key=value lines, comments preserved
	KV     map[string]string
}

func parseSmbConf() ([]iniSection, error) {
	f, err := os.Open(smbConf)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", smbConf, err)
	}
	defer f.Close()

	var sections []iniSection
	var cur *iniSection

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if cur != nil {
				sections = append(sections, *cur)
			}
			name := trimmed[1 : len(trimmed)-1]
			cur = &iniSection{Name: name, KV: make(map[string]string)}
			continue
		}

		if cur != nil {
			cur.Lines = append(cur.Lines, line)
			if !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				cur.KV[strings.ToLower(k)] = v
			}
		}
	}
	if cur != nil {
		sections = append(sections, *cur)
	}
	return sections, scanner.Err()
}

func writeSmbConf(sections []iniSection) error {
	// Backup first
	data, _ := os.ReadFile(smbConf)
	os.WriteFile(smbConf+".bak", data, 0640)

	var sb strings.Builder
	for i, s := range sections {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[" + s.Name + "]\n")

		// Write lines, collapsing consecutive blank lines into one
		prevBlank := false
		for _, line := range s.Lines {
			isBlank := strings.TrimSpace(line) == ""
			if isBlank {
				if !prevBlank {
					sb.WriteString("\n")
				}
				prevBlank = true
				continue
			}
			prevBlank = false
			sb.WriteString(line + "\n")
		}
	}

	// Final pass: collapse any 3+ consecutive newlines to 2
	output := collapseNewlines(sb.String())

	tmp := smbConf + ".tmp"
	if err := os.WriteFile(tmp, []byte(output), 0640); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	return os.Rename(tmp, smbConf)
}

// collapseNewlines reduces runs of 3+ consecutive newlines to exactly 2.
func collapseNewlines(s string) string {
	var out []byte
	blanks := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			blanks++
			if blanks <= 2 {
				out = append(out, '\n')
			}
		} else {
			blanks = 0
			out = append(out, s[i])
		}
	}
	return string(out)
}

func sectionToShare(s iniSection) Share {
	get := func(key string) string { return s.KV[key] }
	raw := make(map[string]string)
	known := map[string]bool{
		"path": true, "comment": true, "valid users": true, "write list": true,
		"read only": true, "browseable": true, "guest ok": true,
		"create mask": true, "directory mask": true,
	}
	for k, v := range s.KV {
		if !known[k] {
			raw[k] = v
		}
	}
	return Share{
		Name:          s.Name,
		Path:          get("path"),
		Comment:       get("comment"),
		ValidUsers:    get("valid users"),
		WriteList:     get("write list"),
		ReadOnly:      get("read only"),
		Browseable:    get("browseable"),
		GuestOK:       get("guest ok"),
		CreateMask:    get("create mask"),
		DirectoryMask: get("directory mask"),
		Raw:           raw,
	}
}

func shareToSection(sh Share) iniSection {
	s := iniSection{Name: sh.Name, KV: make(map[string]string)}
	addLine := func(key, val string) {
		if val == "" {
			return
		}
		s.Lines = append(s.Lines, "\t"+key+" = "+val)
		s.KV[strings.ToLower(key)] = val
	}
	addLine("path", sh.Path)
	addLine("comment", sh.Comment)
	addLine("valid users", sh.ValidUsers)
	addLine("write list", sh.WriteList)
	addLine("read only", sh.ReadOnly)
	addLine("browseable", sh.Browseable)
	addLine("guest ok", sh.GuestOK)
	addLine("create mask", sh.CreateMask)
	addLine("directory mask", sh.DirectoryMask)
	for k, v := range sh.Raw {
		addLine(k, v)
	}
	return s
}

// ─── Global settings ────────────────────────────────────────────────────────

// GlobalSettings holds the editable subset of [global] smb.conf keys.
type GlobalSettings struct {
	// Server identity
	Workgroup    string
	ServerString string
	NetbiosName  string
	// Security / guest
	Security     string
	MapToGuest   string
	GuestAccount string
	// Network
	HostsAllow     string
	HostsDeny      string
	Interfaces     string
	BindInterfaces string
	// Logging
	LogLevel   string
	MaxLogSize string
	// Printing
	LoadPrinters string
	Printing     string
}

// GetGlobalSettings reads [global] section and returns structured settings.
func GetGlobalSettings() GlobalSettings {
	sections, err := parseSmbConf()
	if err != nil {
		return GlobalSettings{}
	}
	for _, s := range sections {
		if strings.ToLower(s.Name) == "global" {
			g := func(k string) string { return s.KV[k] }
			return GlobalSettings{
				Workgroup:      g("workgroup"),
				ServerString:   g("server string"),
				NetbiosName:    g("netbios name"),
				Security:       g("security"),
				MapToGuest:     g("map to guest"),
				GuestAccount:   g("guest account"),
				HostsAllow:     g("hosts allow"),
				HostsDeny:      g("hosts deny"),
				Interfaces:     g("interfaces"),
				BindInterfaces: g("bind interfaces only"),
				LogLevel:       g("log level"),
				MaxLogSize:     g("max log size"),
				LoadPrinters:   g("load printers"),
				Printing:       g("printing"),
			}
		}
	}
	return GlobalSettings{}
}

// UpdateGlobalSection applies key=value updates to [global].
// Rebuilds the section cleanly — no comment lines, no blank lines.
// Pass an empty string value to remove a key.
func UpdateGlobalSection(updates map[string]string) error {
	sections, err := parseSmbConf()
	if err != nil {
		return err
	}
	found := false
	for i, s := range sections {
		if strings.ToLower(s.Name) == "global" {
			sections[i] = applyGlobalUpdates(s, updates)
			found = true
			break
		}
	}
	if !found {
		sections = append([]iniSection{
			applyGlobalUpdates(iniSection{Name: "global", KV: make(map[string]string)}, updates),
		}, sections...)
	}
	return writeSmbConf(sections)
}

// applyGlobalUpdates rebuilds the [global] section cleanly:
// - strips all comment lines and blank lines
// - preserves original key order
// - applies updates (empty string = delete key)
// - appends new keys at the end
func applyGlobalUpdates(s iniSection, updates map[string]string) iniSection {
	// Merged final KV: existing + updates
	merged := make(map[string]string)
	for k, v := range s.KV {
		merged[k] = v
	}
	for k, v := range updates {
		if v != "" {
			merged[k] = v
		} else {
			delete(merged, k)
		}
	}

	// Iterate original lines to preserve key order, skip comments/blanks
	var newLines []string
	seen := make(map[string]bool)
	for _, line := range s.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue // drop comments and blank lines
		}
		if idx := strings.Index(trimmed, "="); idx > 0 {
			normKey := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
			if seen[normKey] {
				continue
			}
			seen[normKey] = true
			if v, ok := merged[normKey]; ok {
				newLines = append(newLines, "\t"+normKey+" = "+v)
			}
			// key not in merged = was deleted, skip
		}
	}
	// Append keys that are new (in updates but not in original lines)
	for k, v := range updates {
		if !seen[k] && v != "" {
			newLines = append(newLines, "\t"+k+" = "+v)
		}
	}

	newKV := make(map[string]string)
	for k, v := range s.KV {
		newKV[k] = v
	}
	for k, v := range updates {
		if v != "" {
			newKV[k] = v
		} else {
			delete(newKV, k)
		}
	}
	return iniSection{Name: s.Name, Lines: newLines, KV: newKV}
}

// ─── Printer sharing ────────────────────────────────────────────────────────

// IsPrinterSharingEnabled reports whether [printers] section exists in smb.conf.
func IsPrinterSharingEnabled() bool {
	sections, _ := parseSmbConf()
	for _, s := range sections {
		if strings.ToLower(s.Name) == "printers" {
			return true
		}
	}
	return false
}

// SetPrinterSharing adds or removes the [printers] section.
func SetPrinterSharing(enable bool) error {
	sections, err := parseSmbConf()
	if err != nil {
		return err
	}
	hasPrinters := false
	for _, s := range sections {
		if strings.ToLower(s.Name) == "printers" {
			hasPrinters = true
			break
		}
	}
	if enable == hasPrinters {
		return nil // no change needed
	}
	if enable {
		printers := iniSection{
			Name: "printers",
			Lines: []string{
				"\tcomment = All Printers",
				"\tbrowseable = no",
				"\tpath = /var/spool/samba",
				"\tprintable = yes",
				"\tguest ok = no",
				"\tread only = yes",
				"\tcreate mask = 0700",
			},
			KV: map[string]string{
				"comment": "All Printers", "browseable": "no",
				"path": "/var/spool/samba", "printable": "yes",
			},
		}
		sections = append(sections, printers)
	} else {
		var filtered []iniSection
		for _, s := range sections {
			n := strings.ToLower(s.Name)
			if n != "printers" && n != "print$" {
				filtered = append(filtered, s)
			}
		}
		sections = filtered
	}
	return writeSmbConf(sections)
}
