package engine

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/maharabhossain1/devclean/scanners/dotfiles"
	"github.com/maharabhossain1/devclean/scanners/library"
)

// RawTrace is a path found during scanning before correlation.
type RawTrace struct {
	Path     string
	Category string
}

// ScanTraces concurrently scans all known trace locations.
func ScanTraces(home string) ([]RawTrace, error) {
	var mu sync.Mutex
	var all []RawTrace
	var wg sync.WaitGroup
	var scanErr error

	add := func(traces []library.Trace) {
		mu.Lock()
		defer mu.Unlock()
		for _, t := range traces {
			all = append(all, RawTrace{Path: t.Path, Category: t.Category})
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		traces, err := library.ScanAll(home)
		if err != nil {
			mu.Lock()
			scanErr = err
			mu.Unlock()
			return
		}
		add(traces)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		traces, err := dotfiles.Scan(home)
		if err != nil {
			return
		}
		add(traces)
	}()

	// Hidden system dirs
	wg.Add(1)
	go func() {
		defer wg.Done()
		systemDirs := []string{
			"/Library/LaunchDaemons",
		}
		var traces []library.Trace
		for _, dir := range systemDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if filepath.Ext(e.Name()) != ".plist" {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				traces = append(traces, library.Trace{
					Path:      filepath.Join(dir, e.Name()),
					SizeBytes: info.Size(),
					ModTime:   info.ModTime(),
					Category:  "launch_daemon",
				})
			}
		}
		add(traces)
	}()

	wg.Wait()
	return all, scanErr
}
