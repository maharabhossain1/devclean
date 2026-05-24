package engine

import (
	"os"
	"path/filepath"
	"time"
)

const (
	recentModThreshold = 24 * time.Hour
	largeFileThreshold = 100 * 1024 * 1024 // 100 MB
)

// ScoreTrace assigns a risk level and computes the size for a correlated trace.
// Orphaned and unknown traces get a risk level; owned traces stay Green.
func ScoreTrace(t *Trace) {
	if t.Status == StatusOwned {
		t.Risk = RiskGreen
		return
	}

	// Compute size if not yet set
	if t.SizeBytes == 0 {
		t.SizeBytes = pathSize(t.Path)
	}

	// Files modified very recently are not safe to touch
	if time.Since(t.ModTime) < recentModThreshold {
		t.Risk = RiskYellow
		return
	}

	switch t.Status {
	case StatusOrphaned:
		t.Risk = RiskGreen

	case StatusDeadAgent:
		// LaunchAgents pointing to missing binaries — likely harmless but worth reviewing
		t.Risk = RiskYellow

	case StatusUnknown:
		// Unknown items are Red by default; elevate to Red if they're executable
		if isExecutable(t.Path) {
			t.Risk = RiskRed
		} else {
			t.Risk = RiskRed // all unknowns are Red until proven safe
		}
	}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0111 != 0
}

func pathSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}
