package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/storage"
)

var sandboxResetForce bool

var sandboxResetCmd = &cobra.Command{
	Use:   "sandbox reset",
	Short: "Clear all tasks, notes, and tag metadata from sandbox",
	Long: `Clear all tasks, notes, tags, and pinned tags from the sandbox project.

This resets the sandbox to a clean state by removing all tasks, notes, and
tag metadata, then resetting to the default "Next" section only.

An undo snapshot is recorded before clearing, allowing task restoration via
'tt undo'. Note that notes and tag metadata are not restored by undo.

Use --force to skip the confirmation prompt.

Examples:
  tt sandbox reset          # Reset with confirmation
  tt sandbox reset --force  # Reset without confirmation`,
	RunE: runSandboxReset,
}

func init() {
	rootCmd.AddCommand(sandboxResetCmd)
	sandboxResetCmd.Flags().BoolVarP(&sandboxResetForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(sandboxResetCmd)
}

func runSandboxReset(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	sandbox := workspace.Runtime.GetSandboxProject()
	if sandbox == nil {
		return fmt.Errorf("sandbox project not found")
	}

	if err := storage.LoadProject(sandbox); err != nil {
		return fmt.Errorf("failed to load sandbox: %w", err)
	}

	taskCount := len(sandbox.Tasks)
	notes, _ := storage.ListNotes(sandbox)
	noteCount := len(notes)
	tagCount := len(sandbox.AllTags())
	pinnedTagCount := len(sandbox.PinnedTagList())

	if taskCount == 0 && noteCount == 0 && tagCount == 0 && pinnedTagCount == 0 {
		if !out.Quiet {
			fmt.Println("Sandbox is already empty.")
		}
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
	if tagCount > 0 {
		parts = append(parts, Pluralize(tagCount, "tag", "tags"))
	}
	if pinnedTagCount > 0 {
		parts = append(parts, Pluralize(pinnedTagCount, "pinned tag", "pinned tags"))
	}
	confirmMsg := fmt.Sprintf("Clear %s from sandbox? [y/N] ", joinWithAnd(parts))

	if !ConfirmYN(confirmMsg, sandboxResetForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Record a legacy per-project undo snapshot before clearing so `tt undo`
	// (which still reads the per-project history) can restore the tasks; the
	// app-layer commit below additionally records the wipe in the workspace
	// ledger.
	settings := workspace.Runtime.Settings()
	undoDepth := 0
	if settings.UndoHistoryDepth != nil {
		undoDepth = *settings.UndoHistoryDepth
	}
	if err := storage.RecordUndoSnapshot(sandbox, "reset sandbox", "", undoDepth, nil); err != nil {
		return fmt.Errorf("failed to record undo snapshot: %w", err)
	}

	// The wipe itself (tasks, short-ID counter, sections, tags, pinned tags)
	// commits through the application layer as one transaction.
	svc := newWorkspaceAppService(workspace)
	if _, err := withResultRetry(func() (app.ProjectResetResult, error) {
		return svc.ResetProjectTasks(sandbox.ID)
	}); err != nil {
		return translateAppError(err)
	}

	// Clear notes directory
	if noteCount > 0 {
		notesDir := storage.NotesDir(sandbox)
		if err := os.RemoveAll(notesDir); err != nil {
			return fmt.Errorf("failed to clear notes: %w", err)
		}
	}

	// Build output message
	var cleared []string
	if taskCount > 0 {
		cleared = append(cleared, Pluralize(taskCount, "task", "tasks"))
	}
	if noteCount > 0 {
		cleared = append(cleared, Pluralize(noteCount, "note", "notes"))
	}
	if tagCount > 0 {
		cleared = append(cleared, Pluralize(tagCount, "tag", "tags"))
	}
	if pinnedTagCount > 0 {
		cleared = append(cleared, Pluralize(pinnedTagCount, "pinned tag", "pinned tags"))
	}
	if !out.Quiet {
		fmt.Printf("Cleared %s from sandbox.\n", joinWithAnd(cleared))
	}
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
