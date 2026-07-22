package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

var (
	taskRecurClear bool
	taskRecurSkip  bool
	taskRecurDefer int
	taskRecurFrom  string
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

Recurrence mode (--from):
  due        - Next due date is computed from the previous due date (default)
  completion - Next due date is computed from the actual completion time;
               late completions push the next occurrence forward

Examples:
  tt task recur 5 weekly              # Task #5 recurs weekly
  tt task recur 12 2w                 # Task #12 recurs every 2 weeks
  tt task recur 8 monthly             # Task #8 recurs monthly
  tt task recur 3 weekly:mon,fri      # Task #3 recurs on Mondays and Fridays
  tt task recur 5 daily --from completion  # Recur daily from completion time
  tt task recur 7 --from due          # Switch task #7 to from-due mode
  tt task recur 5 --clear             # Remove recurrence from task #5
  tt task recur 5 --skip              # Skip next occurrence, advance due date
  tt task recur 5 --defer 3           # Defer recurrence by 3 days`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTaskRecur,
}

func init() {
	taskCmd.AddCommand(taskRecurCmd)
	taskRecurCmd.Flags().BoolVar(&taskRecurClear, "clear", false, "Remove recurrence pattern")
	taskRecurCmd.Flags().BoolVar(&taskRecurSkip, "skip", false, "Skip next occurrence and advance due date")
	taskRecurCmd.Flags().IntVar(&taskRecurDefer, "defer", 0, "Defer recurrence by N days")
	taskRecurCmd.Flags().StringVar(&taskRecurFrom, "from", "", "Recurrence mode: 'due' (default) or 'completion'")
	addOutputFlags(taskRecurCmd)
}

