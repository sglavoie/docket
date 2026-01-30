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
  2025-03-01     Exact date (YYYY-MM-DD)
  clear          Remove snooze (make task visible again)

Examples:
  tt task snooze 5 tomorrow
  tt task snooze 12 +1w
  tt task snooze 3 2025-03-01
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
		// Use the TUI's ParseDate function via a local reimplementation
		// to avoid importing the TUI package from cmd
		parsed, err := parseSnoozeDate(dateStr)
		if err != nil {
			return fmt.Errorf("invalid date: %w", err)
		}
		snoozeDate = &parsed
		desc = parsed.Format(util.DateFormat)
	}

	task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
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

// parseSnoozeDate parses date strings for the snooze CLI command.
func parseSnoozeDate(input string) (time.Time, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Exact date: YYYY-MM-DD
	if date, err := time.ParseInLocation(util.DateFormat, input, time.Local); err == nil {
		return date, nil
	}

	switch input {
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	}

	// Relative: +Nd, +Nw, +Nm
	if len(input) >= 3 && input[0] == '+' {
		unit := input[len(input)-1]
		numStr := input[1 : len(input)-1]
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err == nil {
			switch unit {
			case 'd':
				return today.AddDate(0, 0, n), nil
			case 'w':
				return today.AddDate(0, 0, n*7), nil
			case 'm':
				return today.AddDate(0, n, 0), nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s (use YYYY-MM-DD, tomorrow, +Nd, +Nw, +Nm, or clear)", input)
}
