package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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
  tt add "Implement cache" --parent 1`,
	Aliases: []string{"a"},
	Args:    cobra.MinimumNArgs(1),
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
	addIsNote     bool
)

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&addTypeFlag, "type", "t", "", "task type (epic, story, task, bug, idea)")
	addCmd.Flags().StringVarP(&addParentFlag, "parent", "p", "", "parent task ID or short ID")
	addCmd.Flags().IntVar(&addPriority, "priority", 0, "priority (1=highest, 5=lowest)")
	addCmd.Flags().StringSliceVar(&addTags, "tags", nil, "tags for the task")

	addCmd.Flags().BoolVar(&addIsEpic, "epic", false, "create an epic")
	addCmd.Flags().BoolVar(&addIsStory, "story", false, "create a story")
	addCmd.Flags().BoolVar(&addIsBug, "bug", false, "create a bug")
	addCmd.Flags().BoolVar(&addIsIdea, "idea", false, "create an idea")
	addCmd.Flags().BoolVar(&addIsNote, "note", false, "create a note")
}

// parseInlineMetadata extracts priority (!N) and tags (#tag) from the title.
// Returns the cleaned title, extracted priority (0 if not found), and tags.
func parseInlineMetadata(input string) (title string, priority int, tags []string) {
	// Match priority pattern: !1 through !5
	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	priorityMatch := priorityRe.FindStringSubmatch(input)
	if len(priorityMatch) > 1 {
		priority, _ = strconv.Atoi(priorityMatch[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Match tag pattern: #word
	tagRe := regexp.MustCompile(`\s*#(\w+)\s*`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			tags = append(tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Clean up extra whitespace
	title = strings.TrimSpace(input)
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	return title, priority, tags
}

func runQuickAdd(cmd *cobra.Command, args []string) error {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Parse input with inline metadata
	rawTitle := strings.Join(args, " ")
	title, inlinePriority, inlineTags := parseInlineMetadata(rawTitle)

	if title == "" {
		return fmt.Errorf("task title is required")
	}

	// Determine task type
	taskType, err := ParseTaskTypeFlags(addIsEpic, addIsStory, addIsBug, addIsIdea, addIsNote, addTypeFlag)
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
	if priority < models.PriorityMin || priority > models.PriorityMax {
		return fmt.Errorf("priority must be between %d and %d", models.PriorityMin, models.PriorityMax)
	}

	// Create task
	taskID := uuid.New().String()
	shortID := project.GetNextShortID()
	task := models.NewTask(taskID, shortID, taskType, title)
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
		parent := FindTaskByIdentifier(project, addParentFlag)
		if parent == nil {
			return tterrors.NewParentTaskNotFoundError(addParentFlag)
		}

		if !task.Type.CanBeChildOf(parent.Type) {
			return fmt.Errorf("%s cannot be a child of %s", task.Type.HeadingPrefix(), parent.Type.HeadingPrefix())
		}

		task.ParentID = &parent.ID
	}

	// Add to project
	project.Tasks = append(project.Tasks, task)

	// Save with workflow rules evaluation (state-based rules will fire for new tasks)
	if err := SaveProjectWithRules(project, cfg, configManager); err != nil {
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
