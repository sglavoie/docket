package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// commitCheckInState commits one state transition and returns the result message
// the shell would receive. Committed evidence is deliberately no longer
// origin-tagged, so this runs the command exactly as the shell does and asserts
// nothing about ownership: the note it produces claims no keyboard.
func commitCheckInState(t *testing.T, model *AppModel, context operation.CommandContext, state models.WorkflowState) tea.Msg {
	t.Helper()
	result := model.operations.Executor.Execute(operation.OperationExecutionRequest{ID: operation.OperationTaskSetState, Context: context, Value: state, HasValue: true})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	cmd := result.Message.(operation.OperationCommandResultMsg).Cmd
	return runOriginTaggedCommand(originTaggedCommandMsg{cmd: cmd, epoch: model.interactionEpoch()})()
}

// requireCheckInNote fails unless the footer is saying exactly one check-in note
// and no surface was mounted to say it.
func requireCheckInNote(t *testing.T, model *AppModel, want string) {
	t.Helper()
	if model.operations.CheckInEditor != nil {
		t.Fatal("feedback opened the scheduling editor")
	}
	if got := strings.Count(model.feedback.message, "No check-in:"); got != 1 {
		t.Fatalf("footer = %q, want exactly one note", model.feedback.message)
	}
	if !strings.Contains(model.feedback.message, want) {
		t.Fatalf("footer = %q, want %q", model.feedback.message, want)
	}
}

func requireNoCheckInNote(t *testing.T, model *AppModel) {
	t.Helper()
	if model.operations.CheckInEditor != nil {
		t.Fatal("feedback opened the scheduling editor")
	}
	if strings.Contains(model.feedback.message, "No check-in:") {
		t.Fatalf("footer spoke for nothing: %q", model.feedback.message)
	}
}

// TestCheckInFeedbackNamesCommittedSubsetAndOpensNothing is the whole of what a
// transition into active work now does: the committed subset is named in the
// footer, nothing is mounted, and scheduling afterwards is still a separate
// explicit transaction with its own undo entry.
func TestCheckInFeedbackNamesCommittedSubsetAndOpensNothing(t *testing.T) {
	first, same, scheduled := newBulkPickerTestTask(t, 1, "first"), newBulkPickerTestTask(t, 2, "same"), newBulkPickerTestTask(t, 3, "scheduled")
	same.ApplyWorkflowState(models.StateDoing)
	next := time.Now().AddDate(0, 0, 8)
	scheduled.NextCheckIn = func() *time.Time {
		d := time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.Local)
		return &d
	}()
	env := newAppBulkPickerEnv(t, []*models.Task{first, same, scheduled})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	context := bulkTaskActionTestContext(env.projects[0], first, scheduled)
	model.Update(commitCheckInState(t, model, context, models.StateDoing))
	// Two tasks were committed; only the unscheduled one is the note's subject,
	// so a single task is named rather than counted.
	requireCheckInNote(t, model, "No check-in: first")

	// Explicit scheduling still reaches that task through its own controls.
	day := time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)
	executeCheckInValue(t, model, taskOperationTestContext(first, env.projects[0]), operation.OperationCheckInValue{
		Schedule: models.CheckInSchedule{Next: &day}, DateInput: "2027-01-01",
	})
	if stored := env.reload(t, 0).GetTaskByID(first.ID); stored.NextCheckIn == nil || stored.State() != models.StateDoing {
		t.Fatalf("save=%+v", stored)
	}
	env.undo(t)
	if stored := env.reload(t, 0).GetTaskByID(first.ID); stored.NextCheckIn != nil || stored.State() != models.StateDoing {
		t.Fatal("schedule undo changed the state commit")
	}
	env.undo(t)
	if stored := env.reload(t, 0).GetTaskByID(first.ID); stored.State() != models.StateBacklog {
		t.Fatal("state undo did not restore backlog")
	}
}

