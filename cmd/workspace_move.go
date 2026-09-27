package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

var workspaceMoveForce bool

// errWorkspaceMoveNotConfirmed is an unanswered prompt: stdin closed or not a
// terminal. It is an error rather than a quiet success so a script that piped
// no answer learns the workspace did not move. An explicit "no" is the
// operator's own choice and is not an error; see workspaceMoveCanceled.
var errWorkspaceMoveNotConfirmed = errors.New("move not confirmed: no answer was read; re-run with --force to skip the prompt")

// workspaceMoveCanceled is what a declined prompt prints before exiting 0.
const workspaceMoveCanceled = "Move canceled; nothing changed."

var workspaceMoveCmd = &cobra.Command{
	Use:   "move <destination>",
	Short: "Move the workspace to a new data directory",
	Long: `Relocate the workspace's durable data to a new directory using a staged,
verified flow: inventory, copy with per-file hash verification and fsync, then
an atomic switch of the machine-local active-workspace pointer.

The source directory is retained as a backup by default; it is never deleted, so
an interrupted move can simply be re-run (already-copied files are skipped).
Machine-local runtime state (logs, locks) is not moved.

Close any tt TUI using either the source or destination before relocating. An
active TUI blocks the move. --force skips only the confirmation prompt; it does
not bypass TUI process locks, workspace commit locks, or snapshot validation.`,
	Example: `  tt workspace move ~/Documents/tt
  tt workspace move /Volumes/Backup/tt --force`,
	Args: exactlyOneArg("destination directory"),
	RunE: runWorkspaceMove,
	ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveFilterDirs
	},
}

func init() {
	workspaceMoveCmd.Flags().BoolVarP(&workspaceMoveForce, "force", "f", false, "skip confirmation only; never bypass TUI or workspace safety locks")
}

func runWorkspaceMove(_ *cobra.Command, args []string) error {
	dest, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("invalid destination %q: %w", args[0], err)
	}

	if err = checkDataDirOverride(); err != nil {
		return err
	}
	manager := config.NewManager(resolveConfigPath())
	src := config.WorkspaceRootFromConfigPath(manager.GetConfigPath())

	// A pair no move could accept is refused before the prompt, so declining
	// is never the only way to learn the destination was wrong.
	if err = config.ValidateWorkspaceMovePaths(src, dest); err != nil {
		return err
	}

	fmt.Printf("Move workspace:\n  from: %s\n  to:   %s\n", config.DisplayPath(src), config.DisplayPath(dest))
	switch confirmPrompt("Proceed? The source is retained as a backup. [y/N]: ", workspaceMoveForce) {
	case confirmNoAnswer:
		return errWorkspaceMoveNotConfirmed
	case confirmDeclined:
		fmt.Println(workspaceMoveCanceled)
		return nil
	case confirmAccepted:
	}
	// Moving is a live current-schema operation. Activate or bootstrap the source
	// workspace before inventory so no legacy live config is copied forward.
	if _, loadErr := manager.LoadOrCreateDomains(); loadErr != nil {
		return fmt.Errorf("prepare source workspace for move: %w", loadErr)
	}

	report, err := config.MoveWorkspace(src, dest)
	if err != nil {
		return err
	}

	fmt.Println(successMsg("Workspace moved to %s", config.DisplayPath(report.Destination)))
	fmt.Printf("  %s copied, %s unchanged, %s created\n",
		Pluralize(report.FilesCopied, "file", "files"),
		Pluralize(report.FilesSkipped, "file", "files"),
		Pluralize(report.DirsCreated, "directory", "directories"))
	if report.SourceRetained {
		fmt.Printf("  Source retained at %s (remove it manually once you have verified the move)\n", config.DisplayPath(report.Source))
	}
	return nil
}
