package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
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
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// If no args, list trashed projects
	if len(args) == 0 {
		return listTrashedProjects(cfg)
	}

	slug := args[0]

	// Check if project exists in trash
	if !storage.TrashEntryExists(slug, cfg.DataDir) {
		return tterrors.NewProjectNotFoundInTrashError(slug)
	}

	// Confirmation unless --force
	if !ConfirmStrict(fmt.Sprintf("PERMANENTLY delete '%s'? This cannot be undone.", slug), purgeForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Purge the project
	if err := storage.PurgeFromTrash(slug, cfg.DataDir); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		data, err := json.MarshalIndent(struct {
			Slug   string `json:"slug"`
			Purged bool   `json:"purged"`
		}{Slug: slug, Purged: true}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Permanently deleted '%s'\n", slug)
	return nil
}
