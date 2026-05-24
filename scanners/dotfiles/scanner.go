package dotfiles

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maharabhossain1/devclean/scanners/library"
)

// Scan finds dotfiles and dotdirs in home that look like app traces.
func Scan(home string) ([]library.Trace, error) {
	entries, err := os.ReadDir(home)
	if err != nil {
		return nil, err
	}

	// Never touch these safety-critical or well-known infrastructure dotfiles
	blocked := map[string]bool{
		// Security
		".ssh": true, ".gnupg": true, ".aws": true,
		// Shell config
		".zshrc": true, ".bashrc": true, ".bash_profile": true,
		".zprofile": true, ".profile": true, ".zsh_sessions": true,
		// Tool config files (not app traces)
		".gitconfig": true, ".gitignore": true, ".npmrc": true,
		".yarnrc": true, ".yarnrc.yml": true, ".curlrc": true,
		".wgetrc": true, ".editorconfig": true,
		// XDG base directories — contain many tools, not one app
		".local": true, ".cache": true, ".config": true,
		// Version managers / package managers (handled via dictionary)
		".cargo": true,
		// Shell history — never an app trace
		".zsh_history": true, ".bash_history": true,
		// Dotfiles repo conventions
		".dotfiles": true, ".dotconfig": true,
		// macOS system
		".Trash": true, ".DS_Store": true,
	}

	var traces []library.Trace
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".") {
			continue
		}
		if blocked[name] {
			continue
		}
		// Only consider dirs and rc files likely to be app-specific
		if !entry.IsDir() && !strings.HasSuffix(name, "rc") && !strings.HasSuffix(name, "_history") {
			continue
		}

		path := filepath.Join(home, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		var modTime time.Time
		if info != nil {
			modTime = info.ModTime()
		}

		traces = append(traces, library.Trace{
			Path:     path,
			ModTime:  modTime,
			Category: "dotfile",
		})
	}
	return traces, nil
}
