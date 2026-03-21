package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var sandboxResetForce bool

var sandboxResetCmd = &cobra.Command{
	Use:   "sandbox reset",
	Short: "Clear all tasks, notes, and custom sections from sandbox",
	Long: `Clear all tasks, notes, and custom sections from the sandbox project.

This resets the sandbox to a clean state by removing all tasks, notes, and
custom sections (resetting to default "Next" section only).

An undo snapshot is recorded before clearing, allowing task restoration via
'tt undo'. Note that notes and sections are not restored by undo.

Use --force to skip the confirmation prompt.

Examples:
  tt sandbox reset          # Reset with confirmation
  tt sandbox reset --force  # Reset without confirmation`,
	RunE: runSandboxReset,
}

func init() {
	rootCmd.AddCommand(sandboxResetCmd)
	sandboxResetCmd.Flags().BoolVar(&sandboxResetForce, "force", false, "Skip confirmation prompt")
}

func runSandboxReset(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	sandbox := cfg.GetSandboxProject()
	if sandbox == nil {
		return fmt.Errorf("sandbox project not found")
	}

	if err := storage.LoadProject(sandbox); err != nil {
		return fmt.Errorf("failed to load sandbox: %w", err)
	}

	taskCount := len(sandbox.Tasks)
	notes, _ := storage.ListNotes(sandbox)
	noteCount := len(notes)
	customSections := sandbox.GetCustomSections()
	sectionCount := len(customSections)

	if taskCount == 0 && noteCount == 0 && sectionCount == 0 {
		fmt.Println("Sandbox is already empty.")
		return nil
	}

	// Build confirmation message
	var parts []string
	if taskCount > 0 {
		parts = append(parts, Pluralize(taskCount, "task", "tasks"))
	}
	if noteCount > 0 {
		parts = append(parts, Pluralize(noteCount, "note", "notes"))
	}
	if sectionCount > 0 {
		parts = append(parts, Pluralize(sectionCount, "custom section", "custom sections"))
	}
	confirmMsg := fmt.Sprintf("Clear %s from sandbox? [y/N] ", joinWithAnd(parts))

	if !ConfirmYN(confirmMsg, sandboxResetForce) {
		fmt.Println("Canceled")
		return nil
	}

	// Record undo snapshot before clearing
	undoDepth := 0
	if cfg.UndoHistoryDepth != nil {
		undoDepth = *cfg.UndoHistoryDepth
	}
	if err := storage.RecordUndoSnapshot(sandbox, "reset sandbox", "", undoDepth); err != nil {
		return fmt.Errorf("failed to record undo snapshot: %w", err)
	}

	// Clear tasks and reset NextShortID
	sandbox.Tasks = []*models.Task{}
	sandbox.NextShortID = 1

	// Reset sections to default
	sandbox.Sections = []string{models.SectionNext}
	sandbox.SectionColors = make(map[string]string)

	if err := SaveProject(sandbox, &SaveOptions{Config: cfg, ConfigManager: configManager}); err != nil {
		return fmt.Errorf("failed to save sandbox: %w", err)
	}

	// Clear notes directory
	if noteCount > 0 {
		notesDir := storage.NotesDir(sandbox)
		if err := os.RemoveAll(notesDir); err != nil {
			return fmt.Errorf("failed to clear notes: %w", err)
		}
	}

	// Update project in config (for NextShortID persistence)
	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	// Build output message
	var cleared []string
	if taskCount > 0 {
		cleared = append(cleared, Pluralize(taskCount, "task", "tasks"))
	}
	if noteCount > 0 {
		cleared = append(cleared, Pluralize(noteCount, "note", "notes"))
	}
	if sectionCount > 0 {
		cleared = append(cleared, Pluralize(sectionCount, "custom section", "custom sections"))
	}
	fmt.Printf("Cleared %s from sandbox.\n", joinWithAnd(cleared))
	return nil
}

// joinWithAnd joins strings with commas and "and" for the last item.
func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return fmt.Sprintf("%s, and %s", joinWithComma(parts[:len(parts)-1]), parts[len(parts)-1])
	}
}

func joinWithComma(parts []string) string {
	var result strings.Builder
	for i, p := range parts {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(p)
	}
	return result.String()
}
