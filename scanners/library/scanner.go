package library

import (
	"os"
	"path/filepath"
	"time"
)

// Trace is a raw file/dir found during scanning (pre-correlation).
type Trace struct {
	Path      string
	SizeBytes int64
	ModTime   time.Time
	Category  string // "app_support", "cache", "preference", "log", "container", "launch_agent", "dotfile"
}

// ScanAll scans all relevant ~/Library subdirectories.
func ScanAll(home string) ([]Trace, error) {
	lib := filepath.Join(home, "Library")
	var traces []Trace

	dirs := []struct {
		sub      string
		category string
		depth    int
	}{
		{"Application Support", "app_support", 1},
		{"Caches", "cache", 1},
		{"Logs", "log", 1},
		{"Containers", "container", 1},
		{"Group Containers", "container", 1},
		{"LaunchAgents", "launch_agent", 0},
		{"Preferences", "preference", 0},
	}

	for _, d := range dirs {
		sub := filepath.Join(lib, d.sub)
		found, err := scanDir(sub, d.category, d.depth)
		if err == nil {
			traces = append(traces, found...)
		}
	}

	return traces, nil
}

// scanDir returns direct children (or all files if depth == 0) of dir.
// depth 1 = immediate children only (directories counted as one trace each).
// depth 0 = individual files only.
func scanDir(dir, category string, depth int) ([]Trace, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var traces []Trace
	for _, entry := range entries {
		// Skip hidden system entries
		name := entry.Name()
		if name == ".DS_Store" || name == ".localized" {
			continue
		}

		path := filepath.Join(dir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if depth == 1 && entry.IsDir() {
			// Treat whole directory as one trace; compute size lazily
			traces = append(traces, Trace{
				Path:      path,
				ModTime:   info.ModTime(),
				Category:  category,
				SizeBytes: 0, // populated by correlator when needed
			})
		} else if depth == 0 {
			traces = append(traces, Trace{
				Path:      path,
				SizeBytes: info.Size(),
				ModTime:   info.ModTime(),
				Category:  category,
			})
		}
	}
	return traces, nil
}
