package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/spf13/cobra"
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
}

func runProjectPurge(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
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
		return fmt.Errorf("project '%s' not found in trash", slug)
	}

	// Confirmation unless --force
	if !purgeForce {
		fmt.Printf("PERMANENTLY delete '%s'? This cannot be undone.\n", slug)
		fmt.Print("Type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Purge the project
	if err := storage.PurgeFromTrash(slug, cfg.DataDir); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	fmt.Printf("Permanently deleted '%s'\n", slug)
	return nil
}
