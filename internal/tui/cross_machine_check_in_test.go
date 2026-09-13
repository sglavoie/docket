package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// This role runs only in a forked helper, at the same pre-commit barrier as
// deadline stepping. Every judge below reloads the final document and ledger.
func TestCrossMachineHelperChangesCheckIn(t *testing.T) {
	env := openHelperWorkspace(t)
	target := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	kind := helperEnv(t, helperMutationEnv)
	action := "Set Check-In"
	if kind == "done" || kind == "canceled" {
		action = "Change State"
	}
	defer helperHoldAtCommit(t, action, target.ProjectID, target.TaskID)()
	var err error
	switch kind {
	case "schedule":
		day := time.Date(2040, 4, 20, 0, 0, 0, 0, time.Local)
		_, err = env.service.BulkSetCheckIn([]app.TaskLocator{target}, models.CheckInSchedule{Next: &day, RepeatDays: 14})
	case "done", "canceled":
		_, err = env.service.Transition(app.StateRequest{Locator: target, Target: models.WorkflowState(kind)})
	default:
		t.Fatalf("unknown check-in race action %q", kind)
	}
	if err != nil {
		helperRecordRefusal(t, action, err)
		return
	}
	helperRecord(t, "committed")
}

func checkInRaceRole(name, kind string) crossMachineRole {
	return crossMachineRole{name: name, role: "TestCrossMachineHelperChangesCheckIn", env: []string{
		helperTitleEnv + "=" + helperRaceTitleB, helperMutationEnv + "=" + kind,
	}}
}

func checkInReviewRole(name string) crossMachineRole {
	return crossMachineRole{name: name, role: "TestCrossMachineHelperAppliesTaskMutation", env: []string{
		helperTitleEnv + "=" + helperRaceTitleB, helperMutationEnv + "=" + helperMutationReview,
	}}
}

func seedCheckInRace(project *models.Project) {
	for _, task := range project.Tasks {
		if task.Title != helperRaceTitleB {
			continue
		}
		task.ApplyWorkflowState(models.StateWaiting)
		day := time.Date(2020, 4, 1, 0, 0, 0, 0, time.Local)
		task.NextCheckIn, task.CheckInRepeatDays = &day, 7
	}
}

func requireCheckInRaceConflict(t *testing.T, race crossMachineInterruptedResult) {
	t.Helper()
	if !strings.HasPrefix(race.held, "conflicted:") {
		t.Fatalf("held outcome = %q", race.held)
	}
	records, err := storage.NewConflictStore(config.ConflictsDir(race.dataDir)).List()
	if err != nil || len(records) != 1 {
		t.Fatalf("conflicts = %v, %v", records, err)
	}
	if got := crossMachineUndoCount(t, race.dataDir); got != race.undoBefore+1 {
		t.Fatalf("refusal wrote undo: %d", got)
	}
	requireCrossMachinePlanningUnchanged(t, race)
}

func TestCrossMachineCheckInReviewVersusReschedule(t *testing.T) {
	for _, reviewHeld := range []bool{true, false} {
		name := "schedule-held"
		held, writer := checkInRaceRole("held", "schedule"), checkInReviewRole("writer")
		if reviewHeld {
			name = "review-held"
			held, writer = checkInReviewRole("held"), checkInRaceRole("writer", "schedule")
		}
		t.Run(name, func(t *testing.T) {
			race := runCrossMachineInterruptedMutation(t, held, writer, seedCheckInRace)
			requireCheckInRaceConflict(t, race)
			task := crossMachineTask(t, race.merged, helperRaceTitleB)
			if reviewHeld {
				if task.NextCheckIn.Format(time.DateOnly) != "2040-04-20" || task.CheckInRepeatDays != 14 || task.LastReviewedAt != nil || !task.Review {
					t.Fatalf("reschedule acknowledged or mixed review: %+v", task)
				}
			} else {
				if task.LastReviewedAt == nil || task.Review || task.CheckInRepeatDays != 7 || task.NextCheckIn.Format(time.DateOnly) != task.LastReviewedAt.In(time.Local).AddDate(0, 0, 7).Format(time.DateOnly) {
					t.Fatalf("review mixed reschedule: %+v", task)
				}
			}
		})
	}
}

