package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Quick capture a task",
	Long: `Quick capture a new task with minimal friction.

This is a shorthand for "tt task add" with smart inline parsing:
  - Inline priority: !1 through !5 (e.g., "Fix bug !1")
  - Inline tags: #tag (e.g., "Add feature #backend #urgent")

The default type is "task". Use flags for other types.

Examples:
  tt add "Fix login bug"
  tt add "Fix login bug !1"
  tt add "Add feature #backend #urgent"
  tt add "Critical fix !1 #hotfix"
  tt add --story "User authentication flow"
  tt add "Implement cache" --parent 1
  tt add "Deploy service" --project myproject`,
	Aliases: []string{"a"},
	RunE:    runQuickAdd,
}

var (
	addTypeFlag   string
	addParentFlag string
	addPriority   int
	addTags       []string
	addIsEpic     bool
	addIsStory    bool
	addIsBug      bool
	addIsIdea     bool
)

// Pre-compiled regexps for parseInlineMetadata to avoid recompilation on each call.
//
// anyPriorityRe captures the full digit run after "!" so multi-digit tokens
// like "!19" are evaluated as a whole. Using a "!([1-5])" regex would have
// matched the leading "1" and left "9" stranded in the title.
var (
	anyPriorityRe = regexp.MustCompile(`\s*!([0-9]+)\s*`)
	inlineTagRe   = regexp.MustCompile(`\s*#([\w][\w.-]*)\s*`)
	multiSpaceRe  = regexp.MustCompile(`\s+`)
)

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&addTypeFlag, "type", "t", "", "task type (epic, story, task, bug, idea)")
	addCmd.Flags().StringVar(&addParentFlag, "parent", "", "parent task ID or short ID")
	addCmd.Flags().IntVarP(&addPriority, "priority", "P", 0, "priority (1=highest, 5=lowest)")
	addCmd.Flags().StringSliceVarP(&addTags, "tag", "T", nil, "tags for the task")

	addCmd.Flags().BoolVar(&addIsEpic, "epic", false, "create an epic")
	addCmd.Flags().BoolVar(&addIsStory, "story", false, "create a story")
	addCmd.Flags().BoolVar(&addIsBug, "bug", false, "create a bug")
	addCmd.Flags().BoolVar(&addIsIdea, "idea", false, "create an idea")

	addCmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")

	_ = addCmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
}

// parseInlineMetadata extracts priority (!N) and tags (#tag) from the title.
// Returns the cleaned title, extracted priority (0 if not found), and tags.
func parseInlineMetadata(input string) (title string, priority int, tags []string) {
	// Match every "!<digits>" token in one pass. The first valid 1–5 single-digit
	// token wins (matching prior behavior). Multi-digit or out-of-range tokens
	// are warned about and stripped — never partially consumed.
	for _, match := range anyPriorityRe.FindAllStringSubmatch(input, -1) {
		if len(match) < 2 {
			continue
		}
		digits := match[1]
		if len(digits) == 1 && digits[0] >= '1' && digits[0] <= '5' {
			if priority == 0 {
				priority, _ = strconv.Atoi(digits)
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "warning: !%s ignored (priority must be 1-5)\n", digits)
	}
	input = anyPriorityRe.ReplaceAllString(input, " ")

	// Match tag pattern: #word
	tagMatches := inlineTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			tags = append(tags, match[1])
		}
	}
	input = inlineTagRe.ReplaceAllString(input, " ")

	// Clean up extra whitespace
	title = strings.TrimSpace(input)
	title = multiSpaceRe.ReplaceAllString(title, " ")

	return title, priority, tags
}

func runQuickAdd(cmd *cobra.Command, args []string) error {
	var (
		project   *models.Project
		workspace *CLIWorkspace
		err       error
	)

	if slug, _ := cmd.Flags().GetString("project"); slug != "" {
		workspace, err = LoadCLIWorkspace()
		if err != nil {
			return err
		}
		project = workspace.Runtime.GetProject(slug)
		if project == nil {
			return tterrors.NewProjectNotFoundError(slug)
		}
		if err = storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to load project %s: %w", slug, err)
		}
	} else {
		project, workspace, err = LoadRuntimeCurrentProject()
		if err != nil {
			return err
		}
	}

	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("task title")
	}

	// Parse input with inline metadata
	rawTitle := strings.Join(args, " ")
	title, inlinePriority, inlineTags := parseInlineMetadata(rawTitle)

	if title == "" {
		return fmt.Errorf("title is empty after extracting inline metadata (priority, tags, etc.) — provide a title")
	}
	if err = models.ValidateTaskTitle(title); err != nil {
		return err
	}

	// Determine task type
	taskType, err := ParseTaskTypeFlags(addIsEpic, addIsStory, addIsBug, addIsIdea, addTypeFlag)
	if err != nil {
		return err
	}

	// Determine priority: explicit flag > inline > default
	priority := models.PriorityDefault
	if addPriority > 0 {
		priority = addPriority
	} else if inlinePriority > 0 {
		priority = inlinePriority
	}

	// Validate priority
	if err = ValidatePriority(priority, false); err != nil {
		return err
	}

	// Tags: explicit flags first, then inline (preserving legacy ordering); the
	// application-layer AddTask use case normalizes, validates, and dedups them.
	tags := make([]string, 0, len(addTags)+len(inlineTags))
	tags = append(tags, addTags...)
	tags = append(tags, inlineTags...)

	// Resolve the parent to its stable UUID (with the legacy hierarchy/depth
	// checks); the use case re-validates against the fresh snapshot.
	var parentID *string
	if addParentFlag != "" {
		parent, parentErr := ValidateParentTask(project, addParentFlag, taskType)
		if parentErr != nil {
			return parentErr
		}
		pid := parent.ID
		parentID = &pid
	}

	// Route through the same application-layer use case as `tt task add`: the
	// service owns ID/short-ID minting, validation, the workspace-ledger undo
	// record, and the single atomic ChangeSet commit. Quick-add and `tt task add`
	// are the same task-capture family, so both share this path.
	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		return workflows.service.AddTaskWithWorkflow(app.AddTaskRequest{
			ProjectID: project.ID,
			Title:     title,
			Type:      taskType,
			Priority:  priority,
			Tags:      tags,
			State:     models.StateReady,
			ParentID:  parentID,
		}, workflows.options(false, "Add Task", nil))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	res := workflowResultTask(cascade)

	// Reconstruct the display tag list with the same normalize/dedup semantics the
	// use case applied, keeping the explicit-then-inline order for the message.
	var displayTags models.Task
	for _, tag := range tags {
		displayTags.AddTag(tag)
	}

	// Build output message
	var extras []string
	if priority != 3 {
		extras = append(extras, fmt.Sprintf("priority %d", priority))
	}
	if len(displayTags.Tags) > 0 {
		extras = append(extras, fmt.Sprintf("tags: %s", strings.Join(displayTags.Tags, ", ")))
	}

	msg := fmt.Sprintf("[%s] Created %s #%d: %s", project.Name, taskType.HeadingPrefix(), res.ShortID, title)
	if len(extras) > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(extras, ", "))
	}
	fmt.Println(msg)

	return nil
}
