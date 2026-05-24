package commands

import (
	"fmt"
	"os"

	"github.com/maharabhossain1/devclean/cleaner"
	"github.com/spf13/cobra"
)

func newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "Restore items from the last cleanup session",
		RunE:  runUndo,
	}
}

func runUndo(_ *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	log, err := cleaner.LoadLastLog(home)
	if err != nil {
		return err
	}

	fmt.Printf("Last session: %s  (%d items)\n\n", log.StartedAt.Format("2006-01-02 15:04:05"), len(log.Entries))
	for _, e := range log.Entries {
		fmt.Printf("  %s  (%s)\n", shortenHomePath(e.Path), formatBytes(e.SizeBytes))
	}

	fmt.Println()
	fmt.Println("Note: Items moved to Trash can be restored from the macOS Trash.")
	fmt.Println("Items hard-deleted cannot be recovered.")
	fmt.Println()

	var trashed, deleted int
	for _, e := range log.Entries {
		if e.Method == cleaner.MethodTrash {
			trashed++
		} else {
			deleted++
		}
	}

	if trashed > 0 {
		fmt.Printf("  %d items are in the Trash — open Finder and restore them manually.\n", trashed)
	}
	if deleted > 0 {
		fmt.Printf("  %d items were hard-deleted and cannot be recovered.\n", deleted)
	}
	return nil
}
