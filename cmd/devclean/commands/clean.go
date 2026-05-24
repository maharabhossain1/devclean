package commands

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/maharabhossain1/devclean/engine"
	"github.com/maharabhossain1/devclean/tui"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var autoMode bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Interactive guided cleanup session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runClean(autoMode, dryRun)
		},
	}
	cmd.Flags().BoolVar(&autoMode, "auto", false, "Auto-remove all Green orphans without prompting")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted, touch nothing")
	return cmd
}

func runClean(autoMode, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "Scanning…")
	result, err := engine.RunScan(home)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if dryRun {
		return printDryRun(result)
	}

	total := len(result.Orphans) + len(result.DeadAgents) + len(result.Unknowns) + len(result.DevCaches)
	if total == 0 {
		fmt.Println("Nothing to clean — system is already tidy.")
		return nil
	}

	model := tui.New(result, home, autoMode)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func printDryRun(result *engine.ScanResult) error {
	fmt.Println("DRY RUN — nothing will be deleted\n")

	all := append(result.Orphans, result.DeadAgents...)
	all = append(all, result.Unknowns...)
	all = append(all, result.DevCaches...)

	var total int64
	for _, t := range all {
		marker := riskMarkerPlain(t.Risk)
		fmt.Printf("  %s %-56s %s\n", marker, shortenHomePath(t.Path), formatBytes(t.SizeBytes))
		total += t.SizeBytes
	}
	fmt.Printf("\nTotal recoverable: %s\n", formatBytes(total))
	return nil
}

func riskMarkerPlain(r engine.RiskLevel) string {
	switch r {
	case engine.RiskGreen:
		return "●"
	case engine.RiskYellow:
		return "▲"
	default:
		return "⚠"
	}
}

func shortenHomePath(path string) string {
	home, _ := os.UserHomeDir()
	if len(path) > len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}

func formatBytes(b int64) string {
	if b == 0 {
		return "—"
	}
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.0f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
