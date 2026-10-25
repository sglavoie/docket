package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sglavoie/docket/internal/config"
)

var workspaceBackupCmd = &cobra.Command{
	Use:   "backup <backup-directory>",
	Short: "Create a verified workspace backup",
	Long: `Copy the current workspace into a new timestamped backup below the
given directory. The backup includes task files, attachments, undo history, and
sync state. Docket never overwrites a finished backup. It verifies every copied
file and writes a hash manifest. The live workspace remains selected. Choose a
directory outside the workspace, ideally on another drive. Run
'docket workspace verify-backup' later to read every file again without touching
the live workspace.`,
	Example: `  docket workspace backup ~/Backups/tt
  docket workspace verify-backup ~/Backups/tt/<workspace-id>/<timestamp>`,
	Args:              exactlyOneArg("backup directory"),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(_ *cobra.Command, args []string) error {
		if err := requireExistingWorkspace(); err != nil {
			return err
		}
		root, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		manager := config.NewManager(resolveConfigPath())
		source := config.WorkspaceRootFromConfigPath(manager.GetConfigPath())
		report, err := config.CreateWorkspaceBackup(source, root)
		if err != nil {
			return err
		}
		fmt.Printf("Backup verified: %s (%d files)\n", config.DisplayPath(report.Path), report.Files)
		fmt.Printf("Verify again: docket workspace verify-backup %q\n", report.Path)
		return nil
	},
}

var workspaceVerifyBackupCmd = &cobra.Command{
	Use:   "verify-backup <snapshot-directory>",
	Short: "Check every file in a workspace backup",
	Long: `Read a backup's manifest and every copied file, verify their hashes, and
reject missing, unexpected, or unsafe files. This does not open or change the
current workspace. A verified backup records its original workspace path;
restore only after checking the recovery procedure for an enrolled workspace.`,
	Example:           `  docket workspace verify-backup ~/Backups/tt/<workspace-id>/<timestamp>`,
	Args:              exactlyOneArg("snapshot directory"),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(_ *cobra.Command, args []string) error {
		report, err := config.VerifyWorkspaceBackup(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Backup verified: %s (%d files, created %s)\n", config.DisplayPath(report.Path), report.Files, report.CreatedAt.Local().Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("Original workspace: %s\n", config.DisplayPath(report.Source))
		return nil
	},
}
