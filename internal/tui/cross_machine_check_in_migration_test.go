package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// This helper must not load configuration: doing so would migrate the old
// binary's workspace before that binary has validated the seeded document.
func TestCrossMachineHelperSeedsLegacyCheckIns(t *testing.T) {
	root := os.Getenv(helperDataDirEnv)
	if root == "" {
		t.Skip("forked legacy seeding role")
	}
	path := filepath.Join(root, "projects", "unfiled", "tasks.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := storage.ParseV17ProjectDocument(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	p := &models.Project{Path: t.TempDir(), Name: "Legacy seed", NextShortID: 1}
	task := newFixtureTask(p, "Legacy waiting check-in")
	task.ApplyWorkflowState(models.StateWaiting)
	due := time.Date(2020, 4, 1, 15, 30, 0, 0, time.Local)
	task.DueDate = &due
	newFixtureTask(p, "Legacy doing setup").ApplyWorkflowState(models.StateDoing)
	if err = storage.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(storage.TasksFilePath(p.Path))
	if err != nil {
		t.Fatal(err)
	}
	current, err := storage.ParseV20ProjectDocument(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range current.Tasks {
		legacy.Tasks = append(legacy.Tasks, record.V16TaskRecord)
	}
	body, err := storage.WriteV17ProjectDocument(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(body), storage.FilePermissions); err != nil {
		t.Fatal(err)
	}
}

func TestCrossMachineHelperReviewsMigratedCheckIns(t *testing.T) {
	env := openHelperWorkspace(t)
	target := helperTaskLocator(t, env, "Legacy waiting check-in")
	before := helperStoredTask(t, env, target)
	if before.NextCheckIn == nil || before.NextCheckIn.Format(time.DateOnly) != "2020-04-01" || before.DueDate.Format(helperDeadlineLayout) != "2020-04-01 15:30" || before.CheckInRepeatDays != 0 || before.LastReviewedAt != nil {
		t.Fatalf("migration lost or invented metadata: %+v", before)
	}
	if _, err := env.service.MarkReviewed(target); err != nil {
		t.Fatal(err)
	}
	reviewed := helperStoredTask(t, env, target)
	if !reviewed.CheckIn().IsEmpty() || reviewed.LastReviewedAt == nil || !reviewed.StateChangedAt.Equal(before.StateChangedAt) || !reviewed.DueDate.Equal(*before.DueDate) {
		t.Fatal("migrated one-off review changed the deadline or state age")
	}
	candidates, err := env.service.CheckInSetupCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("setup candidates = %d", len(candidates))
	}
	day := time.Date(2040, 4, 20, 0, 0, 0, 0, time.Local)
	if _, err = env.service.BulkSetCheckIn([]app.TaskLocator{target}, models.CheckInSchedule{Next: &day, RepeatDays: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err = env.service.MarkReviewed(target); err != nil {
		t.Fatal(err)
	}
	recurring := helperStoredTask(t, env, target)
	if recurring.CheckInRepeatDays != 3 || recurring.NextCheckIn.Format(time.DateOnly) != recurring.LastReviewedAt.In(time.Local).AddDate(0, 0, 3).Format(time.DateOnly) {
		t.Fatal("early recurrence is not based on the actual review day")
	}
	for _, state := range []models.WorkflowState{models.StateDone, models.StateInbox} {
		if _, err = env.service.Transition(app.StateRequest{Locator: target, Target: state}); err != nil {
			t.Fatal(err)
		}
		stored := helperStoredTask(t, env, target)
		if !stored.CheckIn().IsEmpty() || stored.LastReviewedAt == nil || !stored.DueDate.Equal(*before.DueDate) {
			t.Fatal("completion/reopen lost history or revived reminder")
		}
	}
}
