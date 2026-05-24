package engine

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"howett.net/plist"
)

// BuildAppRegistry catalogs all installed applications and tools.
func BuildAppRegistry() ([]AppEntry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var entries []AppEntry

	entries = append(entries, scanApplicationDirs(home)...)
	entries = append(entries, scanBrew()...)
	entries = append(entries, scanBrewCask()...)
	entries = append(entries, scanNPMGlobal()...)
	entries = append(entries, scanPipGlobal()...)
	entries = append(entries, scanCargoInstall(home)...)
	entries = append(entries, scanMCPServers(home)...)
	entries = append(entries, scanAdHocBinaries(home)...)

	return deduplicate(entries), nil
}

func scanApplicationDirs(home string) []AppEntry {
	dirs := []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join(home, "Applications"),
	}

	var entries []AppEntry
	for _, dir := range dirs {
		apps, _ := filepath.Glob(filepath.Join(dir, "*.app"))
		for _, app := range apps {
			name := strings.TrimSuffix(filepath.Base(app), ".app")
			entry := AppEntry{
				Name:        name,
				BundleID:    readBundleID(app),
				Source:      SourceApplicationsDir,
				InstallPath: app,
				LastLaunch:  readLastLaunch(app),
				SizeBytes:   dirSize(app),
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

func readBundleID(appPath string) string {
	infoPlist := filepath.Join(appPath, "Contents", "Info.plist")
	data, err := os.ReadFile(infoPlist)
	if err != nil {
		return ""
	}

	var info struct {
		CFBundleIdentifier string
		CFBundleVersion    string
	}
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return ""
	}
	return info.CFBundleIdentifier
}

func readLastLaunch(appPath string) time.Time {
	out, err := exec.Command("mdls", "-name", "kMDItemLastUsedDate", "-raw", appPath).Output()
	if err != nil {
		return time.Time{}
	}
	s := strings.TrimSpace(string(out))
	if s == "(null)" || s == "" {
		return time.Time{}
	}
	// mdls returns: 2024-01-15 10:23:45 +0000
	t, err := time.Parse("2006-01-02 15:04:05 +0000", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func scanBrew() []AppEntry {
	out, err := exec.Command("brew", "list", "--formula", "--full-name").Output()
	if err != nil {
		return nil
	}
	var entries []AppEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" {
			continue
		}
		entries = append(entries, AppEntry{
			Name:        name,
			Source:      SourceBrew,
			InstallPath: brewPrefix(name),
		})
	}
	return entries
}

func scanBrewCask() []AppEntry {
	out, err := exec.Command("brew", "list", "--cask").Output()
	if err != nil {
		return nil
	}
	var entries []AppEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" {
			continue
		}
		entries = append(entries, AppEntry{
			Name:        name,
			Source:      SourceBrewCask,
			InstallPath: brewCaskAppPath(name),
		})
	}
	return entries
}

func brewPrefix(formula string) string {
	out, err := exec.Command("brew", "--prefix", formula).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func brewCaskAppPath(cask string) string {
	// Best-effort: look for matching .app in /Applications
	dirs := []string{"/Applications", "/Applications/Utilities"}
	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.app"))
		for _, m := range matches {
			name := strings.ToLower(strings.TrimSuffix(filepath.Base(m), ".app"))
			if strings.Contains(name, strings.ToLower(cask)) {
				return m
			}
		}
	}
	return ""
}

func scanNPMGlobal() []AppEntry {
	managers := []string{"npm", "pnpm"}
	var entries []AppEntry
	for _, mgr := range managers {
		out, err := exec.Command(mgr, "list", "-g", "--depth=0", "--parseable").Output()
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			name := filepath.Base(line)
			if name == "lib" || name == "node_modules" {
				continue
			}
			entries = append(entries, AppEntry{
				Name:        name,
				Source:      SourceNPM,
				InstallPath: line,
			})
		}
	}
	return entries
}

func scanPipGlobal() []AppEntry {
	out, err := exec.Command("pip3", "list", "--format=columns").Output()
	if err != nil {
		out, err = exec.Command("pip", "list", "--format=columns").Output()
		if err != nil {
			return nil
		}
	}
	var entries []AppEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()
		if firstLine || strings.HasPrefix(line, "---") {
			firstLine = false
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			entries = append(entries, AppEntry{
				Name:    parts[0],
				Version: func() string {
					if len(parts) >= 2 {
						return parts[1]
					}
					return ""
				}(),
				Source: SourcePip,
			})
		}
	}
	return entries
}

func scanCargoInstall(home string) []AppEntry {
	out, err := exec.Command("cargo", "install", "--list").Output()
	if err != nil {
		return nil
	}
	var entries []AppEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		// Lines like: "ripgrep v13.0.0:"
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "    ") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				entries = append(entries, AppEntry{
					Name:        parts[0],
					Source:      SourceCargo,
					InstallPath: filepath.Join(home, ".cargo", "bin", parts[0]),
				})
			}
		}
	}
	return entries
}

func scanMCPServers(home string) []AppEntry {
	// Claude Code MCP servers
	mcpPaths := []string{
		filepath.Join(home, ".claude", "claude_desktop_config.json"),
		filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
	}

	var entries []AppEntry
	for _, p := range mcpPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		servers := parseMCPConfig(string(data))
		for _, s := range servers {
			entries = append(entries, AppEntry{
				Name:        s,
				Source:      SourceMCP,
				InstallPath: p,
			})
		}
	}
	return entries
}

func parseMCPConfig(content string) []string {
	// Simple extraction: look for "command": "..." lines
	var names []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"command"`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.Trim(strings.TrimSpace(parts[1]), `",`)
				if val != "" {
					names = append(names, filepath.Base(val))
				}
			}
		}
	}
	return names
}

func scanAdHocBinaries(home string) []AppEntry {
	dirs := []string{
		filepath.Join(home, "bin"),
		filepath.Join(home, ".local", "bin"),
		"/usr/local/bin",
	}
	var entries []AppEntry
	for _, dir := range dirs {
		infos, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, info := range infos {
			if info.IsDir() {
				continue
			}
			path := filepath.Join(dir, info.Name())
			fi, err := os.Stat(path)
			if err != nil {
				continue
			}
			if fi.Mode()&0111 == 0 {
				continue // not executable
			}
			entries = append(entries, AppEntry{
				Name:        info.Name(),
				Source:      SourceBinary,
				InstallPath: path,
				SizeBytes:   fi.Size(),
			})
		}
	}
	return entries
}

// IndexByBundleID returns a map from bundle ID to AppEntry for fast lookup.
func IndexByBundleID(apps []AppEntry) map[string]AppEntry {
	m := make(map[string]AppEntry, len(apps))
	for _, a := range apps {
		if a.BundleID != "" {
			m[strings.ToLower(a.BundleID)] = a
		}
	}
	return m
}

// IndexByName returns a map from lowercase name to AppEntry.
func IndexByName(apps []AppEntry) map[string]AppEntry {
	m := make(map[string]AppEntry, len(apps))
	for _, a := range apps {
		m[strings.ToLower(a.Name)] = a
	}
	return m
}

func deduplicate(entries []AppEntry) []AppEntry {
	seen := make(map[string]bool)
	out := make([]AppEntry, 0, len(entries))
	for _, e := range entries {
		key := string(e.Source) + ":" + e.Name
		if !seen[key] {
			seen[key] = true
			out = append(out, e)
		}
	}
	return out
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}
