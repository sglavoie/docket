package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var taskPriorityCmd = &cobra.Command{
	Use:   "priority <task-id>... <priority>",
	Short: "Set task priority",
	Long: `Set the priority of one or more tasks. The last argument is the priority value.

The last argument is always the priority value; all preceding arguments are task IDs.

Priority levels:
  1 - Critical/Urgent
  2 - High
  3 - Medium (default)
  4 - Low
  5 - Minimal

Examples:
  tt task priority 5 1       # Set task #5 to highest priority
  tt task priority 1 2 3 2   # Set tasks #1, #2, #3 to high priority`,
	RunE: runTaskPriority,
}

func init() {
	taskCmd.AddCommand(taskPriorityCmd)
	addOutputFlags(taskPriorityCmd)
	taskPriorityCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= 1 {
			return []string{"1", "2", "3", "4", "5"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTaskPriority(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and priority")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Last argument is the priority value, everything before is task IDs
	taskIDs := args[:len(args)-1]
	priorityArg := args[len(args)-1]

	priority, err := strconv.Atoi(priorityArg)
	if err != nil {
		return tterrors.NewInvalidValueError("priority", priorityArg, fmt.Sprintf("must be %d-%d", models.PriorityMin, models.PriorityMax))
	}

	if errValidate := ValidatePriority(priority, false); errValidate != nil {
		return errValidate
	}

	type taskEntry struct {
		ID        string   `json:"id"`
		ShortID   int      `json:"short_id"`
		Type      string   `json:"type"`
		Title     string   `json:"title"`
		Status    string   `json:"status"`
		Priority  int      `json:"priority"`
		Tags      []string `json:"tags"`
		Section   string   `json:"section,omitempty"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
	}

	var errs []string
	succeeded := 0
	var updated []taskEntry
	for _, taskID := range taskIDs {
		var oldPriority int

		alreadyAtPriority := false
		task, err := WithTaskUpdate(taskID, func(task *models.Task) error {
			oldPriority = task.Priority

			if oldPriority == priority {
				alreadyAtPriority = true
				return fmt.Errorf("already at priority %d", priority)
			}

			task.Priority = priority
			return nil
		})

		if err != nil {
			if alreadyAtPriority && len(taskIDs) > 1 {
				// In batch mode, silently skip tasks already at the target priority
				continue
			}
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, err))
			continue
		}

		tags := task.Tags
		if tags == nil {
			tags = []string{}
		}
		updated = append(updated, taskEntry{
			ID:        task.ID,
			ShortID:   task.ShortID,
			Type:      task.Type.String(),
			Title:     task.Title,
			Status:    task.Status.String(),
			Priority:  task.Priority,
			Tags:      tags,
			Section:   task.Section,
			CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		if !out.Quiet && !out.JSON {
			fmt.Printf("Updated priority for task #%d: %d -> %d (%s)\n", task.ShortID, oldPriority, priority, task.Title)
		}
		succeeded++
	}

	if out.JSON {
		if updated == nil {
			updated = []taskEntry{}
		}
		data, err := json.MarshalIndent(updated, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal tasks to JSON: %w", err)
		}
		fmt.Println(string(data))
		return ReportBatchErrors(succeeded, errs)
	}

	return ReportBatchErrors(succeeded, errs)
}
