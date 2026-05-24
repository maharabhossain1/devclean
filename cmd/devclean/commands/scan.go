package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/maharabhossain1/devclean/engine"
	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Full system audit — ranked report, no changes",
		RunE:  runScan,
	}
}

func runScan(cmd *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "Scanning… (this may take a few seconds)")

	start := time.Now()
	result, err := engine.RunScan(home)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	elapsed := time.Since(start)

	printReport(result, elapsed)
	return nil
}

func printReport(r *engine.ScanResult, elapsed time.Duration) {
	totalRecoverable := int64(0)
	for _, t := range r.Orphans {
		totalRecoverable += t.SizeBytes
	}
	for _, t := range r.DevCaches {
		totalRecoverable += t.SizeBytes
	}

	fmt.Printf("\nDevClean | Scan complete in %.0fs | Recoverable: %s\n\n",
		elapsed.Seconds(), fmtSize(totalRecoverable))

	printOrphans(r.Orphans)
	printDeadAgents(r.DeadAgents)
	printUnknowns(r.Unknowns)
	printUnusedApps(r.UnusedApps)
	printDevCaches(r.DevCaches)
}

func printOrphans(traces []engine.Trace) {
	if len(traces) == 0 {
		return
	}
	sort.Slice(traces, func(i, j int) bool {
		return traces[i].SizeBytes > traces[j].SizeBytes
	})
	fmt.Printf("ORPHANED APP TRACES                                    SIZE    AGE     RISK\n")
	fmt.Printf("──────────────────────────────────────────────────────────────────────────\n")
	for _, t := range traces {
		marker := riskMarker(t.Risk)
		fmt.Printf("%s %-52s %-7s %-7s %s\n",
			marker,
			shortenPath(t.Path),
			fmtSize(t.SizeBytes),
			fmtAge(t.ModTime),
			strings.Title(string(t.Risk)),
		)
	}
	fmt.Println()
}

func printDeadAgents(traces []engine.Trace) {
	if len(traces) == 0 {
		return
	}
	fmt.Printf("DEAD LAUNCH AGENTS (binary missing)                   SIZE    AGE     RISK\n")
	fmt.Printf("──────────────────────────────────────────────────────────────────────────\n")
	for _, t := range traces {
		marker := riskMarker(t.Risk)
		fmt.Printf("%s %-52s %-7s %-7s %s\n",
			marker,
			shortenPath(t.Path),
			fmtSize(t.SizeBytes),
			fmtAge(t.ModTime),
			strings.Title(string(t.Risk)),
		)
	}
	fmt.Println()
}

func printUnknowns(traces []engine.Trace) {
	if len(traces) == 0 {
		return
	}
	sort.Slice(traces, func(i, j int) bool {
		return traces[i].SizeBytes > traces[j].SizeBytes
	})
	fmt.Printf("UNKNOWN INSTALLS (no registered owner)                SIZE    AGE     RISK\n")
	fmt.Printf("──────────────────────────────────────────────────────────────────────────\n")
	for _, t := range traces {
		marker := riskMarker(t.Risk)
		fmt.Printf("%s %-52s %-7s %-7s %s\n",
			marker,
			shortenPath(t.Path),
			fmtSize(t.SizeBytes),
			fmtAge(t.ModTime),
			strings.Title(string(t.Risk)),
		)
	}
	fmt.Println()
}

func printUnusedApps(apps []engine.AppEntry) {
	if len(apps) == 0 {
		return
	}
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].SizeBytes > apps[j].SizeBytes
	})
	fmt.Printf("UNUSED APPS (not launched in 90+ days)\n")
	fmt.Printf("──────────────────────────────────────\n")
	for _, a := range apps {
		lastUsed := "never"
		if !a.LastLaunch.IsZero() {
			lastUsed = a.LastLaunch.Format("2006-01-02")
		}
		fmt.Printf("  %-24s %-8s last used: %-12s via: %s\n",
			a.Name, fmtSize(a.SizeBytes), lastUsed, a.Source)
	}
	fmt.Println()
}

func printDevCaches(caches []engine.Trace) {
	if len(caches) == 0 {
		return
	}
	sort.Slice(caches, func(i, j int) bool {
		return caches[i].SizeBytes > caches[j].SizeBytes
	})
	fmt.Printf("DEV CACHES (regenerable)                              SIZE\n")
	fmt.Printf("──────────────────────────────────────────────────────────\n")
	for _, c := range caches {
		fmt.Printf("  %-52s %s\n", shortenPath(c.Path), fmtSize(c.SizeBytes))
	}
	fmt.Println()
}

func riskMarker(r engine.RiskLevel) string {
	switch r {
	case engine.RiskGreen:
		return "●"
	case engine.RiskYellow:
		return "▲"
	case engine.RiskRed:
		return "⚠"
	default:
		return " "
	}
}

func shortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func fmtSize(bytes int64) string {
	if bytes == 0 {
		return "—"
	}
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func fmtAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dwk", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dyr", int(d.Hours()/(24*365)))
	}
}
