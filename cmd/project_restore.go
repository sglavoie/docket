package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var projectRestoreCmd = &cobra.Command{
	Use:   "restore [slug-or-entry-id]",
	Short: "Restore a deleted project from trash",
	Long: `Restore a previously deleted project from the trash folder.

If no argument is provided, lists all trashed generations.
When more than one deleted generation shares a slug, pass the specific
trash entry ID shown in the list to disambiguate.
Use 'tt project delete <slug-or-id>' to delete a project.

Examples:
  tt project restore                       # List trashed projects
  tt project restore my-project            # Restore by slug (must be unique)
  tt project restore my-project__17520...  # Restore a specific generation`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectRestore,
}

func init() {
	projectCmd.AddCommand(projectRestoreCmd)
	addOutputFlags(projectRestoreCmd)
}

func runProjectRestore(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
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

	ref := args[0]
	if ref == "" {
		return tterrors.NewRequiredFieldError("project slug")
	}

	entry, err := resolveTrashEntry(cfg.DataDir, ref)
	if err != nil {
		return err
	}

	// Check if active project with same slug exists
	if cfg.GetProject(entry.Slug) != nil {
		return fmt.Errorf("active project '%s' already exists; rename or delete it first", entry.Slug)
	}

	// Route the restore through the application layer: the trash-data directory move
	// back to projects/, the drained trash-entry removal, and the config rewrite (the
	// project re-added and its references re-applied) all commit as ONE atomic
	// journaled transaction. The config artifact is built from the identity the plan
	// reconstructs from the tombstone, so it commits atomically with the move; a
	// failure leaves the entry in trash and the config untouched (no manual rollback).
	svc := newAppService(cfg)
	res, err := svc.RestoreProject(app.RestoreProjectRequest{
		DataDir: cfg.DataDir,
		EntryID: entry.EntryID,
		BuildConfig: func(project *models.Project, tomb *storage.TrashTombstone) ([]storage.ArtifactChange, error) {
			cfg.AddProject(project)
			applyRestoredConfigRefs(cfg, project.ID, tomb)
			return encodeConfigArtifacts(configManager, cfg)
		},
	})
	if err != nil {
		return translateAppError(err)
	}

	return out.OutputProject(res.Project, func() { fmt.Println(successMsg("Restored project '%s'", res.Project.Name)) })
}

// resolveTrashEntry finds the single trash entry matching ref, requiring an
// explicit entry ID when more than one deleted generation shares a slug.
func resolveTrashEntry(dataDir, ref string) (storage.TrashEntry, error) {
	entries, err := storage.FindTrashEntries(dataDir, ref)
	if err != nil {
		return storage.TrashEntry{}, fmt.Errorf("failed to search trash: %w", err)
	}
	switch len(entries) {
	case 0:
		return storage.TrashEntry{}, tterrors.NewProjectNotFoundInTrashError(ref)
	case 1:
		return entries[0], nil
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "multiple trashed generations match '%s'; restore a specific one by its entry ID:\n", ref)
		for _, e := range entries {
			fmt.Fprintf(&b, "  %s  (%s, deleted %s)\n", e.EntryID, e.Name, storage.FormatRelativeTime(e.DeletedAt))
		}
		return storage.TrashEntry{}, fmt.Errorf("%s", strings.TrimRight(b.String(), "\n"))
	}
}

// applyRestoredConfigRefs decodes and re-applies the configuration references
// captured in the tombstone. Decode failures are non-fatal: the project data is
// already restored, so we skip reference restoration rather than abort.
func applyRestoredConfigRefs(cfg *config.Config, projectID string, tomb *storage.TrashTombstone) {
	if tomb == nil || len(tomb.ConfigRefs) == 0 {
		return
	}
	var refs config.ProjectReferences
	if err := json.Unmarshal(tomb.ConfigRefs, &refs); err != nil {
		return
	}
	cfg.ApplyProjectReferences(projectID, refs)
}

func listTrashedProjects(cfg *config.Config) error {
	entries, err := storage.ListTrashEntries(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to list trashed projects: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No projects in trash.")
		return nil
	}

	// Determine which slugs have more than one generation so we can hint that
	// the entry ID is required to disambiguate them.
	slugCounts := make(map[string]int, len(entries))
	for _, e := range entries {
		slugCounts[e.Slug]++
	}

	fmt.Println("Projects in trash:")
	for _, e := range entries {
		line := fmt.Sprintf("  %s  (deleted %s)", e.Slug, storage.FormatRelativeTime(e.DeletedAt))
		if slugCounts[e.Slug] > 1 {
			line += fmt.Sprintf("  [%s]", e.EntryID)
		}
		fmt.Println(line)
	}
	fmt.Println("\nTo restore: tt project restore <slug>")
	fmt.Println("For a slug with multiple generations, restore by the [entry-id] shown above.")
	return nil
}
