package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestTaskResultTransactionIDPropagationFromFakeRepository(t *testing.T) {
	project := newProject("proj-1", "Alpha")
	repo := newFakeRepo(project)
	expectedRev := repo.revisions[project.ID]
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID:   "proj-1",
		Title:       "Write the report",
		Priority:    1,
		Tags:        []string{"#work", "urgent"},
		Context:     " Client-Acme ",
		Destination: app.CaptureDestinationBacklog,
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
	if res.Revision == "" || res.Revision == expectedRev {
		t.Errorf("Revision = %q, want a new non-empty revision (was %q)", res.Revision, expectedRev)
	}
	if res.TransactionID == "" {
		t.Fatal("TransactionID is empty; fake repository result was not propagated")
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
	if task.Context != "client-acme" {
		t.Errorf("context = %q, want client-acme", task.Context)
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

	_, err := svc.Capture(app.CaptureRequest{ProjectID: "proj-1", Title: "ok", Tags: []string{"!!!"}})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("a rejected capture committed %d times, want 0", len(repo.commits))
	}
}

func TestCaptureCanonicalizesLegacyMarkerSpellingAsOrdinaryTag(t *testing.T) {
	project := newProject("proj-1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Fix", Tags: []string{"type:BUG", "backend"}})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got := project.GetTaskByID(res.TaskID).Tags; !slices.Equal(got, []string{"type-bug", "backend"}) {
		t.Fatalf("tags = %v, want [type-bug backend]", got)
	}
}

func TestCaptureAllowsIndependentSemanticTags(t *testing.T) {
	project := newProject("proj-1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Both", Tags: []string{"bug", "idea"}})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got := project.GetTaskByID(res.TaskID).Tags; !slices.Equal(got, []string{"bug", "idea"}) || len(repo.commits) != 1 {
		t.Fatalf("semantic tags = %v commits=%d", got, len(repo.commits))
	}
}

func TestCaptureRejectsInvalidDestination(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{ProjectID: "proj-1", Title: "ok", Destination: app.CaptureDestination("invalid")})
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
	res, err := svc.Edit(app.EditRequest{Locator: repo.loc("t1"), Title: &newTitle, Description: &newDesc})
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
	_, err := svc.Edit(app.EditRequest{Locator: repo.loc("t1"), Title: &bad})
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

	_, err := svc.Transition(app.StateRequest{Locator: repo.loc("t1"), Target: models.StateDoing})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if task.State() != models.StateDoing {
		t.Errorf("task state = %q, want in_progress", task.State())
	}
}

func TestTransitionAppliesEveryStateWithInjectedClock(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	staleCompletion := now.Add(-24 * time.Hour)
	deferredUntil := now.Add(24 * time.Hour)

	for _, target := range models.AllWorkflowStates() {
		target := target
		t.Run(target.String(), func(t *testing.T) {
			task := newTask("t1", 1, "task")
			task.WorkflowState = models.StateBacklog
			if target == models.StateBacklog {
				task.WorkflowState = models.StateInbox
			}
			task.CompletedAt = &staleCompletion
			task.DeferredUntil = &deferredUntil
			project := newProject("proj-1", "Alpha", task)
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			_, err := svc.Transition(app.StateRequest{Locator: repo.loc(task.ID), Target: target})
			if err != nil {
				t.Fatalf("Transition(%q): %v", target, err)
			}
			if task.State() != target {
				t.Errorf("task state = %q, want %q", task.State(), target)
			}
			if !task.UpdatedAt.Equal(now) {
				t.Errorf("UpdatedAt = %v, want injected time %v", task.UpdatedAt, now)
			}
			if task.DeferredUntil != nil {
				t.Errorf("DeferredUntil = %v, want nil", task.DeferredUntil)
			}
			if target.IsTerminal() {
				if task.CompletedAt == nil || !task.CompletedAt.Equal(now) {
					t.Errorf("CompletedAt = %v, want injected time %v", task.CompletedAt, now)
				}
			} else if task.CompletedAt != nil {
				t.Errorf("CompletedAt = %v, want nil", task.CompletedAt)
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

	_, err := svc.Transition(app.StateRequest{Locator: repo.loc(task.ID), Target: models.WorkflowState("todo")})
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

	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc("t1"), Target: models.StateDone}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	_, err := svc.Transition(app.StateRequest{Locator: repo.loc("t1"), Target: models.StateDoing})
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

	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc("t1"), Target: models.StateDone}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	_, err := svc.Transition(app.StateRequest{Locator: repo.loc("t1"), Target: models.StateInbox})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if task.State() != models.StateInbox {
		t.Errorf("task state = %q, want inbox", task.State())
	}
	if task.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil after reopen", task.CompletedAt)
	}
}

func TestSetPriorityValidatesRange(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SetPriority(repo.loc("t1"), 99); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SetPriority(repo.loc("t1"), -1); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SetPriority(repo.loc("t1"), 1); err != nil {
		t.Fatalf("SetPriority(1): %v", err)
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
	// Zero is the unset value, not an out-of-range priority, so it clears.
	if _, err := svc.SetPriority(repo.loc("t1"), 0); err != nil {
		t.Fatalf("SetPriority(0): %v", err)
	}
	if task.Priority != 0 {
		t.Errorf("priority = %d, want 0", task.Priority)
	}
}

func TestSetDueDateSetsAndClears(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	due := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	if _, err := svc.SetDueDate(repo.loc("t1"), &due); err != nil {
		t.Fatalf("SetDueDate: %v", err)
	}
	if task.DueDate == nil || task.DueDate.Format("2006-01-02") != due.Format("2006-01-02") {
		t.Fatalf("due=%v, want date %s", task.DueDate, due.Format("2006-01-02"))
	}
	if _, err := svc.SetDueDate(repo.loc("t1"), nil); err != nil {
		t.Fatalf("clear SetDueDate: %v", err)
	}
	if task.DueDate != nil {
		t.Errorf("due=%v, want cleared", task.DueDate)
	}
}

func TestToggleTagAddsAndRemoves(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "#backend"); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	if !task.HasTag("backend") {
		t.Errorf("tags = %v, want backend", task.Tags)
	}
	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "backend"); err != nil {
		t.Fatalf("toggle off: %v", err)
	}
	if task.HasTag("backend") {
		t.Errorf("tags = %v, want backend removed", task.Tags)
	}
}

