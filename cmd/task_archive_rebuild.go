package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func init() {
	taskArchiveCmd.AddCommand(newTaskArchiveRebuildIndexCmd())
}

func newTaskArchiveRebuildIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebuild-index",
		Short: "Rebuild the archive summary index",
		Long: `Rebuild the derived archive summary index from completed.md and completed.N.md.

The archive markdown files remain canonical. This command repairs missing,
stale, or corrupt .completed.index.json files without changing tasks.md.

Examples:
  tt task archive rebuild-index
  tt task archive rebuild-index --project my-project
  tt task archive rebuild-index --all
  tt task archive rebuild-index --json`,
		Args: cobra.NoArgs,
		RunE: runTaskArchiveRebuildIndex,
	}

	cmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")
	cmd.Flags().Bool("all", false, "rebuild archive indexes for all projects")
	addOutputFlags(cmd)

	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectSlugs())

	return cmd
}

type archiveRebuildIndexResult struct {
	Project     string `json:"project"`
	Slug        string `json:"slug"`
	IndexPath   string `json:"index_path"`
	SourceCount int    `json:"source_count"`
	TaskCount   int    `json:"task_count"`
	Rebuilt     bool   `json:"rebuilt"`
	Error       string `json:"error,omitempty"`
}

type archiveRebuildIndexSummary struct {
	TotalProjects int                         `json:"total_projects"`
	Rebuilt       int                         `json:"rebuilt"`
	Projects      []archiveRebuildIndexResult `json:"projects"`
}

func runTaskArchiveRebuildIndex(cmd *cobra.Command, _ []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	all, _ := cmd.Flags().GetBool("all")
	projectRef, _ := cmd.Flags().GetString("project")
	if all && projectRef != "" {
		return fmt.Errorf("--all and --project are mutually exclusive")
	}

	if all {
		return runTaskArchiveRebuildIndexAll(out)
	}

	project, _, err := loadArchiveAccessProject(cmd)
	if err != nil {
		return err
	}

	result, err := rebuildArchiveIndexForProject(project)
	if err != nil {
		return fmt.Errorf("failed to rebuild archive index for %s: %w", project.Name, err)
	}

	if out.JSON {
		return printJSON(result)
	}
	if !out.Quiet {
		printArchiveRebuildIndexResult(result)
	}
	return nil
}

func runTaskArchiveRebuildIndexAll(out MutateOutput) error {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	projects := archiveIndexRepairProjects(workspace.Runtime.AllProjects())
	results := make([]archiveRebuildIndexResult, 0, len(projects))
	failures := 0
	rebuilt := 0

	for _, project := range projects {
		result, err := rebuildArchiveIndexForProject(project)
		if err != nil {
			result.Project = project.Name
			result.Slug = project.Slug
			result.Error = err.Error()
			failures++
		} else {
			rebuilt++
		}
		results = append(results, result)
	}

	summary := archiveRebuildIndexSummary{
		TotalProjects: len(projects),
		Rebuilt:       rebuilt,
		Projects:      results,
	}

	if out.JSON {
		if err := printJSON(summary); err != nil {
			return err
		}
		if failures > 0 {
			return fmt.Errorf("%s failed during archive index rebuild", Pluralize(failures, "project", "projects"))
		}
		return nil
	}

	if !out.Quiet {
		for _, result := range results {
			if result.Error != "" {
				fmt.Printf("[%s] Archive index rebuild failed: %s\n", result.Project, result.Error)
				continue
			}
			printArchiveRebuildIndexResult(result)
		}
		fmt.Printf("\nTotal: rebuilt archive indexes for %s\n", Pluralize(rebuilt, "project", "projects"))
	}

	if failures > 0 {
		return fmt.Errorf("%s failed during archive index rebuild", Pluralize(failures, "project", "projects"))
	}
	return nil
}

func archiveIndexRepairProjects(all []*models.Project) []*models.Project {
	projects := make([]*models.Project, 0, len(all))
	for _, project := range all {
		if project == nil || project.Sandbox {
			continue
		}
		projects = append(projects, project)
	}
	return projects
}

func rebuildArchiveIndexForProject(project *models.Project) (archiveRebuildIndexResult, error) {
	result := archiveRebuildIndexResult{
		Project: project.Name,
		Slug:    project.Slug,
		Rebuilt: true,
	}

	if err := storage.RebuildArchiveIndex(project); err != nil {
		return result, err
	}

	status, err := storage.InspectArchiveIndex(project)
	if err != nil {
		return result, err
	}
	result.IndexPath = status.IndexPath
	result.SourceCount = status.SourceCount
	result.TaskCount = status.TaskCount
	if status.NeedsRebuild {
		return result, fmt.Errorf("rebuilt archive index is still %s: %s", status.Issue, status.Detail)
	}

	return result, nil
}

func printArchiveRebuildIndexResult(result archiveRebuildIndexResult) {
	prefix := ""
	if result.Project != "" {
		prefix = fmt.Sprintf("[%s] ", result.Project)
	}
	fmt.Printf("%sRebuilt archive index: %s from %s\n",
		prefix,
		Pluralize(result.TaskCount, "task", "tasks"),
		Pluralize(result.SourceCount, "archive file", "archive files"),
	)
}
