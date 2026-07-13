package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

var (
	archiveListType   string
	archiveListStatus string
	archiveListSince  string
	archiveListUntil  string
	archiveShowNoGlow bool
)

func init() {
	taskArchiveCmd.AddCommand(newTaskArchiveListCmd())
	taskArchiveCmd.AddCommand(newTaskArchiveShowCmd())
}

func newTaskArchiveListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List archived tasks",
		Long: `List tasks from the derived archive summary index.

The archive index is rebuilt from completed.md and completed.N.md when missing
or stale. Full archived task bodies are not parsed for list output.

Examples:
  tt task archive list
  tt task archive list --project my-project
  tt task archive list --since 2026-01-01 --until 2026-01-31
  tt task archive list --type bug --status done --json`,
		Args: cobra.NoArgs,
		RunE: runTaskArchiveList,
	}

	cmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")
	cmd.Flags().StringVarP(&archiveListType, "type", "t", "", "filter by type (epic, story, task, bug, idea)")
	cmd.Flags().StringVarP(&archiveListStatus, "status", "s", "", "filter by status (done, canceled)")
	cmd.Flags().StringVar(&archiveListSince, "since", "", "include tasks completed on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&archiveListUntil, "until", "", "include tasks completed on or before YYYY-MM-DD")
	addOutputFlags(cmd)

	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectSlugs())
	_ = cmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
	_ = cmd.RegisterFlagCompletionFunc("status", completeEnum(statusCompletions))

	return cmd
}

func newTaskArchiveShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id-or-prefix>",
		Short: "Show archived task details",
		Long: `Display a full archived task by short ID, exact ID, or ID prefix.

The lookup uses the archive summary index, then parses only the matched source
archive file for full task details.

Examples:
  tt task archive show 42
  tt task archive show a1b2c3d4
  tt task archive show a1b2c3d4 --project my-project
  tt task archive show a1b2c3d4 --json`,
		Args: cobra.ExactArgs(1),
		RunE: runTaskArchiveShow,
	}

	cmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")
	cmd.Flags().BoolVar(&archiveShowNoGlow, "no-glow", false, "disable glow rendering")
	addOutputFlags(cmd)

	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectSlugs())

	return cmd
}

func runTaskArchiveList(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, _, err := loadArchiveAccessProject(cmd)
	if err != nil {
		return err
	}

	filters, err := parseArchiveListFilters()
	if err != nil {
		return err
	}

	summaries, err := storage.LoadArchivedTaskSummaries(project)
	if err != nil {
		return archiveSummaryUnavailableError(project, err)
	}

	filtered := filterArchiveSummaries(summaries, filters)

	if out.JSON {
		return displayArchiveSummariesJSON(project, filtered)
	}
	if out.Quiet {
		return nil
	}

	if len(filtered) == 0 {
		if len(summaries) == 0 {
			fmt.Println("No archived tasks found.")
		} else {
			fmt.Println("No archived tasks match the current filters.")
		}
		return nil
	}

	displayArchiveSummaryTable(project, filtered)
	return nil
}

func archiveSummaryUnavailableError(project *models.Project, err error) error {
	rebuildCmd := "tt task archive rebuild-index"
	projectName := "project"
	if project != nil {
		projectName = project.Name
		if project.Slug != "" {
			rebuildCmd += " --project " + project.Slug
		}
	}
	return fmt.Errorf("archive summaries are unavailable for %s: %w; run '%s' or 'tt doctor --fix'", projectName, err, rebuildCmd)
}

func runTaskArchiveShow(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, cfg, err := loadArchiveAccessProject(cmd)
	if err != nil {
		return err
	}

	task, err := storage.FindArchivedTask(project, args[0])
	if err != nil {
		return err
	}

	if out.JSON {
		return displayTaskShowJSON(cfg, task)
	}
	if out.Quiet {
		return nil
	}

	if cfg != nil && cfg.GlowEnabled && Deps.Renderer.Available() && !archiveShowNoGlow {
		return Deps.Renderer.RenderTaskMarkdown(task)
	}

	displayTaskDetail(task)
	return nil
}

func loadArchiveAccessProject(cmd *cobra.Command) (*models.Project, *config.Config, error) {
	var projectRef string
	if cmd != nil {
		projectRef, _ = cmd.Flags().GetString("project")
	}

	var args []string
	if projectRef != "" {
		args = []string{projectRef}
	}

	project, cfg, _, err := LoadConfigAndProjectOrCurrent(args)
	if err != nil {
		return nil, nil, err
	}
	return project, cfg, nil
}

type archiveListFilters struct {
	taskType  *models.TaskType
	status    *models.TaskStatus
	since     *time.Time
	untilNext *time.Time
}