func TestToggleTagAllowsIndependentSemanticTags(t *testing.T) {
	task := newTask("t1", 1, "task")
	task.Tags = []string{"idea", "keep"}
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "bug"); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	if !slices.Equal(task.Tags, []string{"idea", "keep", "bug"}) || len(repo.commits) != 1 {
		t.Fatalf("independent add tags=%v commits=%d", task.Tags, len(repo.commits))
	}
}

func TestSetPinnedTogglesPin(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SetPinned(repo.loc("t1"), true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if !task.Pinned {
		t.Error("task not pinned")
	}
	if _, err := svc.SetPinned(repo.loc("t1"), false); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if task.Pinned {
		t.Error("task still pinned")
	}
}

func TestDeferAndClear(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.Defer(repo.loc("t1"), &until); err != nil {
		t.Fatalf("Defer: %v", err)
	}
	if task.DeferredUntil == nil || !task.DeferredUntil.Equal(until) {
		t.Fatalf("DeferredUntil = %v, want %v", task.DeferredUntil, until)
	}
	if task.DeferCount != 1 {
		t.Fatalf("DeferCount = %d, want 1", task.DeferCount)
	}
	if _, err := svc.Defer(repo.loc("t1"), nil); err != nil {
		t.Fatalf("clear Defer: %v", err)
	}
	if task.DeferredUntil != nil {
		t.Errorf("DeferredUntil = %v, want cleared", task.DeferredUntil)
	}
	if task.DeferCount != 1 {
		t.Errorf("clear changed DeferCount to %d, want 1", task.DeferCount)
	}
}

func TestCaptureWithDeferralStartsTheCount(t *testing.T) {
	project := newProject("proj-1", "Alpha")
	repo := newFakeRepo(project)
	until := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	if _, err := newTestService(repo).Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Later",
		DeferSet:  true,
		Defer:     &until,
	}); err != nil {
		t.Fatalf("Capture deferred task: %v", err)
	}
	if len(project.Tasks) != 1 {
		t.Fatalf("captured tasks = %d, want 1", len(project.Tasks))
	}
	if project.Tasks[0].DeferCount != 1 {
		t.Fatalf("captured defer count = %d, want 1", project.Tasks[0].DeferCount)
	}
}

// TestDeferRejectsCompletedAndBacklogsDoing proves terminal work is rejected
// while active work is normalized to Backlog before being hidden.
func TestDeferRejectsCompletedAndBacklogsDoing(t *testing.T) {
	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	done := newTask("t1", 1, "task")
	done.Complete()
	inProgress := newTask("t2", 2, "task")
	inProgress.Start()
	repo := newFakeRepo(newProject("proj-1", "Alpha", done, inProgress))
	svc := newTestService(repo)

	if _, err := svc.Defer(repo.loc("t1"), &until); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("deferral done: err = %v, want ErrInvalidInput", err)
	}
	if done.DeferredUntil != nil {
		t.Error("completed task should not be deferred")
	}
	if _, err := svc.Defer(repo.loc("t2"), &until); err != nil {
		t.Fatalf("deferral Doing: %v", err)
	}
	if inProgress.State() != models.StateBacklog || inProgress.DeferredUntil == nil {
		t.Fatalf("deferred Doing task = state %s date %v, want Backlog with date", inProgress.State(), inProgress.DeferredUntil)
	}
}

func TestMutationTaskNotFound(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	if _, err := svc.SetPriority(app.TaskLocator{ProjectID: "proj-1", TaskID: "ghost"}, 2); !errors.Is(err, app.ErrTaskNotFound) {
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

	_, err := svc.SetPriority(repo.loc("t1"), 2)
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

	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "focus"); err != nil {
		t.Fatalf("toggle on: %v", err)
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
