package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	taskRecurClear bool
	taskRecurSkip  bool
	taskRecurDefer int
)

var taskRecurCmd = &cobra.Command{
	Use:   "recur <task-id> [pattern]",
	Short: "Set or clear task recurrence",
	Long: `Set or clear the recurrence pattern for a task.

When a recurring task is completed, the system automatically generates
the next instance with an updated due date.

Recurrence patterns:
  daily              - Every day
  weekly             - Every week
  biweekly           - Every 2 weeks
  monthly            - Every month
  yearly             - Every year
  Nd                 - Every N days (e.g., 3d = every 3 days)
  Nw                 - Every N weeks (e.g., 2w = every 2 weeks)
  Nm                 - Every N months (e.g., 6m = every 6 months)
  Ny                 - Every N years (e.g., 2y = every 2 years)
  weekly:mon,wed,fri - Weekly on specific days

Examples:
  tt task recur 5 weekly        # Task #5 recurs weekly
  tt task recur 12 2w           # Task #12 recurs every 2 weeks
  tt task recur 8 monthly       # Task #8 recurs monthly
  tt task recur 3 weekly:mon,fri # Task #3 recurs on Mondays and Fridays
  tt task recur 5 --clear       # Remove recurrence from task #5
  tt task recur 5 --skip        # Skip next occurrence, advance due date
  tt task recur 5 --defer 3     # Defer recurrence by 3 days`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTaskRecur,
}

func init() {
	taskCmd.AddCommand(taskRecurCmd)
	taskRecurCmd.Flags().BoolVar(&taskRecurClear, "clear", false, "Remove recurrence pattern")
	taskRecurCmd.Flags().BoolVar(&taskRecurSkip, "skip", false, "Skip next occurrence and advance due date")
	taskRecurCmd.Flags().IntVar(&taskRecurDefer, "defer", 0, "Defer recurrence by N days")
	addOutputFlags(taskRecurCmd)
}

