package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

var taskSnoozeCmd = &cobra.Command{
	Use:   "snooze [task-id] <date|duration|clear>",
	Short: "Snooze a task until a specific date",
	Long: `Snooze a task to hide it from all views until the specified date.
Snoozed tasks reappear automatically when the snooze date arrives.

If only a date is given (no task ID), an interactive selector is shown.

Accepted formats:
  tomorrow       Tomorrow's date
  eod            End of today
  eow            End of this week (configurable: fri/sat/sun)
  eom            End of this month
  eoy            End of this year
  this <weekday> This week's occurrence (e.g. this fri)
  next <weekday> Next occurrence of that weekday (e.g. next mon)
  +1d, +3d       Relative days from now
  +1w, +2w       Relative weeks from now
  +1m            Relative months from now
  2026-06-01     Exact date (YYYY-MM-DD)
  next week      7 days from today
  next month     Same day next month
  in N days/weeks/months  Relative offset
  clear/none     Remove snooze (make task visible again)

Examples:
  tt task snooze 5 tomorrow
  tt task snooze 12 +1w
  tt task snooze 3 2026-06-01
  tt task snooze 7 clear
  tt task snooze tomorrow`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTaskSnooze,
}

func init() {
	taskCmd.AddCommand(taskSnoozeCmd)
	addOutputFlags(taskSnoozeCmd)
	taskSnoozeCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		dateCompletions := []string{
			"clear", "none", "tomorrow",
			"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
		}
		if len(args) == 0 {
			// First arg could be task ID; complete task IDs
			return completeCurrentProjectTaskIDs()(nil, nil, "")
		}
		if len(args) == 1 {
			return dateCompletions, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTaskSnooze(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	// 1 arg: interactive task selection; date is the only arg
	// 2 args: args[0]=task-id, args[1]=date
	var task *models.Task
	var dateStr string
	if len(args) == 1 {
		dateStr = args[0]
		selectedTask, selErr := SelectTask(project, nil,
			func(t *models.Task) bool { return !t.IsTerminalState() },
			"Select task to snooze:", "No active tasks found.")
		if selErr != nil {
			return selErr
		}
		task = selectedTask
	} else {
		dateStr = args[1]
		found, findErr := FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
		task = found
	}

	var snoozeDate *time.Time
	var desc string

	if dateStr == "clear" || dateStr == "none" {
		snoozeDate = nil
		desc = "cleared"
	} else {
		parsed, parseErr := util.ParseRelativeDate(dateStr)
		if parseErr != nil {
			return fmt.Errorf("invalid date %q: %w\nAccepted formats: YYYY-MM-DD, +Nd/+Nw/+Nm, today, tomorrow, eod/eow/eom/eoy, this <weekday>, next <weekday>, next week/month, in N days/weeks/months", dateStr, parseErr)
		}
		// Reject snoozing to today or the past since the task would be immediately visible
		today := util.StartOfDay(time.Now())
		if !parsed.After(today) {
			return fmt.Errorf("snooze date must be in the future (got %s)", parsed.Format(util.DateFormat))
		}
		snoozeDate = &parsed
		desc = parsed.Format(util.DateFormat)
	}

	// Pre-check: skip the mutation if clearing snooze on a non-snoozed task. This is
	// a read-only adapter guard that keeps the friendly no-op message; the completed
	// and in-progress guards for the snooze-set path now live in the application
	// service (Service.Snooze).
	if snoozeDate == nil && task.SnoozedUntil == nil {
		fmt.Printf("Task #%d is not snoozed.\n", task.ShortID)
		return nil
	}

	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
		return workflows.service.GuardedSnoozeWithWorkflowByLocator(locator, snoozeDate, workflows.options(true, "Snooze Task", task))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	if !workflowResultCommitted(cascade) {
		fmt.Printf("Task #%d is not snoozed.\n", task.ShortID)
		return nil
	}
	res := workflowResultTask(cascade)

	if out.JSON {
		return emitMutation(out, "", res)
	}
	if !out.Quiet {
		if snoozeDate == nil {
			fmt.Printf("Cleared snooze from task #%d: %s\n", task.ShortID, res.Title)
		} else {
			fmt.Printf("Snoozed task #%d until %s: %s\n", task.ShortID, desc, res.Title)
		}
	}
	return nil
}