func TestCrossMachineCheckInContentEditMerges(t *testing.T) {
	for _, scheduleHeld := range []bool{true, false} {
		held, writer := checkInRaceRole("held", "schedule"), crossMachineRole{name: "writer", role: "TestCrossMachineHelperSetsResumeNote", env: []string{helperTitleEnv + "=" + helperRaceTitleB, helperNoteEnv + "=Keep the vendor evidence"}}
		if !scheduleHeld {
			held, writer = writer, held
			held.name, writer.name = "held", "writer"
		}
		t.Run(held.role, func(t *testing.T) {
			race := runCrossMachineInterruptedMutation(t, held, writer, seedCheckInRace)
			if race.held != "committed" {
				t.Fatal(race.held)
			}
			task := crossMachineTask(t, race.merged, helperRaceTitleB)
			if task.ResumeNote != "Keep the vendor evidence" || task.NextCheckIn.Format(time.DateOnly) != "2040-04-20" || task.CheckInRepeatDays != 14 || !task.Review || task.LastReviewedAt != nil {
				t.Fatalf("independent metadata lost: %+v", task)
			}
			if crossMachineUndoCount(t, race.dataDir) != race.undoBefore+2 {
				t.Fatal("missing independent undo")
			}
			requireCrossMachinePlanningUnchanged(t, race)
		})
	}
}

func TestCrossMachineCheckInTerminalVersusReminder(t *testing.T) {
	for _, state := range []string{"done", "canceled"} {
		for _, terminalHeld := range []bool{true, false} {
			held, writer := checkInRaceRole("held", "schedule"), checkInRaceRole("writer", state)
			name := state + "-schedule-held"
			if terminalHeld {
				held, writer = checkInRaceRole("held", state), checkInRaceRole("writer", "schedule")
				name = state + "-terminal-held"
			}
			t.Run(name, func(t *testing.T) {
				race := runCrossMachineInterruptedMutation(t, held, writer, seedCheckInRace)
				requireCheckInRaceConflict(t, race)
				task := crossMachineTask(t, race.merged, helperRaceTitleB)
				if terminalHeld {
					if task.IsTerminalState() || task.NextCheckIn == nil || task.CheckInRepeatDays != 14 {
						t.Fatal("schedule winner was overwritten")
					}
				} else if task.State() != models.WorkflowState(state) || !task.CheckIn().IsEmpty() {
					t.Fatal("terminal winner retained a reminder")
				}
			})
		}
	}
}

func TestCrossMachineCheckInTerminalVersusReview(t *testing.T) {
	for _, state := range []string{"done", "canceled"} {
		for _, reviewHeld := range []bool{true, false} {
			held, writer := checkInReviewRole("held"), checkInRaceRole("writer", state)
			name := state + "-review-held"
			if !reviewHeld {
				held, writer = checkInRaceRole("held", state), checkInReviewRole("writer")
				name = state + "-terminal-held"
			}
			t.Run(name, func(t *testing.T) {
				race := runCrossMachineInterruptedMutation(t, held, writer, seedCheckInRace)
				requireCheckInRaceConflict(t, race)
				task := crossMachineTask(t, race.merged, helperRaceTitleB)
				if reviewHeld {
					if task.State() != models.WorkflowState(state) || !task.CheckIn().IsEmpty() || task.LastReviewedAt != nil {
						t.Fatal("refused review changed terminal winner")
					}
				} else if task.IsTerminalState() || task.LastReviewedAt == nil || task.CheckInRepeatDays != 7 || task.Review {
					t.Fatal("refused terminal transition changed reviewed winner")
				}
			})
		}
	}
}

func TestCrossMachineCheckInVersusDeadlineStep(t *testing.T) {
	for _, stepHeld := range []bool{true, false} {
		held, writer := checkInRaceRole("held", "schedule"), dueStepRole("writer", "", helperRaceTitleB)
		name := "schedule-held"
		if stepHeld {
			held, writer = dueStepRole("held", "", helperRaceTitleB), checkInRaceRole("writer", "schedule")
			name = "step-held"
		}
		t.Run(name, func(t *testing.T) {
			race := runCrossMachineInterruptedMutation(t, held, writer, seedCheckInRace)
			task := crossMachineTask(t, race.merged, helperRaceTitleB)
			if task.NextCheckIn == nil || task.NextCheckIn.Format(time.DateOnly) != "2040-04-20" || task.CheckInRepeatDays != 14 {
				t.Fatal("deadline stepping changed the check-in")
			}
			if stepHeld {
				if race.held != "expired" || task.DueDate != nil {
					t.Fatal("stale deadline step was not refused whole")
				}
				requireCrossMachineStagedNothing(t, race, "Set Check-In")
				requireCrossMachinePlanningUnchanged(t, race)
			} else if race.held != "committed" || task.DueDate == nil || crossMachineUndoCount(t, race.dataDir) != race.undoBefore+2 {
				t.Fatal("independent schedule did not merge with deadline")
			}
		})
	}
}
