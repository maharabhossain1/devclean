package cleaner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// DeletionMethod tracks how an item was removed so undo knows what to do.
type DeletionMethod string

const (
	MethodTrash  DeletionMethod = "trash"
	MethodDelete DeletionMethod = "delete"
)

// LogEntry is one deleted item recorded for undo.
type LogEntry struct {
	Path      string         `json:"path"`
	SizeBytes int64          `json:"size_bytes"`
	Method    DeletionMethod `json:"method"`
	DeletedAt time.Time      `json:"deleted_at"`
}

// UndoLog is written to ~/.devclean/undo/<timestamp>.json after a session.
type UndoLog struct {
	SessionID string       `json:"session_id"`
	StartedAt time.Time    `json:"started_at"`
	Entries   []LogEntry   `json:"entries"`
}

// Session holds state for one clean/uninstall operation.
type Session struct {
	log     UndoLog
	logDir  string
}

// NewSession creates a new deletion session.
func NewSession(home string) *Session {
	logDir := filepath.Join(home, ".devclean", "undo")
	_ = os.MkdirAll(logDir, 0700)
	return &Session{
		logDir: logDir,
		log: UndoLog{
			SessionID: time.Now().Format("2006-01-02T15-04-05"),
			StartedAt: time.Now(),
		},
	}
}

// Remove deletes a path, using Trash when possible.
func (s *Session) Remove(path string, sizeBytes int64) error {
	method, err := moveToTrash(path)
	if err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	s.log.Entries = append(s.log.Entries, LogEntry{
		Path:      path,
		SizeBytes: sizeBytes,
		Method:    method,
		DeletedAt: time.Now(),
	})
	return nil
}

// Save writes the undo log to disk.
func (s *Session) Save() error {
	if len(s.log.Entries) == 0 {
		return nil
	}
	name := filepath.Join(s.logDir, s.log.SessionID+".json")
	data, err := json.MarshalIndent(s.log, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, data, 0600)
}

// Count returns the number of items deleted this session.
func (s *Session) Count() int { return len(s.log.Entries) }

// TotalSize returns bytes freed this session.
func (s *Session) TotalSize() int64 {
	var total int64
	for _, e := range s.log.Entries {
		total += e.SizeBytes
	}
	return total
}

// moveToTrash moves the path to the macOS Trash via AppleScript.
// Falls back to os.RemoveAll for paths that can't be trashed (e.g. /Library/*).
func moveToTrash(path string) (DeletionMethod, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	script := fmt.Sprintf(`tell application "Finder" to delete POSIX file %q`, absPath)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err == nil {
		return MethodTrash, nil
	}

	// Fallback: hard delete
	if err := os.RemoveAll(absPath); err != nil {
		return "", err
	}
	return MethodDelete, nil
}

// LoadLastLog loads the most recent undo log from disk.
func LoadLastLog(home string) (*UndoLog, error) {
	logDir := filepath.Join(home, ".devclean", "undo")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, fmt.Errorf("no undo log found")
	}

	var latest string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			latest = filepath.Join(logDir, e.Name())
		}
	}
	if latest == "" {
		return nil, fmt.Errorf("no undo log found")
	}

	data, err := os.ReadFile(latest)
	if err != nil {
		return nil, err
	}
	var log UndoLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, err
	}
	return &log, nil
}