func runTaskRecur(cmd *cobra.Command, args []string) error { //nolint:gocyclo
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Reject conflicting action flags
	actionCount := 0
	var actionFlags []string
	if taskRecurClear {
		actionCount++
		actionFlags = append(actionFlags, "--clear")
	}
	if taskRecurSkip {
		actionCount++
		actionFlags = append(actionFlags, "--skip")
	}
	if cmd.Flags().Changed("defer") {
		actionCount++
		actionFlags = append(actionFlags, "--defer")
	}
	if actionCount > 1 {
		return fmt.Errorf("conflicting flags: %v (specify only one)", actionFlags)
	}
	// Setting a pattern and an action flag is also conflicting
	if actionCount > 0 && len(args) >= 2 {
		return fmt.Errorf("cannot combine %s with a recurrence pattern argument", actionFlags[0])
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, err := FindTaskByIdentifier(project, args[0])
	if err != nil {
		return err
	}

	// Handle --skip flag
	if taskRecurSkip {
		if task.Recurrence == nil {
			return fmt.Errorf("task #%d is not recurring; set recurrence first with: tt task recur %d <pattern>", task.ShortID, task.ShortID)
		}
		task.SkipRecurrence()
		if err := SaveProject(project, &SaveOptions{
			Config: cfg, ConfigManager: configManager,
			UndoAction: "Skip Recurrence", UndoContext: fmt.Sprintf("#%d", task.ShortID),
		}); err != nil {
			return tterrors.NewSaveProjectError(project.Name, err)
		}
		if out.JSON {
			return outputSingleTaskJSON(task)
		}
		if !out.Quiet {
			dueStr := "none"
			if task.DueDate != nil {
				dueStr = task.DueDate.Format("Mon Jan 2, 2006")
			}
			fmt.Printf("Skipped: next due %s\n", dueStr)
		}
		return nil
	}

	// Handle --defer flag
	if cmd.Flags().Changed("defer") && taskRecurDefer <= 0 {
		return fmt.Errorf("defer days must be a positive number, got %d", taskRecurDefer)
	}
	if taskRecurDefer > 0 {
		if task.Recurrence == nil {
			return fmt.Errorf("task #%d is not recurring; set recurrence first with: tt task recur %d <pattern>", task.ShortID, task.ShortID)
		}
		task.DeferRecurrence(taskRecurDefer)
		if err := SaveProject(project, &SaveOptions{
			Config: cfg, ConfigManager: configManager,
			UndoAction: "Defer Recurrence", UndoContext: fmt.Sprintf("%d days #%d", taskRecurDefer, task.ShortID),
		}); err != nil {
			return tterrors.NewSaveProjectError(project.Name, err)
		}
		if out.JSON {
			return outputSingleTaskJSON(task)
		}
		if !out.Quiet {
			dueStr := "none"
			if task.DueDate != nil {
				dueStr = task.DueDate.Format("Mon Jan 2, 2006")
			}
			fmt.Printf("Deferred: now due %s\n", dueStr)
		}
		return nil
	}

	// Handle --clear flag
	if taskRecurClear {
		if task.Recurrence == nil {
			fmt.Printf("Task #%d has no recurrence pattern\n", task.ShortID)
			return nil
		}
		task.ClearRecurrence()
		if err := SaveProject(project, &SaveOptions{
			Config: cfg, ConfigManager: configManager,
			UndoAction: "Clear Recurrence", UndoContext: fmt.Sprintf("#%d", task.ShortID),
		}); err != nil {
			return tterrors.NewSaveProjectError(project.Name, err)
		}
		if out.JSON {
			return outputSingleTaskJSON(task)
		}
		if !out.Quiet {
			fmt.Printf("Removed recurrence from task #%d (%s)\n", task.ShortID, task.Title)
		}
		return nil
	}

	// Require pattern argument when not clearing/skipping/deferring
	if len(args) < 2 {
		if task.Recurrence != nil {
			fmt.Printf("Task #%d recurs %s\n", task.ShortID, task.Recurrence.String())
			if task.Recurrence.Streak > 0 || task.Recurrence.MaxStreak > 0 {
				fmt.Printf("Streak: %d (best: %d)\n", task.Recurrence.Streak, task.Recurrence.MaxStreak)
			}
			if task.Recurrence.Skipped > 0 {
				fmt.Printf("Skipped: %d times\n", task.Recurrence.Skipped)
			}
			upcoming := task.NextNDueDates(3)
			if len(upcoming) > 0 {
				fmt.Println("Upcoming:")
				for i, d := range upcoming {
					fmt.Printf("  %d. %s\n", i+1, d.Format("Mon Jan 2, 2006"))
				}
			}
		} else {
			fmt.Printf("Task #%d has no recurrence pattern\n", task.ShortID)
		}
		return nil
	}

	pattern := models.ParseRecurrence(args[1])
	if pattern == nil {
		return fmt.Errorf("invalid recurrence pattern: %s", args[1])
	}

	oldPattern := ""
	if task.Recurrence != nil {
		oldPattern = task.Recurrence.String()
	}

	if err := task.SetRecurrence(pattern); err != nil {
		return fmt.Errorf("invalid recurrence pattern: %w", err)
	}

	// Warn when setting monthly recurrence on a task whose due date falls on
	// day 29–31: those days don't exist in all months, so the recurrence will
	// silently clamp to the last day of shorter months.
	if pattern.Unit == models.RecurrenceMonthly && task.DueDate != nil && task.DueDate.Day() > 28 {
		fmt.Fprintf(os.Stderr, "warning: due date is on day %d; months with fewer days will use the last day instead\n", task.DueDate.Day())
	}

	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Set Recurrence", UndoContext: fmt.Sprintf("%s #%d", pattern.String(), task.ShortID),
	}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		if oldPattern != "" {
			fmt.Printf("Updated recurrence for task #%d: %s -> %s (%s)\n", task.ShortID, oldPattern, pattern.String(), task.Title)
		} else {
			fmt.Printf("Set recurrence for task #%d: %s (%s)\n", task.ShortID, pattern.String(), task.Title)
		}
	}
	return nil
}
