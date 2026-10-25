package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

// Shell completion reads the workspace without LoadCLIWorkspace, which runs
// recovery and creates a workspace that does not exist yet: a Tab press must
// write nothing. Every failure completes nothing, quietly.

// completionDataDir resolves the data directory the command would use,
// honoring --data-dir and the active-workspace pointer.
func completionDataDir() string {
	manager := config.NewManager(resolveConfigPath())
	return config.WorkspaceRootFromConfigPath(manager.GetConfigPath())
}

// completeProjectSlugs offers every cataloged project's slug, archived
// included, described by its name.
func completeProjectSlugs(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	// --all exports every project and refuses a named one, so offer none.
	if cmd != nil {
		if all, err := cmd.Flags().GetBool("all"); err == nil && all {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	catalog, err := config.NewDomainFileStore(completionDataDir()).LoadCatalogReadOnly()
	if err != nil || catalog == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []cobra.Completion
	for _, project := range catalog.Projects {
		if strings.HasPrefix(project.Slug, toComplete) {
			out = append(out, cobra.CompletionWithDesc(project.Slug, project.Name))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeConflictIDs offers the short conflict IDs `docket conflict list` prints,
// described by the conflicted project's name.
func completeConflictIDs(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	records, err := storage.NewConflictStore(config.ConflictsDir(completionDataDir())).List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []cobra.Completion
	for _, record := range records {
		id := shortConflictID(record.ID)
		if strings.HasPrefix(id, toComplete) {
			out = append(out, cobra.CompletionWithDesc(id, record.ProjectName))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
