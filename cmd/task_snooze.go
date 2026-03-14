package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

var taskSnoozeCmd = &cobra.Command{
	Use:   "snooze <task-id> <date|duration|clear>",
	Short: "Snooze a task until a specific date",
	Long: `Snooze a task to hide it from all views until the specified date.
Snoozed tasks reappear automatically when the snooze date arrives.

Accepted formats:
  tomorrow       Tomorrow's date
  +1d, +3d       Relative days from now
  +1w, +2w       Relative weeks from now
  +1m            Relative months from now
  2026-06-01     Exact date (YYYY-MM-DD)
  clear          Remove snooze (make task visible again)

Examples:
  tt task snooze 5 tomorrow
  tt task snooze 12 +1w
  tt task snooze 3 2026-06-01
  tt task snooze 7 clear`,
	Args: cobra.ExactArgs(2),
	RunE: runTaskSnooze,
}

func init() {
	taskCmd.AddCommand(taskSnoozeCmd)
}

func runTaskSnooze(cmd *cobra.Command, args []string) error {
	taskID := args[0]
	dateStr := args[1]

	var snoozeDate *time.Time
	var desc string

	if dateStr == "clear" || dateStr == "none" {
		snoozeDate = nil
		desc = "cleared"
	} else {
		parsed, err := util.ParseRelativeDate(dateStr)
		if err != nil {
			return fmt.Errorf("invalid date %q: %w\nAccepted formats: YYYY-MM-DD, +Nd/+Nw/+Nm, today, tomorrow, mon-sun, next week/month, in N days/weeks/months", dateStr, err)
		}
		// Reject snoozing to today or the past since the task would be immediately visible
		today := util.StartOfDay(time.Now())
		if !parsed.After(today) {
			return fmt.Errorf("snooze date must be in the future (got %s)", parsed.Format(util.DateFormat))
		}
		snoozeDate = &parsed
		desc = parsed.Format(util.DateFormat)
	}

	task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
		// Prevent snoozing completed/canceled tasks (it makes no sense to hide them)
		if snoozeDate != nil && task.Status.IsComplete() {
			return fmt.Errorf("cannot snooze a %s task", task.Status)
		}
		task.SnoozedUntil = snoozeDate
		return nil
	})

	if err != nil {
		return err
	}

	if snoozeDate == nil {
		fmt.Printf("Cleared snooze from task #%d: %s\n", task.ShortID, task.Title)
	} else {
		fmt.Printf("Snoozed task #%d until %s: %s\n", task.ShortID, desc, task.Title)
	}

	return nil
}