func runTaskRecur(cmd *cobra.Command, args []string) error { //nolint:gocyclo
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Validate --from value
	var recurMode models.RecurMode
	if cmd.Flags().Changed("from") {
		switch taskRecurFrom {
		case "due":
			recurMode = models.RecurModeFromDue
		case "completion":
			recurMode = models.RecurModeFromCompletion
		default:
			return fmt.Errorf("invalid --from value %q: must be 'due' or 'completion'", taskRecurFrom)
		}
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
	// --from only makes sense with a pattern argument or on its own to update an
	// existing pattern's mode. Silently ignoring it with --skip/--clear/--defer
	// would be confusing, so reject the combination explicitly.
	if cmd.Flags().Changed("from") && actionCount > 0 {
		return fmt.Errorf("cannot combine --from with %s", actionFlags[0])
	}

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	task, err := FindTaskByIdentifier(project, args[0])
	if err != nil {
		return err
	}

	// Every recurrence mutation is routed through the application service so it
	// commits atomically, revision-checked, and undoably via the workspace ledger.
	// The adapter keeps flag parsing, read-only prechecks, and presentation.
	svc := newWorkspaceAppService(workspace)
	locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}

	// Handle --skip flag
	if taskRecurSkip {
		if task.Recurrence == nil {
			return fmt.Errorf("task #%d is not recurring; set recurrence first with: tt task recur %d <pattern>", task.ShortID, task.ShortID)
		}
		res, skipErr := withResultRetry(func() (app.RecurrenceResult, error) {
			return svc.SkipRecurrenceByLocator(locator)
		})
		if skipErr != nil {
			return translateAppError(skipErr)
		}
		if out.JSON {
			return emitMutation(out, "", res.Task)
		}
		if !out.Quiet {
			fmt.Printf("Skipped: next due %s\n", recurDueString(res))
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
		res, deferErr := withResultRetry(func() (app.RecurrenceResult, error) {
			return svc.DeferRecurrenceByLocator(locator, taskRecurDefer)
		})
		if deferErr != nil {
			return translateAppError(deferErr)
		}
		if out.JSON {
			return emitMutation(out, "", res.Task)
		}
		if !out.Quiet {
			fmt.Printf("Deferred: now due %s\n", recurDueString(res))
		}
		return nil
	}

	// Handle --clear flag
	if taskRecurClear {
		if task.Recurrence == nil {
			// Read-only no-op: no mutation and no undo entry recorded.
			if out.JSON {
				return outputSingleTaskJSON(task)
			}
			if !out.Quiet {
				fmt.Printf("Task #%d has no recurrence pattern\n", task.ShortID)
			}
			return nil
		}
		res, clearErr := withResultRetry(func() (app.RecurrenceResult, error) {
			return svc.ClearRecurrenceByLocator(locator)
		})
		if clearErr != nil {
			return translateAppError(clearErr)
		}
		if out.JSON {
			return emitMutation(out, "", res.Task)
		}
		if !out.Quiet {
			fmt.Printf("Removed recurrence from task #%d (%s)\n", task.ShortID, res.Task.Title)
		}
		return nil
	}

	// Require pattern argument when not clearing/skipping/deferring,
	// unless --from is used alone to update the mode of an existing pattern.
	if len(args) < 2 {
		if cmd.Flags().Changed("from") && task.Recurrence != nil {
			// --from alone: update mode on existing recurrence
			oldPattern := task.Recurrence.String()
			res, modeErr := withResultRetry(func() (app.RecurrenceResult, error) {
				return svc.SetRecurrenceModeByLocator(locator, recurMode)
			})
			if modeErr != nil {
				return translateAppError(modeErr)
			}
			if out.JSON {
				return emitMutation(out, "", res.Task)
			}
			if !out.Quiet {
				fmt.Printf("Updated recurrence mode for task #%d: %s -> %s (%s)\n", task.ShortID, oldPattern, res.Pattern, res.Task.Title)
			}
			return nil
		}
		if cmd.Flags().Changed("from") && task.Recurrence == nil {
			return fmt.Errorf("task #%d has no recurrence pattern; set one first with: tt task recur %d <pattern>", task.ShortID, task.ShortID)
		}
		// Read-only recurrence display path.
		if out.JSON {
			return outputSingleTaskJSON(task)
		}
		if out.Quiet {
			return nil
		}
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
	// --from overrides any mode embedded in the pattern string (e.g. trailing ~)
	if cmd.Flags().Changed("from") {
		pattern.Mode = recurMode
	}

	oldPattern := ""
	if task.Recurrence != nil {
		oldPattern = task.Recurrence.String()
	}

	// Warn when setting monthly recurrence on a task whose due date falls on
	// day 29–31: those days don't exist in all months, so the recurrence will
	// silently clamp to the last day of shorter months. Yearly recurrence on
	// Feb 29 clamps to Feb 28 in non-leap years for the same reason. The warning
	// is derived from the loaded task's due date, which the mutation does not change.
	if pattern.Unit == models.RecurrenceMonthly && task.DueDate != nil && task.DueDate.Day() > 28 {
		fmt.Fprintf(os.Stderr, "warning: due date is on day %d; months with fewer days will use the last day instead\n", task.DueDate.Day())
	}
	if pattern.Unit == models.RecurrenceYearly && task.DueDate != nil &&
		task.DueDate.Month() == time.February && task.DueDate.Day() == 29 {
		fmt.Fprintf(os.Stderr, "warning: due date is Feb 29; non-leap years will use Feb 28 instead\n")
	}

	res, setErr := withResultRetry(func() (app.RecurrenceResult, error) {
		return svc.SetRecurrenceByLocator(locator, pattern)
	})
	if setErr != nil {
		return translateAppError(setErr)
	}

	if out.JSON {
		return emitMutation(out, "", res.Task)
	}
	if !out.Quiet {
		if oldPattern != "" {
			fmt.Printf("Updated recurrence for task #%d: %s -> %s (%s)\n", task.ShortID, oldPattern, res.Pattern, res.Task.Title)
		} else {
			fmt.Printf("Set recurrence for task #%d: %s (%s)\n", task.ShortID, res.Pattern, res.Task.Title)
		}
	}
	return nil
}

// recurDueString formats the post-mutation due date carried on a RecurrenceResult
// for CLI output, reusing the shared task due-date formatter.
func recurDueString(res app.RecurrenceResult) string {
	return formatTaskDueDate(&models.Task{DueDate: res.DueDate, HasTime: res.HasTime})
}
