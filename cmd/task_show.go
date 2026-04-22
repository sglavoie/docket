package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
	"github.com/sglavoie/tt/internal/util"
)

var taskShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show task details",
	Long: `Display detailed information about a task.

If no ID is specified, opens an interactive selector.
Uses glow for rendering if available and enabled in config.

Examples:
  tt task show 5
  tt task show
  tt task show 3 --no-glow
  tt t show`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskShow,
}

var noGlow bool

func init() {
	taskCmd.AddCommand(taskShowCmd)
	taskShowCmd.Flags().BoolVar(&noGlow, "no-glow", false, "disable glow rendering")
	addOutputFlags(taskShowCmd)
}

func runTaskShow(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task

	if len(args) > 0 {
		var findErr error
		task, findErr = FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
	} else {
		// Interactive selection
		activeTasks := filterTasks(project.Tasks)
		if len(activeTasks) == 0 {
			fmt.Println(NoItemsMsg("tasks", "tt task add <title>"))
			return nil
		}

		items := TasksToSelectorItems(activeTasks)
		selected, err := tui.RunSelector("Select task:", items)
		if err != nil {
			return err
		}

		task, err = GetTaskFromSelector(selected)
		if err != nil {
			return err
		}
	}

	if out.JSON {
		return displayTaskShowJSON(task)
	}

	if out.Quiet {
		return nil
	}

	// Use glow if available and enabled
	useGlow := cfg.GlowEnabled && Deps.Renderer.Available() && !noGlow
	if useGlow {
		return Deps.Renderer.RenderTaskMarkdown(task)
	}

	displayTaskDetail(task)
	return nil
}

func displayTaskDetail(task *models.Task) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle := lipgloss.NewStyle()

	// Header
	fmt.Println(titleStyle.Render(fmt.Sprintf("%s #%d: %s",
		task.Type.HeadingPrefix(), task.ShortID, task.Title)))
	fmt.Println()

	// Metadata
	fmt.Printf("%s %s %s\n",
		labelStyle.Render("Status:"),
		task.Status.Icon(),
		valueStyle.Render(string(task.Status)))

	fmt.Printf("%s %d\n",
		labelStyle.Render("Priority:"),
		task.Priority)

	fmt.Printf("%s %s\n",
		labelStyle.Render("Type:"),
		valueStyle.Render(task.Type.HeadingPrefix()))

	if task.Dimmed {
		fmt.Printf("%s yes\n", labelStyle.Render("Dimmed:"))
	}

	if len(task.Tags) > 0 {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Tags:"),
			valueStyle.Render(strings.Join(task.Tags, ", ")))
	}

	fmt.Printf("%s %s\n",
		labelStyle.Render("Created:"),
		valueStyle.Render(task.CreatedAt.Format(util.DateTimeFormat)))

	fmt.Printf("%s %s\n",
		labelStyle.Render("Updated:"),
		valueStyle.Render(storage.FormatRelativeTime(task.UpdatedAt)))

	if task.CompletedAt != nil {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Completed:"),
			valueStyle.Render(task.CompletedAt.Format(util.DateTimeFormat)))
	}

	if task.DueDate != nil {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Due:"),
			valueStyle.Render(formatTaskDueDate(task)))
	}

	// Description
	if task.Description != "" {
		fmt.Println()
		fmt.Println(labelStyle.Render("Description:"))
		fmt.Println(task.Description)
	}

	// Children summary
	if task.HasProgressChildren() {
		fmt.Println()
		total := 0
		completed := 0
		for _, child := range task.Children {
			total++
			if child.IsComplete() {
				completed++
			}
		}
		fmt.Printf("%s %d/%d completed (%.0f%%)\n",
			labelStyle.Render("Subtasks:"),
			completed, total,
			task.GetProgress())
	}
}

