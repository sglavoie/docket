package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
)

var sandboxResetForce bool

var sandboxResetCmd = &cobra.Command{
	Use:   "sandbox reset",
	Short: "Clear all tasks, notes, and tag metadata from sandbox",
	Long: `Clear all tasks, notes, tags, and pinned tags from the sandbox project.

This resets the sandbox to a clean state by removing all tasks, notes, and tag
metadata. The reset commits
as one workspace transaction, so 'tt undo' restores the complete prior state.

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

	svc := newWorkspaceAppService(workspace)
	preview, err := svc.PreviewSandboxReset(sandbox.ID)
	if err != nil {
		return translateAppError(err)
	}
	if preview.Counts.Empty() {
		if !out.Quiet {
			fmt.Println("Sandbox is already empty.")
		}
		return nil
	}

	// Build confirmation message
	parts := sandboxResetParts(preview.Counts)
	confirmMsg := fmt.Sprintf("Clear %s from sandbox? [y/N] ", joinWithAnd(parts))

	if !ConfirmYN(confirmMsg, sandboxResetForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	result, err := withResultRetry(func() (app.SandboxResetResult, error) {
		return svc.ResetSandbox(sandbox.ID)
	})
	if err != nil {
		return translateAppError(err)
	}

	if !out.Quiet {
		fmt.Printf("Cleared %s from sandbox.\n", joinWithAnd(sandboxResetParts(result.Cleared)))
	}
	return nil
}

func sandboxResetParts(counts app.SandboxResetCounts) []string {
	var parts []string
	if counts.Tasks > 0 {
		parts = append(parts, Pluralize(counts.Tasks, "task", "tasks"))
	}
	if counts.Notes > 0 {
		parts = append(parts, Pluralize(counts.Notes, "note", "notes"))
	}
	if counts.Tags > 0 {
		parts = append(parts, Pluralize(counts.Tags, "tag", "tags"))
	}
	if counts.PinnedTags > 0 {
		parts = append(parts, Pluralize(counts.PinnedTags, "pinned tag", "pinned tags"))
	}
	if counts.ShortIDCounter > 0 {
		parts = append(parts, "short-ID counter")
	}
	return parts
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