// TestCheckInFeedbackSuppressesRepeatsNoopsAndFailures covers everything that
// must produce no note at all, plus the two deliveries that must still produce
// one: a note takes no keyboard, so neither navigating away nor an untagged
// delivery silences it.
func TestCheckInFeedbackSuppressesRepeatsNoopsAndFailures(t *testing.T) {
	for _, mode := range []string{"navigation", "duplicate", "noop", "failure"} {
		t.Run(mode, func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "task")
			if mode == "noop" {
				task.ApplyWorkflowState(models.StateWaiting)
			}
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			msg := commitCheckInState(t, model, bulkTaskActionTestContext(env.projects[0], task), models.StateWaiting)
			if mode == "navigation" {
				model.beginCommandOriginUpdate(syntheticSpecialKey(tea.KeyDown))
			}
			if mode == "failure" {
				msg = coordinator.TransactionErrorMsg{Err: errors.New("refused")}
			}
			model.Update(msg)
			if mode == "noop" || mode == "failure" {
				requireNoCheckInNote(t, model)
				return
			}
			requireCheckInNote(t, model, "No check-in: task")
			if mode == "duplicate" {
				// Redelivering the same transaction says nothing further: the
				// suppression is by transaction, not by what the footer holds.
				model.clearStatus()
				model.Update(msg)
				requireNoCheckInNote(t, model)
			}
			stored := env.reload(t, 0).GetTaskByID(task.ID)
			if stored.State() != models.StateWaiting || stored.NextCheckIn != nil || stored.DueDate != nil {
				t.Fatal("feedback mutated the task")
			}
			env.undo(t)
			if env.reload(t, 0).GetTaskByID(task.ID).State() != models.StateBacklog {
				t.Fatal("feedback created an undo entry")
			}
		})
	}
}

// TestCheckInFeedbackOmitsConcurrentlyScheduledTargets pins the re-resolution:
// the note is derived from this commit's own subset, rechecked against storage,
// so a target another writer scheduled in the meantime is dropped rather than
// contradicted.
func TestCheckInFeedbackOmitsConcurrentlyScheduledTargets(t *testing.T) {
	first, second := newBulkPickerTestTask(t, 1, "first"), newBulkPickerTestTask(t, 2, "second")
	env := newAppBulkPickerEnv(t, []*models.Task{first, second})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	msg := commitCheckInState(t, model, bulkTaskActionTestContext(env.projects[0], first, second), models.StateDoing)
	next := time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)
	if _, err := model.domain.service.BulkSetCheckIn([]app.TaskLocator{taskref.For(second, env.projects[0]).Locator()}, models.CheckInSchedule{Next: &next}); err != nil {
		t.Fatal(err)
	}
	model.Update(msg)
	requireCheckInNote(t, model, "No check-in: first")
	if strings.Contains(model.feedback.message, "second") {
		t.Fatalf("note named a scheduled target: %q", model.feedback.message)
	}
}

// TestCheckInFeedbackCreationPathsAndOpenAfter covers compact and multiline
// capture and the full form, and holds the create-and-edit continuation: the
// note rides the creation and the external editor still opens, because nothing
// waits for anything any more.
func TestCheckInFeedbackCreationPathsAndOpenAfter(t *testing.T) {
	for _, path := range []string{"capture", "bulk", "form", "open-after"} {
		t.Run(path, func(t *testing.T) {
			env := newAppBulkPickerEnv(t, []*models.Task{})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			features := coordinator.NewFeatureCoordinators(model.domain.service)
			req := app.CaptureRequest{ProjectID: env.projects[0].ID, Title: "new", Destination: app.CaptureDestination(models.StateDoing)}
			var cmd tea.Cmd
			switch path {
			case "capture", "open-after":
				cmd = features.Capture.CaptureCreate(req, path == "open-after")
			case "bulk":
				second := req
				second.Title = "second"
				cmd = features.Capture.CaptureBulk([]app.CaptureRequest{req, second}, true)
			case "form":
				cmd = features.TaskForm.CreateForm(app.AddTaskRequest{ProjectID: req.ProjectID, Title: req.Title, Destination: req.Destination}, -1, false)
			}
			model.Update(runOriginTaggedCommand(originTaggedCommandMsg{cmd: cmd, epoch: model.interactionEpoch()})())
			want := "No check-in: new"
			if path == "bulk" {
				want = "No check-in: 2 tasks"
			}
			requireCheckInNote(t, model, want)
			if path != "open-after" {
				return
			}
			// The continuation is untouched: the editor's own return still lands,
			// and the note neither delayed it nor was re-said by it.
			stored := env.reload(t, 0).Tasks[0]
			model.clearStatus()
			model.Update(TaskContentEditedMsg{Task: stored, Project: env.projects[0], NewTitle: stored.Title})
			requireNoCheckInNote(t, model)
		})
	}
}

