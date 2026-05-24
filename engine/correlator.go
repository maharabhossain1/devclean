package engine

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"howett.net/plist"
)

// DictionaryEntry is one entry from the embedded app-trace YAML.
type DictionaryEntry struct {
	BundleID    string   `yaml:"bundle_id"`
	BrewFormula string   `yaml:"brew_formula"`
	BrewCask    string   `yaml:"brew_cask"`
	Traces      []string `yaml:"traces"`
}

// AppDictionary maps a well-known app key to its trace patterns.
type AppDictionary map[string]DictionaryEntry

// Correlator matches raw traces against the installed-app registry.
type Correlator struct {
	byBundleID map[string]AppEntry // lowercase bundle ID → entry
	byName     map[string]AppEntry // lowercase name → entry
	dict       AppDictionary
	home       string
}

// NewCorrelator builds a Correlator from the app registry and dictionary.
func NewCorrelator(apps []AppEntry, dict AppDictionary, home string) *Correlator {
	return &Correlator{
		byBundleID: IndexByBundleID(apps),
		byName:     IndexByName(apps),
		dict:       dict,
		home:       home,
	}
}

// Correlate classifies a single raw trace.
func (c *Correlator) Correlate(raw RawTrace) Trace {
	trace := Trace{
		Path:     raw.Path,
		ModTime:  modTime(raw.Path),
		Category: raw.Category,
	}

	// LaunchAgents/Daemons need special handling
	if raw.Category == "launch_agent" || raw.Category == "launch_daemon" {
		return c.correlatePlist(trace)
	}

	// macOS system components are always owned — never flag them
	base := filepath.Base(raw.Path)
	if isAppleSystemPath(base, raw.Path) {
		trace.Status = StatusOwned
		trace.OwnerApp = "macOS"
		return trace
	}

	// Try bundle ID from the path itself (e.g. "com.zoom.us")
	if bundleID := extractBundleID(raw.Path); bundleID != "" {
		if app, ok := c.byBundleID[strings.ToLower(bundleID)]; ok {
			trace.Status = StatusOwned
			trace.OwnerApp = app.Name
			return trace
		}
	}

	// Fuzzy name match — "Application Support/Slack" → "slack"
	if name := extractAppName(raw.Path); name != "" {
		if app, ok := c.byName[strings.ToLower(name)]; ok {
			trace.Status = StatusOwned
			trace.OwnerApp = app.Name
			return trace
		}
		// Also try partial match (e.g. "zoom.us" → "zoom")
		for key, app := range c.byName {
			if strings.Contains(strings.ToLower(name), key) || strings.Contains(key, strings.ToLower(name)) {
				trace.Status = StatusOwned
				trace.OwnerApp = app.Name
				return trace
			}
		}
	}

	// Dictionary lookup
	if owner := c.dictLookup(raw.Path); owner != "" {
		// Check whether the owning app is actually installed
		if _, ok := c.byName[strings.ToLower(owner)]; ok {
			trace.Status = StatusOwned
			trace.OwnerApp = owner
			return trace
		}
		// Dictionary matched but app not installed → orphan
		trace.Status = StatusOrphaned
		trace.OwnerApp = owner
		return trace
	}

	// Not matched by any method
	trace.Status = StatusUnknown
	return trace
}

func (c *Correlator) correlatePlist(trace Trace) Trace {
	data, err := os.ReadFile(trace.Path)
	if err != nil {
		trace.Status = StatusUnknown
		return trace
	}

	var agent struct {
		Label   string
		Program string
		ProgramArguments []string
	}
	if _, err := plist.Unmarshal(data, &agent); err != nil {
		trace.Status = StatusUnknown
		return trace
	}

	binary := agent.Program
	if binary == "" && len(agent.ProgramArguments) > 0 {
		binary = agent.ProgramArguments[0]
	}

	if binary != "" {
		// Expand ~ in binary path
		if strings.HasPrefix(binary, "~/") {
			binary = filepath.Join(c.home, binary[2:])
		}
		if _, err := os.Stat(binary); os.IsNotExist(err) {
			trace.Status = StatusDeadAgent
			trace.Category = "dead_agent"
			return trace
		}
	}

	// Binary exists — try to match label to a known app
	label := strings.ToLower(agent.Label)
	for key, app := range c.byBundleID {
		if strings.Contains(label, strings.ToLower(key)) {
			trace.Status = StatusOwned
			trace.OwnerApp = app.Name
			return trace
		}
	}
	for key, app := range c.byName {
		if strings.Contains(label, key) {
			trace.Status = StatusOwned
			trace.OwnerApp = app.Name
			return trace
		}
	}

	// Fall through to dictionary lookup (catches e.g. Keystone → Chrome)
	if owner := c.dictLookup(trace.Path); owner != "" {
		if _, ok := c.byName[strings.ToLower(owner)]; ok {
			trace.Status = StatusOwned
			trace.OwnerApp = owner
			return trace
		}
		trace.Status = StatusOrphaned
		trace.OwnerApp = owner
		return trace
	}

	trace.Status = StatusUnknown
	return trace
}