// taskShowJSONEntry is the full representation of a task for JSON output.
type taskShowJSONEntry struct {
	ID              string                    `json:"id"`
	ShortID         int                       `json:"short_id"`
	Type            string                    `json:"type"`
	Title           string                    `json:"title"`
	Description     string                    `json:"description"`
	Status          string                    `json:"status"`
	Priority        int                       `json:"priority"`
	ParentID        *string                   `json:"parent_id"`
	Tags            []string                  `json:"tags"`
	Section         string                    `json:"section"`
	CustomSection   string                    `json:"custom_section,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	CompletedAt     *time.Time                `json:"completed_at"`
	DueDate         *time.Time                `json:"due_date"`
	HasTime         bool                      `json:"has_time"`
	DependsOn       []string                  `json:"depends_on"`
	RelatedTo       []string                  `json:"related_to"`
	Pinned          bool                      `json:"pinned"`
	PinnedAt        *time.Time                `json:"pinned_at,omitempty"`
	Active          bool                      `json:"active"`
	Dimmed          bool                      `json:"dimmed"`
	SnoozedUntil    *time.Time                `json:"snoozed_until"`
	Stage           string                    `json:"stage,omitempty"`
	Source          string                    `json:"source,omitempty"`
	URL             string                    `json:"url,omitempty"`
	Recurrence      *models.RecurrencePattern `json:"recurrence"`
	Checklist       []models.ChecklistItem    `json:"checklist"`
	Attachments     []models.Attachment       `json:"attachments"`
	Comments        []*models.Comment         `json:"comments"`
	Activity        []*models.ActivityEntry   `json:"activity"`
	ScoreDimensions map[string]int            `json:"score_dimensions"`
	ChildIDs        []string                  `json:"child_ids"`
	BugAttrs        *models.BugAttrs          `json:"bug_attrs,omitempty"`
	IdeaAttrs       *models.IdeaAttrs         `json:"idea_attrs,omitempty"`
	TaskAttrs       *models.TaskAttrs         `json:"task_attrs,omitempty"`
	StoryAttrs      *models.StoryAttrs        `json:"story_attrs,omitempty"`
	EpicAttrs       *models.EpicAttrs         `json:"epic_attrs,omitempty"`
}

func displayTaskShowJSON(task *models.Task) error {
	tags := task.Tags
	if tags == nil {
		tags = []string{}
	}
	dependsOn := task.DependsOn
	if dependsOn == nil {
		dependsOn = []string{}
	}
	relatedTo := task.RelatedTo
	if relatedTo == nil {
		relatedTo = []string{}
	}
	checklist := task.Checklist
	if checklist == nil {
		checklist = []models.ChecklistItem{}
	}
	attachments := task.Attachments
	if attachments == nil {
		attachments = []models.Attachment{}
	}
	comments := task.Comments
	if comments == nil {
		comments = []*models.Comment{}
	}
	activity := task.Activity
	if activity == nil {
		activity = []*models.ActivityEntry{}
	}
	scoreDims := task.ScoreDimensions
	if scoreDims == nil {
		scoreDims = map[string]int{}
	}

	var childIDs []string
	for _, child := range task.Children {
		childIDs = append(childIDs, child.ID)
	}
	if childIDs == nil {
		childIDs = []string{}
	}

	entry := taskShowJSONEntry{
		ID:              task.ID,
		ShortID:         task.ShortID,
		Type:            task.Type.String(),
		Title:           task.Title,
		Description:     task.Description,
		Status:          task.Status.String(),
		Priority:        task.Priority,
		ParentID:        task.ParentID,
		Tags:            tags,
		Section:         task.Section,
		CustomSection:   task.CustomSection,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
		CompletedAt:     task.CompletedAt,
		DueDate:         task.DueDate,
		DependsOn:       dependsOn,
		RelatedTo:       relatedTo,
		Pinned:          task.Pinned,
		PinnedAt:        task.PinnedAt,
		Active:          task.Active,
		Dimmed:          task.Dimmed,
		SnoozedUntil:    task.SnoozedUntil,
		Stage:           task.Stage,
		Source:          task.Source,
		URL:             task.URL,
		Recurrence:      task.Recurrence,
		Checklist:       checklist,
		Attachments:     attachments,
		Comments:        comments,
		Activity:        activity,
		ScoreDimensions: scoreDims,
		ChildIDs:        childIDs,
		BugAttrs:        task.BugAttrs,
		IdeaAttrs:       task.IdeaAttrs,
		TaskAttrs:       task.TaskAttrs,
		StoryAttrs:      task.StoryAttrs,
		EpicAttrs:       task.EpicAttrs,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