// TestCheckInFeedbackOpenAfterFailureKeepsItsOwnMessage holds the partial-capture
// recovery: a refused external edit still speaks for itself, and no scheduling
// text is layered over the error it has to show.
func TestCheckInFeedbackOpenAfterFailureKeepsItsOwnMessage(t *testing.T) {
	env := newAppBulkPickerEnv(t, []*models.Task{})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	features := coordinator.NewFeatureCoordinators(model.domain.service)
	cmd := features.Capture.CaptureCreate(app.CaptureRequest{ProjectID: env.projects[0].ID, Title: "new", Destination: app.CaptureDestination(models.StateDoing)}, true)
	model.Update(runOriginTaggedCommand(originTaggedCommandMsg{cmd: cmd, epoch: model.interactionEpoch()})())
	requireCheckInNote(t, model, "No check-in: new")

	model.clearStatus()
	model.Update(TaskContentEditedMsg{Err: errors.New("editor canceled")})
	requireNoCheckInNote(t, model)
	if len(env.reload(t, 0).Tasks) != 1 {
		t.Fatal("canceling the editor removed the committed creation")
	}
}

// TestCheckInFeedbackLineAndOutlineEdits covers the two commit paths that
// publish their own toast before any message travels, so the note has to be
// recorded at the commit and spoken from the one deferred closure.
func TestCheckInFeedbackLineAndOutlineEdits(t *testing.T) {
	for _, path := range []string{"line", "outline", "content"} {
		t.Run(path, func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "before")
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			state := app.CaptureDestination(models.StateDoing)
			switch path {
			case "line":
				cmd := model.secondaryFeatures.TaskItem.EditTaskLine(app.TaskLineEditRequest{Source: taskref.For(task, env.projects[0]).Locator(), Title: task.Title, Destination: &state}, nil)
				model.Update(runOriginTaggedCommand(originTaggedCommandMsg{cmd: cmd, epoch: model.interactionEpoch()})())
			case "content":
				model.features = coordinator.NewFeatureCoordinators(model.domain.service)
				model.Update(TaskContentEditedMsg{Task: task, Project: env.projects[0], NewTitle: task.Title + " $doing"})
			default:
				model.features = coordinator.NewFeatureCoordinators(model.domain.service)
				project, revision, err := model.features.Outline.Project(env.projects[0].ID)
				if err != nil {
					t.Fatal(err)
				}
				buffer, scope, err := dumpOutlineBuffer(project, nil, "Backlog", time.Now())
				if err != nil {
					t.Fatal(err)
				}
				buffer = strings.Replace(buffer, "$backlog", "$doing", 1) + "outline child $waiting\n"
				model.Update(OutlineEditedMsg{Project: project, Revision: revision, ScopeTaskIDs: scope, Buffer: buffer})
			}
			want := "No check-in: before"
			if path == "outline" {
				want = "No check-in: 2 tasks"
			}
			requireCheckInNote(t, model, want)
		})
	}
}

