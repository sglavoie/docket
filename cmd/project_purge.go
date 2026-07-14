package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tterrors"
)

var purgeForce bool

var projectPurgeCmd = &cobra.Command{
	Use:   "purge [slug]",
	Short: "Permanently delete a project from trash",
	Long: `Permanently delete a project from the trash folder.

This action is IRREVERSIBLE. The project data will be permanently deleted.

If no slug is provided, lists all projects in trash.
Use --force to skip the confirmation prompt.

Examples:
  tt project purge              # List trashed projects
  tt project purge my-project   # Purge with confirmation
  tt project purge my-project --force  # Purge without confirmation`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectPurge,
}

func init() {
	projectCmd.AddCommand(projectPurgeCmd)
	projectPurgeCmd.Flags().BoolVarP(&purgeForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(projectPurgeCmd)
}

func runProjectPurge(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// If no args, list trashed projects
	if len(args) == 0 {
		return listTrashedProjects(workspace.Runtime.DataDir())
	}

	ref := args[0]
	if ref == "" {
		return tterrors.NewRequiredFieldError("project slug")
	}

	// Resolve the trash entry, requiring an explicit entry ID when more than one
	// deleted generation shares a slug.
	entry, err := resolveTrashEntry(workspace.Runtime.DataDir(), ref)
	if err != nil {
		return err
	}

	// Confirmation unless --force
	if !ConfirmStrict(fmt.Sprintf("PERMANENTLY delete '%s'? This cannot be undone.", entry.Slug), purgeForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Route the purge through the application layer: the trash entry's whole subtree
	// is removed as ONE atomic journaled transaction (files first, then directories
	// deepest-first, all contained within the approved roots). This is irreversible.
	svc := newWorkspaceAppService(workspace)
	if _, err := svc.PurgeProject(app.PurgeProjectRequest{
		DataDir: workspace.Runtime.DataDir(),
		EntryID: entry.EntryID,
		Slug:    entry.Slug,
		Confirm: true,
	}); err != nil {
		return translateAppError(err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			Slug    string `json:"slug"`
			EntryID string `json:"entry_id"`
			Purged  bool   `json:"purged"`
		}{Slug: entry.Slug, EntryID: entry.EntryID, Purged: true})
	}
	fmt.Printf("Permanently deleted '%s'\n", entry.Slug)
	return nil
}
