package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/maharabhossain1/devclean/cleaner"
	"github.com/maharabhossain1/devclean/engine"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <app>",
		Short: "Remove an app and all its traces in one shot",
		Args:  cobra.ExactArgs(1),
		RunE:  runUninstall,
	}
}

func runUninstall(cmd *cobra.Command, args []string) error {
	appName := args[0]
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fmt.Printf("Scanning for %q and its traces…\n\n", appName)

	// Build registry + dictionary to find traces
	dict, err := engine.LoadDictionary()
	if err != nil {
		return err
	}
	apps, err := engine.BuildAppRegistry()
	if err != nil {
		return err
	}

	// Find the app in the registry
	var target *engine.AppEntry
	for i, a := range apps {
		if strings.EqualFold(a.Name, appName) ||
			strings.EqualFold(strings.ToLower(a.Name), strings.ToLower(appName)) {
			target = &apps[i]
			break
		}
	}

	// Collect all traces for this app from the dictionary
	traces := findTracesForApp(appName, dict, home)

	if target == nil && len(traces) == 0 {
		return fmt.Errorf("no app or traces found for %q", appName)
	}

	// Print what we found
	var items []string
	var totalSize int64

	if target != nil {
		size := target.SizeBytes
		totalSize += size
		items = append(items, fmt.Sprintf("  %-56s %s  (app bundle)", target.InstallPath, formatBytes(size)))
	}

	for _, t := range traces {
		if _, err := os.Stat(t.Path); err != nil {
			continue // skip missing paths
		}
		items = append(items, fmt.Sprintf("  %-56s %s", shortenHomePath(t.Path), formatBytes(t.SizeBytes)))
		totalSize += t.SizeBytes
	}

	if len(items) == 0 {
		fmt.Printf("No files found for %q on disk.\n", appName)
		return nil
	}

	fmt.Println("Items to remove:")
	for _, item := range items {
		fmt.Println(item)
	}
	fmt.Printf("\nTotal: %d items  (%s)\n\n", len(items), formatBytes(totalSize))

	if !confirm(fmt.Sprintf("Remove all %d items?", len(items))) {
		fmt.Println("Cancelled.")
		return nil
	}

	session := cleaner.NewSession(home)

	// Uninstall the app bundle first
	if target != nil {
		if err := uninstallApp(target); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove app bundle: %v\n", err)
		}
	}

	// Remove all traces
	var removed, failed int
	for _, t := range traces {
		if _, err := os.Stat(t.Path); err != nil {
			continue
		}
		if err := session.Remove(t.Path, t.SizeBytes); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", t.Path, err)
			failed++
		} else {
			fmt.Printf("  ✓ %s\n", shortenHomePath(t.Path))
			removed++
		}
	}

	_ = session.Save()

	fmt.Printf("\nRemoved %d items. %d failed.\n", removed, failed)
	if removed > 0 {
		fmt.Println("Undo log saved. Run `devclean undo` to restore.")
	}
	return nil
}

func findTracesForApp(appName string, dict engine.AppDictionary, home string) []engine.Trace {
	lower := strings.ToLower(appName)
	var traces []engine.Trace

	for key, entry := range dict {
		if strings.ToLower(key) != lower &&
			!strings.Contains(strings.ToLower(key), lower) &&
			!strings.Contains(lower, strings.ToLower(key)) {
			continue
		}
		for _, pattern := range entry.Traces {
			path := expandHome(pattern, home)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			t := engine.Trace{
				Path:     path,
				Status:   engine.StatusOrphaned,
				Risk:     engine.RiskGreen,
				Category: "trace",
			}
			engine.ScoreTrace(&t)
			traces = append(traces, t)
		}
	}
	return traces
}

func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return home + path[1:]
	}
	return path
}

func uninstallApp(app *engine.AppEntry) error {
	switch app.Source {
	case engine.SourceBrewCask:
		out, err := exec.Command("brew", "uninstall", "--cask", app.Name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("brew uninstall: %s", out)
		}
	case engine.SourceBrew:
		out, err := exec.Command("brew", "uninstall", app.Name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("brew uninstall: %s", out)
		}
	default:
		// Move .app to trash via AppleScript
		script := fmt.Sprintf(`tell application "Finder" to delete POSIX file %q`, app.InstallPath)
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			return os.RemoveAll(app.InstallPath)
		}
	}
	return nil
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}