// TestCheckInFeedbackRidesTheLiveLineAndStandsAlone pins the footer composition:
// the note joins whatever line this Update published, ahead of the recovery
// controls so `u undo` keeps the tail, and publishes itself when nothing else
// spoke.
func TestCheckInFeedbackRidesTheLiveLineAndStandsAlone(t *testing.T) {
	for _, live := range []string{"recovery", "plain", "none"} {
		t.Run(live, func(t *testing.T) {
			cluster := &feedbackController{}
			switch live {
			case "recovery":
				cluster.message = formatRecoveryFeedback("Set Doing", "1 task", true)
			case "plain":
				cluster.message = "Captured 1 task"
			}
			appended := cluster.appendNote("No check-in: one · menu: Set up check-ins")
			if appended != (live != "none") {
				t.Fatalf("appendNote=%v with a %s line", appended, live)
			}
			switch live {
			case "recovery":
				if cluster.message != "Set Doing: 1 task · No check-in: one · menu: Set up check-ins · u undo · Ctrl+O origin" {
					t.Fatalf("recovery line = %q", cluster.message)
				}
			case "plain":
				if cluster.message != "Captured 1 task · No check-in: one · menu: Set up check-ins" {
					t.Fatalf("plain line = %q", cluster.message)
				}
			default:
				if cluster.message != "" {
					t.Fatalf("empty line invented text: %q", cluster.message)
				}
			}
		})
	}
}

// TestCheckInFeedbackUnderNestedOverlayStillSpeaks is the point of the change: a
// modal that owns the keyboard is not a reason to withhold a sentence, and it is
// never displaced by one either.
func TestCheckInFeedbackUnderNestedOverlayStillSpeaks(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	context := bulkTaskActionTestContext(env.projects[0], task)
	blocker, err := model.operations.NewCheckInEditor(model, operation.OperationExecutionRequest{ID: operation.OperationTaskSetCheckIn, Context: context})
	if err != nil {
		t.Fatal(err)
	}
	model.operations.CheckInEditor = blocker
	model.Update(commitCheckInState(t, model, context, models.StateDoing))
	if model.operations.CheckInEditor != blocker {
		t.Fatal("feedback replaced the mounted overlay")
	}
	if !strings.Contains(model.feedback.message, "No check-in: task") {
		t.Fatalf("footer under an overlay = %q", model.feedback.message)
	}
}

// TestCheckInFeedbackUnreadableCommittedProjectStaysSilent keeps the note honest
// about state it could not read: a failed reload reports nothing rather than
// naming a task out of the session cache.
func TestCheckInFeedbackUnreadableCommittedProjectStaysSilent(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	msg := commitCheckInState(t, model, bulkTaskActionTestContext(env.projects[0], task), models.StateDoing)
	if err := os.Remove(filepath.Join(env.projects[0].Path, "tasks.md")); err != nil {
		t.Fatal(err)
	}
	model.Update(msg)
	requireNoCheckInNote(t, model)
}

// TestCheckInFeedbackCommandOriginSurvivesDelayedExecution keeps session 9's
// guarantee for the state change itself: navigating away must not cancel an
// authorized commit, and the note follows that commit either way.
func TestCheckInFeedbackCommandOriginSurvivesDelayedExecution(t *testing.T) {
	for _, navigate := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "navigation"}[navigate], func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "task")
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			result := model.operations.Executor.Execute(operation.OperationExecutionRequest{ID: operation.OperationTaskSetState, Context: bulkTaskActionTestContext(env.projects[0], task), Value: models.StateDoing, HasValue: true})
			if result.Err != nil {
				t.Fatal(result.Err)
			}
			command := result.Message.(operation.OperationCommandResultMsg).Cmd
			pending := withCommandOrigin(command, model.interactionEpoch())()
			if navigate {
				model.beginCommandOriginUpdate(syntheticSpecialKey(tea.KeyDown))
			}
			_, execute := model.Update(pending)
			if env.reload(t, 0).GetTaskByID(task.ID).State() != models.StateBacklog {
				t.Fatal("scheduling command committed eagerly")
			}
			model.Update(execute())
			if env.reload(t, 0).GetTaskByID(task.ID).State() != models.StateDoing {
				t.Fatal("navigation canceled the authorized state change")
			}
			requireCheckInNote(t, model, "No check-in: task")
		})
	}
}