// dictLookup checks whether the path matches any trace pattern in the dictionary.
// Returns the app key if matched, empty string otherwise.
func (c *Correlator) dictLookup(path string) string {
	expanded := expandHome(path, c.home)
	for appKey, entry := range c.dict {
		for _, pattern := range entry.Traces {
			patternExpanded := expandHome(pattern, c.home)
			if matchPattern(expanded, patternExpanded) {
				return appKey
			}
		}
	}
	return ""
}

func matchPattern(path, pattern string) bool {
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}
		// Also check if path starts with the non-glob prefix
		prefix := strings.SplitN(pattern, "*", 2)[0]
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern || strings.HasPrefix(path, pattern+"/")
}

// extractBundleID tries to pull a reverse-DNS bundle ID from a path component.
// e.g. "com.zoom.us" from "~/Library/Caches/com.zoom.us"
func extractBundleID(path string) string {
	base := filepath.Base(path)
	// Remove .plist extension
	base = strings.TrimSuffix(base, ".plist")
	// A bundle ID has at least two dots and starts with a known TLD
	parts := strings.Split(base, ".")
	if len(parts) >= 3 {
		tlds := []string{"com", "org", "net", "io", "dev", "app", "us", "co", "ai", "se", "at", "fr", "de", "ru"}
		for _, tld := range tlds {
			if parts[0] == tld {
				return base
			}
		}
	}
	return ""
}

// extractAppName gets the last path component stripped of common suffixes and prefixes.
func extractAppName(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".plist")
	name = strings.TrimSuffix(name, ".app")
	name = strings.TrimPrefix(name, ".") // handle dotfiles like ~/.npm → "npm"
	return name
}

// isAppleSystemPath returns true for paths that belong to macOS system components.
func isAppleSystemPath(base, fullPath string) bool {
	// Bundle ID prefixes used by Apple — including team-ID-prefixed group containers
	appleGroupPrefixes := []string{
		"com.apple.",
		"group.com.apple.",
		"group.is.workflow.",
		"systemgroup.com.apple.",
		"74J34U3R6X.com.apple.", // iWork family
		"243LU875E5.groups.com.apple.", // Podcasts and similar
	}
	for _, p := range appleGroupPrefixes {
		if strings.HasPrefix(base, p) {
			return true
		}
	}
	// Any path component containing .com.apple. (catches team-ID prefixes)
	if strings.Contains(base, ".com.apple.") {
		return true
	}

	// Well-known macOS system directory names (non-bundle-ID form)
	appleSystemDirs := map[string]bool{
		// ~/Library/Application Support
		"AddressBook": true, "Animoji": true, "AskPermission": true,
		"CallHistoryDB": true, "CallHistoryTransactions": true,
		"DifferentialPrivacy": true, "DiskImages": true,
		"FaceTime": true, "FileProvider": true, "homeenergyd": true,
		"iCloud": true, "icdd": true, "Knowledge": true,
		"locationaccessstored": true, "Microsoft": true,
		"networkserviceproxy": true,
		// ~/Library/Caches
		"GeoServices": true, "GameKit": true, "CloudKit": true,
		"PassKit": true, "SiriTTS": true, "familycircled": true,
		"FamilyCircle": true, "mbuseragent": true, "askpermissiond": true,
		"SentryCrash": true, "chrome_crashpad_handler": true,
		"io.sentry": true, "fanal": true, "Caches": true,
		// ~/Library/Logs
		"SiriTTSService": true, "PrivacyPreservingMeasurement": true,
		"Baseband": true, "Assistant": true, "DiagnosticReports": true,
		"AppAnalytics": true, "CrashReporter": true, "Homebrew": true,
		// ~/Library/Preferences (system plists)
		"ByHost": true, "MobileMeAccounts.plist": true,
		"TokenBucketRateLimiter.plist": true, "ContextStoreAgent.plist": true,
		"loginwindow.plist": true, "pbs.plist": true, "sharedfilelistd.plist": true,
		"mbuseragent.plist": true, "icloudmailagent.plist": true,
		"diagnostics_agent.plist": true, "familycircled.plist": true,
		"no_backup": true,
		".GlobalPreferences.plist": true, ".GlobalPreferences_m.plist": true,
		// Misc
		"BraveSoftware": true, "Chromium": true,
	}
	return appleSystemDirs[base]
}

func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
