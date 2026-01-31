package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

var taskActivityCmd = &cobra.Command{
	Use:     "activity <task-id>",
	Aliases: []string{"act", "history"},
	Short:   "Show task activity log",
	Long: `Show the activity log for a task, displaying field changes over time.

The activity log records changes to status, priority, section, tags, due date,
title, and other fields. Entries are shown in reverse chronological order.

Examples:
  tt task activity 5           # Show activity log for task 5
  tt task activity 5 --since 7d  # Show activity from the last 7 days`,
	Args: cobra.ExactArgs(1),
	RunE: runTaskActivity,
}

var activitySinceFlag string

func init() {
	taskCmd.AddCommand(taskActivityCmd)
	taskActivityCmd.Flags().StringVar(&activitySinceFlag, "since", "", "Filter entries newer than duration (e.g., 1d, 7d, 2w)")
}

func runTaskActivity(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return tterrors.NewTaskNotFoundError(args[0])
	}

	fmt.Printf("#%d: %s\n", task.ShortID, task.Title)
	fmt.Println(strings.Repeat("-", 40))

	if len(task.Activity) == 0 {
		fmt.Println("No activity recorded.")
		return nil
	}

	// Parse since filter
	var sinceTime time.Time
	if activitySinceFlag != "" {
		dur, err := parseDuration(activitySinceFlag)
		if err != nil {
			return fmt.Errorf("invalid --since value: %w", err)
		}
		sinceTime = time.Now().Add(-dur)
	}

	count := 0
	// Print in reverse chronological order (newest first)
	for i := len(task.Activity) - 1; i >= 0; i-- {
		entry := task.Activity[i]

		if !sinceTime.IsZero() && entry.Timestamp.Before(sinceTime) {
			continue
		}

		relTime := storage.FormatRelativeTime(entry.Timestamp)
		ts := fmt.Sprintf("[%s] %s", entry.Timestamp.Format("2006-01-02 15:04"), relTime)

		var detail string
		if entry.Field == "created" {
			detail = "created"
		} else if entry.OldValue == "" {
			detail = fmt.Sprintf("%s: %s", entry.Field, entry.NewValue)
		} else {
			detail = fmt.Sprintf("%s: %s > %s", entry.Field, entry.OldValue, entry.NewValue)
		}

		fmt.Printf("  %s  %s\n", ts, detail)
		count++
	}

	if count == 0 {
		fmt.Println("No activity entries match the filter.")
	} else {
		fmt.Printf("\n%d entries\n", count)
	}

	return nil
}

// parseDuration parses human-friendly duration strings like "1d", "7d", "2w".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("too short: %q", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid number: %q", numStr)
	}

	switch unit {
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * util.Day, nil
	case 'w':
		return time.Duration(num) * util.Week, nil
	default:
		return 0, fmt.Errorf("unknown unit: %c (use h, d, or w)", unit)
	}
}
