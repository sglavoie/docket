package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestCaptureCreatesTaskAndEmitsOneChangeSet(t *testing.T) {
	project := newProject("proj-1", "Alpha")
	repo := newFakeRepo(project)
	expectedRev := repo.revisions[project.ID]
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID: "proj-1",
		Title:     "Write the report",
		Priority:  1,
		Tags:      []string{"#work", "urgent"},
		State:     models.StateNext,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	if res.TaskID != "gen-task-1" {
		t.Errorf("TaskID = %q, want gen-task-1", res.TaskID)
	}
	if res.ShortID != 1 {
		t.Errorf("ShortID = %d, want 1", res.ShortID)
	}
	if res.QualifiedRef != "alpha:#1" {
		t.Errorf("QualifiedRef = %q, want alpha:#1", res.QualifiedRef)
	}
	if res.State != models.StateNext {
		t.Errorf("State = %q, want next", res.State)
	}
	if res.Revision == "" || res.Revision == expectedRev {
		t.Errorf("Revision = %q, want a new non-empty revision (was %q)", res.Revision, expectedRev)
	}

	if got := len(project.Tasks); got != 1 {
		t.Fatalf("project has %d tasks, want 1", got)
	}
	task := project.Tasks[0]
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
	if !task.HasTag("work") || !task.HasTag("urgent") {
		t.Errorf("tags = %v, want work+urgent", task.Tags)
	}

	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatalf("expected exactly one changeset in one commit, got %v", repo.commits)
	}
	if cs.Project != project {
		t.Errorf("changeset project = %p, want %p", cs.Project, project)
	}
	if cs.ExpectedRevision != expectedRev {
		t.Errorf("ExpectedRevision = %q, want captured snapshot revision %q", cs.ExpectedRevision, expectedRev)
	}
	if cs.UndoAction != "Add Task" {
		t.Errorf("UndoAction = %q, want Add Task", cs.UndoAction)
	}
}

func TestCaptureRejectsEmptyTitle(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{ProjectID: "proj-1", Title: "   "})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("a rejected capture committed %d times, want 0", len(repo.commits))
	}
}

func TestCaptureRejectsInvalidTag(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{ProjectID: "proj-1", Title: "ok", Tags: []string{"bad,tag"}})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("a rejected capture committed %d times, want 0", len(repo.commits))
	}
}

func TestCaptureRejectsInvalidState(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{ProjectID: "proj-1", Title: "ok", State: models.WorkflowState("backlog")})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCaptureUnknownProject(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{ProjectID: "missing", Title: "ok"})
	if !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
}

