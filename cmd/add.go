package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
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
var (
	inlinePriorityRe  = regexp.MustCompile(`\s*!([1-5])\s*`)
	invalidPriorityRe = regexp.MustCompile(`\s*!([0-9]+)\s*`)
	inlineTagRe       = regexp.MustCompile(`\s*#(\w+)\s*`)
	multiSpaceRe      = regexp.MustCompile(`\s+`)
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
	// Match priority pattern: !1 through !5
	priorityMatch := inlinePriorityRe.FindStringSubmatch(input)
	if len(priorityMatch) > 1 {
		priority, _ = strconv.Atoi(priorityMatch[1])
		input = inlinePriorityRe.ReplaceAllString(input, " ")
	} else {
		// Check for out-of-range priority (e.g., !0, !6, !99)
		invalidMatch := invalidPriorityRe.FindStringSubmatch(input)
		if len(invalidMatch) > 1 {
			fmt.Fprintf(os.Stderr, "warning: !%s ignored (priority must be 1-5)\n", invalidMatch[1])
			input = invalidPriorityRe.ReplaceAllString(input, " ")
		}
	}

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
		project       *models.Project
		cfg           *config.Config
		configManager *config.Manager
		err           error
	)

	if slug, _ := cmd.Flags().GetString("project"); slug != "" {
		cfg, configManager, err = LoadConfig()
		if err != nil {
			return err
		}
		project = cfg.GetProject(slug)
		if project == nil {
			return tterrors.NewProjectNotFoundError(slug)
		}
		if err = storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to load project %s: %w", slug, err)
		}
	} else {
		project, cfg, configManager, err = LoadCurrentProject()
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

	// Create task
	taskID := uuid.New().String()
	shortID := project.GetNextShortID()
	task, err := models.NewTask(taskID, shortID, taskType, title)
	if err != nil {
		return err
	}
	task.Priority = priority

	// Add tags: explicit flags first, then inline
	for _, tag := range addTags {
		task.AddTag(tag)
	}
	for _, tag := range inlineTags {
		task.AddTag(tag)
	}

	// Handle parent
	if addParentFlag != "" {
		parent, err := ValidateParentTask(project, addParentFlag, task.Type)
		if err != nil {
			return err
		}
		task.ParentID = &parent.ID
	}

	// Add to project
	project.Tasks = append(project.Tasks, task)

	// Save with workflow rules evaluation (state-based rules will fire for new tasks)
	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: configManager,
		UndoAction: "Add Task", UndoContext: fmt.Sprintf("#%d %s", shortID, title),
	}); err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}

	// Build output message
	var extras []string
	if priority != 3 {
		extras = append(extras, fmt.Sprintf("priority %d", priority))
	}
	if len(task.Tags) > 0 {
		extras = append(extras, fmt.Sprintf("tags: %s", strings.Join(task.Tags, ", ")))
	}

	msg := fmt.Sprintf("[%s] Created %s #%d: %s", project.Name, taskType.HeadingPrefix(), shortID, title)
	if len(extras) > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(extras, ", "))
	}
	fmt.Println(msg)

	return nil
}