func parseArchiveListFilters() (archiveListFilters, error) {
	var filters archiveListFilters

	if archiveListType != "" {
		taskType, ok := models.ParseTaskType(archiveListType)
		if !ok {
			return filters, tterrors.NewInvalidTaskTypeError(archiveListType)
		}
		filters.taskType = &taskType
	}

	if archiveListStatus != "" {
		status, ok := models.ParseTaskStatus(archiveListStatus)
		if !ok {
			return filters, fmt.Errorf("invalid status %q (valid: %s)", archiveListStatus, strings.Join(statusCompletions, ", "))
		}
		filters.status = &status
	}

	since, err := parseArchiveListDateFlag("--since", archiveListSince)
	if err != nil {
		return filters, err
	}
	filters.since = since

	until, err := parseArchiveListDateFlag("--until", archiveListUntil)
	if err != nil {
		return filters, err
	}
	if until != nil {
		untilNext := until.AddDate(0, 0, 1)
		filters.untilNext = &untilNext
	}

	if filters.since != nil && filters.untilNext != nil && !filters.since.Before(*filters.untilNext) {
		return filters, fmt.Errorf("--since must be on or before --until")
	}

	return filters, nil
}

func parseArchiveListDateFlag(flagName, value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(util.DateFormat, value, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid %s date %q (expected YYYY-MM-DD)", flagName, value)
	}
	return &parsed, nil
}

func filterArchiveSummaries(summaries []storage.ArchiveTaskSummary, filters archiveListFilters) []storage.ArchiveTaskSummary {
	filtered := make([]storage.ArchiveTaskSummary, 0, len(summaries))
	for _, summary := range summaries {
		if archiveSummaryMatches(summary, filters) {
			filtered = append(filtered, summary)
		}
	}
	return filtered
}

func archiveSummaryMatches(summary storage.ArchiveTaskSummary, filters archiveListFilters) bool {
	if filters.taskType != nil && summary.Type != *filters.taskType {
		return false
	}
	if filters.status != nil && !stateMatchesLegacyStatus(summary.WorkflowState, *filters.status) {
		return false
	}
	if filters.since != nil || filters.untilNext != nil {
		if summary.CompletedAt == nil {
			return false
		}
		if filters.since != nil && summary.CompletedAt.Before(*filters.since) {
			return false
		}
		if filters.untilNext != nil && !summary.CompletedAt.Before(*filters.untilNext) {
			return false
		}
	}
	return true
}

type archiveSummaryJSONEntry struct {
	Project       string     `json:"project"`
	ID            string     `json:"id"`
	IDPrefix      string     `json:"id_prefix"`
	ShortID       int        `json:"short_id"`
	Type          string     `json:"type"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	Priority      int        `json:"priority"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	EstimatedDays float64    `json:"estimated_days,omitempty"`
	Effort        string     `json:"effort,omitempty"`
	StoryPoints   int        `json:"story_points,omitempty"`
	SourceFile    string     `json:"source_file"`
	SourceLine    int        `json:"source_line,omitempty"`
}

func displayArchiveSummariesJSON(project *models.Project, summaries []storage.ArchiveTaskSummary) error {
	entries := make([]archiveSummaryJSONEntry, 0, len(summaries))
	for _, summary := range summaries {
		entries = append(entries, archiveSummaryJSONEntry{
			Project:       project.Name,
			ID:            summary.ID,
			IDPrefix:      shortDisplayID(summary.ID),
			ShortID:       summary.ShortID,
			Type:          summary.Type.String(),
			Title:         summary.Title,
			Status:        models.LegacyStatusForState(summary.WorkflowState).String(),
			Priority:      summary.Priority,
			CreatedAt:     summary.CreatedAt,
			CompletedAt:   summary.CompletedAt,
			EstimatedDays: summary.EstimatedDays,
			Effort:        string(summary.Effort),
			StoryPoints:   summary.StoryPoints,
			SourceFile:    summary.Source.FileName,
			SourceLine:    summary.Source.HeadingLine,
		})
	}
	return printJSON(entries)
}

func displayArchiveSummaryTable(project *models.Project, summaries []storage.ArchiveTaskSummary) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "#", "Type", "Status", "Completed", "Title", "Source"})

	for _, summary := range summaries {
		t.AppendRow(table.Row{
			shortDisplayID(summary.ID),
			summary.ShortID,
			summary.Type.Icon(),
			summary.WorkflowState.Icon(),
			formatArchiveCompletedAt(summary.CompletedAt),
			util.Truncate(summary.Title, 40),
			formatArchiveSource(summary.Source),
		})
	}

	fmt.Printf("Archived tasks in %s:\n\n", project.Name)
	fmt.Println(t.Render())
}

func formatArchiveCompletedAt(completedAt *time.Time) string {
	if completedAt == nil {
		return ""
	}
	return completedAt.Format(util.DateFormat)
}

func formatArchiveSource(source storage.ArchiveTaskSource) string {
	if source.FileName == "" {
		return ""
	}
	if source.HeadingLine > 0 {
		return fmt.Sprintf("%s:%d", source.FileName, source.HeadingLine)
	}
	return source.FileName
}