func TestCaptureUnknownParent(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	missing := "no-such-parent"
	_, err := svc.Capture(app.CaptureRequest{ProjectID: "proj-1", Title: "ok", ParentID: &missing})
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestEditUpdatesTitleAndDescription(t *testing.T) {
	task := newTask("t1", 1, "old title")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	newTitle := "new title"
	newDesc := "a body"
	res, err := svc.Edit(app.EditRequest{TaskID: "t1", Title: &newTitle, Description: &newDesc})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if res.Title != "new title" {
		t.Errorf("Title = %q, want new title", res.Title)
	}
	if task.Description != "a body" {
		t.Errorf("Description = %q, want a body", task.Description)
	}
	if !task.UpdatedAt.Equal(time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("UpdatedAt = %v, want the injected clock time", task.UpdatedAt)
	}
}

func TestEditRejectsInvalidTitle(t *testing.T) {
	task := newTask("t1", 1, "old title")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	bad := "## heading"
	_, err := svc.Edit(app.EditRequest{TaskID: "t1", Title: &bad})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestTransitionMovesThroughNonterminalStates(t *testing.T) {
	task := newTask("t1", 1, "task")
	task.ApplyWorkflowState(models.StateInbox)
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Transition(app.StateRequest{TaskID: "t1", Target: models.StateInProgress})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if res.State != models.StateInProgress {
		t.Errorf("State = %q, want in_progress", res.State)
	}
	if task.State() != models.StateInProgress {
		t.Errorf("task state = %q, want in_progress", task.State())
	}
}

func TestTransitionAppliesEveryStateWithInjectedClock(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	staleCompletion := now.Add(-24 * time.Hour)
	snoozedUntil := now.Add(24 * time.Hour)

	for _, target := range models.AllWorkflowStates() {
		target := target
		t.Run(target.String(), func(t *testing.T) {
			task := newTask("t1", 1, "task")
			task.WorkflowState = models.StateReady
			if target == models.StateReady {
				task.WorkflowState = models.StateInbox
			}
			task.CompletedAt = &staleCompletion
			task.SnoozedUntil = &snoozedUntil
			task.Dimmed = true
			project := newProject("proj-1", "Alpha", task)
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			res, err := svc.Transition(app.StateRequest{TaskID: task.ID, Target: target})
			if err != nil {
				t.Fatalf("Transition(%q): %v", target, err)
			}
			if res.State != target || task.State() != target {
				t.Errorf("result/task state = %q/%q, want %q", res.State, task.State(), target)
			}
			if !task.UpdatedAt.Equal(now) {
				t.Errorf("UpdatedAt = %v, want injected time %v", task.UpdatedAt, now)
			}
			if task.SnoozedUntil != nil {
				t.Errorf("SnoozedUntil = %v, want nil", task.SnoozedUntil)
			}
			if target.IsTerminal() {
				if task.CompletedAt == nil || !task.CompletedAt.Equal(now) {
					t.Errorf("CompletedAt = %v, want injected time %v", task.CompletedAt, now)
				}
			} else if task.CompletedAt != nil {
				t.Errorf("CompletedAt = %v, want nil", task.CompletedAt)
			}
			if target == models.StateInbox && task.Dimmed {
				t.Error("inbox transition should clear Dimmed")
			}
		})
	}
}

func TestTransitionRejectsInvalidStateWithoutMutation(t *testing.T) {
	task := newTask("t1", 1, "task")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	before := task.State()

	_, err := svc.Transition(app.StateRequest{TaskID: task.ID, Target: models.WorkflowState("todo")})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("Transition invalid state error = %v, want ErrInvalidInput", err)
	}
	if task.State() != before || len(repo.commits) != 0 {
		t.Fatalf("invalid transition changed state/committed: state=%q commits=%d", task.State(), len(repo.commits))
	}
}

func TestTransitionRejectedFromTerminalState(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.Complete("t1"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	_, err := svc.Transition(app.StateRequest{TaskID: "t1", Target: models.StateInProgress})
	if !errors.Is(err, app.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
	var te *app.TransitionError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want *TransitionError", err)
	}
}

func TestReopenFromTerminalReturnsToInbox(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.Complete("t1"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	res, err := svc.Transition(app.StateRequest{TaskID: "t1", Target: models.StateInbox})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if res.State != models.StateInbox {
		t.Errorf("State = %q, want inbox", res.State)
	}
	if task.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil after reopen", task.CompletedAt)
	}
}

func TestCompleteBlockedByUnresolvedDependency(t *testing.T) {
	blocker := newTask("dep", 1, "blocker")
	blocker.ApplyWorkflowState(models.StateInbox)
	dependent := newTask("t2", 2, "dependent")
	dependent.DependsOn = []string{"dep"}
	project := newProject("proj-1", "Alpha", blocker, dependent)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Complete("t2")
	if !errors.Is(err, app.ErrDependencyBlocked) {
		t.Fatalf("err = %v, want ErrDependencyBlocked", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("blocked completion committed %d times, want 0", len(repo.commits))
	}
}

func TestCompleteGeneratesRecurringInstance(t *testing.T) {
	task := newTask("t1", 1, "daily standup")
	due := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	task.DueDate = &due
	if err := task.SetRecurrence(models.ParseRecurrence("daily")); err != nil {
		t.Fatalf("SetRecurrence: %v", err)
	}
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Complete("t1")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if task.State() != models.StateDone {
		t.Errorf("completed task state = %q, want done", task.State())
	}
	fixedNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if task.CompletedAt == nil || !task.CompletedAt.Equal(fixedNow) || !task.UpdatedAt.Equal(fixedNow) {
		t.Errorf("completed timestamps = %v/%v, want injected time %v", task.CompletedAt, task.UpdatedAt, fixedNow)
	}
	if res.RecurringGenerated != 1 {
		t.Errorf("RecurringGenerated = %d, want 1", res.RecurringGenerated)
	}
	if len(project.Tasks) != 2 {
		t.Fatalf("project has %d tasks, want 2 (original + generated instance)", len(project.Tasks))
	}
	generated := project.Tasks[0]
	if generated.State() != models.StateInbox {
		t.Errorf("generated state = %q, want inbox", generated.State())
	}
	if !generated.CreatedAt.Equal(fixedNow) || !generated.UpdatedAt.Equal(fixedNow) {
		t.Errorf("generated timestamps = %v/%v, want injected time %v", generated.CreatedAt, generated.UpdatedAt, fixedNow)
	}
}

func TestSetPriorityValidatesRange(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SetPriority("t1", 99); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SetPriority("t1", 1); err != nil {
		t.Fatalf("SetPriority(1): %v", err)
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
}

func TestSetDueDateSetsAndClears(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	due := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	if _, err := svc.SetDueDate("t1", &due, true); err != nil {
		t.Fatalf("SetDueDate: %v", err)
	}
	if task.DueDate == nil || !task.DueDate.Equal(due) || !task.HasTime {
		t.Fatalf("due=%v hasTime=%v, want set with time", task.DueDate, task.HasTime)
	}
	if _, err := svc.SetDueDate("t1", nil, false); err != nil {
		t.Fatalf("clear SetDueDate: %v", err)
	}
	if task.DueDate != nil || task.HasTime {
		t.Errorf("due=%v hasTime=%v, want cleared", task.DueDate, task.HasTime)
	}
}

func TestAddAndRemoveTag(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AddTag("t1", "#backend"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	if !task.HasTag("backend") {
		t.Errorf("tags = %v, want backend", task.Tags)
	}
	if _, err := svc.RemoveTag("t1", "backend"); err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}
	if task.HasTag("backend") {
		t.Errorf("tags = %v, want backend removed", task.Tags)
	}
}

func TestSetPinnedTogglesPin(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SetPinned("t1", true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if !task.Pinned {
		t.Error("task not pinned")
	}
	if _, err := svc.SetPinned("t1", false); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if task.Pinned {
		t.Error("task still pinned")
	}
}

func TestSnoozeAndClear(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.Snooze("t1", &until); err != nil {
		t.Fatalf("Snooze: %v", err)
	}
	if task.SnoozedUntil == nil || !task.SnoozedUntil.Equal(until) {
		t.Fatalf("SnoozedUntil = %v, want %v", task.SnoozedUntil, until)
	}
	if _, err := svc.Snooze("t1", nil); err != nil {
		t.Fatalf("clear Snooze: %v", err)
	}
	if task.SnoozedUntil != nil {
		t.Errorf("SnoozedUntil = %v, want cleared", task.SnoozedUntil)
	}
}

func TestParkAndUnpark(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.Park("t1", "resume at step 3"); err != nil {
		t.Fatalf("Park: %v", err)
	}
	if !task.IsParked() {
		t.Fatal("task not parked")
	}
	if _, err := svc.Unpark("t1"); err != nil {
		t.Fatalf("Unpark: %v", err)
	}
	if task.IsParked() {
		t.Error("task still parked")
	}
}

// TestParkRejectsCompletedTask proves the completed-task guard now lives in the
// application service: parking a done task is rejected as invalid input and the
// task is left untouched, matching the legacy CLI edge check.
func TestParkRejectsCompletedTask(t *testing.T) {
	task := newTask("t1", 1, "task")
	task.Complete()
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	_, err := svc.Park("t1", "resume later")
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if task.IsParked() {
		t.Error("completed task should not be parked")
	}
}

// TestSnoozeRejectsCompletedAndInProgress proves the snooze-set guards moved into
// the application service: snoozing a done or an in-progress task is rejected,
// while clearing a snooze remains allowed on any task.
func TestSnoozeRejectsCompletedAndInProgress(t *testing.T) {
	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	done := newTask("t1", 1, "task")
	done.Complete()
	inProgress := newTask("t2", 2, "task")
	inProgress.Start()
	repo := newFakeRepo(newProject("proj-1", "Alpha", done, inProgress))
	svc := newTestService(repo)

	if _, err := svc.Snooze("t1", &until); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("snooze done: err = %v, want ErrInvalidInput", err)
	}
	if done.SnoozedUntil != nil {
		t.Error("completed task should not be snoozed")
	}
	if _, err := svc.Snooze("t2", &until); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("snooze in-progress: err = %v, want ErrInvalidInput", err)
	}
	if inProgress.SnoozedUntil != nil {
		t.Error("in-progress task should not be snoozed")
	}
}

// TestToggleDimmedAndUndim proves the dim use cases flip the dimmed flag and that
// Undim clears it (discarding, not toggling).
func TestToggleDimmedAndUndim(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.ToggleDimmed("t1"); err != nil {
		t.Fatalf("ToggleDimmed: %v", err)
	}
	if !task.Dimmed {
		t.Fatal("task should be dimmed after toggle")
	}
	if _, err := svc.Undim("t1"); err != nil {
		t.Fatalf("Undim: %v", err)
	}
	if task.Dimmed {
		t.Error("task should not be dimmed after undim")
	}
}

// TestApplyAndClearTypeAttrs proves the type-attribute use cases apply and clear
// type-specific fields, and that an invalid value is rejected before any commit.
func TestApplyAndClearTypeAttrs(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.ApplyTypeAttrs("t1", models.TypeAttrValues{TaskEffort: "small"}); err != nil {
		t.Fatalf("ApplyTypeAttrs: %v", err)
	}
	if task.TaskAttrs == nil || task.TaskAttrs.Effort != "small" {
		t.Fatalf("TaskAttrs = %+v, want effort small", task.TaskAttrs)
	}

	if _, err := svc.ApplyTypeAttrs("t1", models.TypeAttrValues{TaskEffort: "gigantic"}); err == nil {
		t.Fatal("expected invalid effort to be rejected")
	}

	if _, err := svc.ClearTypeAttrs("t1"); err != nil {
		t.Fatalf("ClearTypeAttrs: %v", err)
	}
	if task.TaskAttrs != nil {
		t.Errorf("TaskAttrs = %+v, want cleared", task.TaskAttrs)
	}
}

func TestMutationTaskNotFound(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	if _, err := svc.SetPriority("ghost", 2); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestOptimisticConcurrencyConflictSurfacesStaleRevision(t *testing.T) {
	task := newTask("t1", 1, "task")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// Simulate a concurrent writer advancing the project's revision after the
	// use case captured its snapshot revision but before it commits.
	repo.preCommit = func() {
		repo.revisions[project.ID] = "diverged-revision"
	}

	_, err := svc.SetPriority("t1", 2)
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("err = %v, want storage.ErrStaleRevision", err)
	}
}

func TestCommitCarriesCapturedExpectedRevision(t *testing.T) {
	task := newTask("t1", 1, "task")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	captured := repo.revisions[project.ID]
	capturedBase := repo.documents[project.ID]
	svc := newTestService(repo)

	if _, err := svc.AddTag("t1", "focus"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatalf("want exactly one changeset in one commit, got %v", repo.commits)
	}
	if cs.ExpectedRevision != captured {
		t.Errorf("ExpectedRevision = %q, want %q", cs.ExpectedRevision, captured)
	}
	if cs.Base == nil {
		t.Fatal("ChangeSet.Base is nil; want exact loaded ProjectDocument")
	}
	if cs.Base != capturedBase {
		t.Fatal("ChangeSet.Base is not the document retained by the snapshot")
	}
	if len(cs.Base.Tasks[0].Tags) != 0 {
		t.Fatalf("snapshot base was mutated with local project: tags=%v", cs.Base.Tasks[0].Tags)
	}
	if cs.UndoAction != "Add Tag" {
		t.Errorf("UndoAction = %q, want Add Tag", cs.UndoAction)
	}
}
