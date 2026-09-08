package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// seedResumeNoteTask seeds one task carrying a stale touch time, so every test
// below can tell a real write from a no-op by whether UpdatedAt moved.
func seedResumeNoteTask(t *testing.T) (*app.Service, *fakeRepo, *models.Task, time.Time) {
	t.Helper()
	stale := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	task := newTask("t1", 1, "Wire the importer")
	task.ApplyWorkflowState(models.StateDoing)
	task.UpdatedAt = stale
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	return newTestService(repo), repo, task, stale
}

// TestSetResumeNoteWritesEditsAndClears walks the whole life of one note
// through the single entry point, because create, replace, and clear are one
// operation rather than three: absence is the only spelling of "no note".
func TestSetResumeNoteWritesEditsAndClears(t *testing.T) {
	svc, repo, task, _ := seedResumeNoteTask(t)

	result, err := svc.SetResumeNote(repo.loc("t1"), "stopped at the CSV column mapping")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !result.Changed || task.ResumeNote != "stopped at the CSV column mapping" {
		t.Fatalf("create = changed %v note %q", result.Changed, task.ResumeNote)
	}
	if result.Task.TransactionID == "" {
		t.Fatal("create committed no transaction")
	}

	if _, err = svc.SetResumeNote(repo.loc("t1"), "waiting on the vendor schema"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if task.ResumeNote != "waiting on the vendor schema" {
		t.Fatalf("replace note = %q", task.ResumeNote)
	}

	cleared, err := svc.SetResumeNote(repo.loc("t1"), "")
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if !cleared.Changed || task.ResumeNote != "" {
		t.Fatalf("clear = changed %v note %q, want a committed empty note", cleared.Changed, task.ResumeNote)
	}
	// The clear names itself in the ledger, so `u` offers to put the note back
	// rather than to unset something.
	if committed, ok := repo.lastCommit(); !ok || committed.UndoAction != "Clear Resume Note" {
		t.Fatalf("clear undo action = %q, want Clear Resume Note", committed.UndoAction)
	}
}

// TestSetResumeNoteNoOpCommitsNothing is the rule that keeps `u` honest: a
// prompt closed on unchanged text is not something to undo, so it may not
// advance the touch time or mint a ledger entry.
func TestSetResumeNoteNoOpCommitsNothing(t *testing.T) {
	svc, repo, task, stale := seedResumeNoteTask(t)
	task.ResumeNote = "stopped at the CSV column mapping"

	before := len(repo.commits)
	result, err := svc.SetResumeNote(repo.loc("t1"), "stopped at the CSV column mapping")
	if err != nil {
		t.Fatalf("no-op: %v", err)
	}
	if result.Changed {
		t.Fatal("an unchanged note reported a change")
	}
	if len(repo.commits) != before {
		t.Fatalf("commits = %d, want the %d it started with", len(repo.commits), before)
	}
	if !task.UpdatedAt.Equal(stale) {
		t.Fatalf("UpdatedAt = %s, want the untouched %s", task.UpdatedAt, stale)
	}

	// Clearing a note the task never had is the same no-op, not an empty write.
	task.ResumeNote = ""
	if cleared, err := svc.SetResumeNote(repo.loc("t1"), ""); err != nil || cleared.Changed {
		t.Fatalf("clearing an absent note = changed %v err %v", cleared.Changed, err)
	}
	if len(repo.commits) != before {
		t.Fatalf("clearing an absent note committed: %d", len(repo.commits))
	}
}

// TestSetResumeNoteTouchesOnlyTheTouchTime holds the field boundary. A resume
// note is orientation beside the work, so it may move UpdatedAt and nothing
// else — the state clock and the planning membership in particular.
func TestSetResumeNoteTouchesOnlyTheTouchTime(t *testing.T) {
	svc, repo, task, stale := seedResumeNoteTask(t)
	stateClock := task.StateChangedAt
	state := task.State()

	if _, err := svc.SetResumeNote(repo.loc("t1"), "resumed here"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !task.UpdatedAt.After(stale) {
		t.Fatalf("UpdatedAt = %s, want it advanced past %s", task.UpdatedAt, stale)
	}
	if !task.StateChangedAt.Equal(stateClock) {
		t.Fatalf("StateChangedAt = %s, want the untouched %s", task.StateChangedAt, stateClock)
	}
	if task.State() != state {
		t.Fatalf("state = %s, want the untouched %s", task.State(), state)
	}
}

// TestSetResumeNoteAcceptsEveryLifecycleState is the coverage claim: the note
// is content, not a step, so a done or canceled task takes one exactly as a
// doing one does.
func TestSetResumeNoteAcceptsEveryLifecycleState(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		t.Run(string(state), func(t *testing.T) {
			task := newTask("t1", 1, "Wire the importer")
			task.ApplyWorkflowState(state)
			repo := newFakeRepo(newProject("proj-a", "Alpha", task))
			svc := newTestService(repo)

			if _, err := svc.SetResumeNote(repo.loc("t1"), "picked up here"); err != nil {
				t.Fatalf("%s refused a resume note: %v", state, err)
			}
			if task.ResumeNote != "picked up here" {
				t.Fatalf("%s note = %q", state, task.ResumeNote)
			}
		})
	}
}

// TestSetResumeNoteRefusesWhatTheWriterWouldRefuse keeps one gate. The service
// validates rather than repairs, so a note the canonical document could not
// hold never reaches a commit.
func TestSetResumeNoteRefusesWhatTheWriterWouldRefuse(t *testing.T) {
	svc, repo, task, _ := seedResumeNoteTask(t)
	before := len(repo.commits)

	for name, note := range map[string]string{
		"multiline": "first line\nsecond line",
		"untrimmed": "  leading space",
		"too long":  strings.Repeat("x", models.MaxResumeNoteLength+1),
		"carriage":  "first\rsecond",
		"trailing":  "trailing space ",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.SetResumeNote(repo.loc("t1"), note); !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("%s = %v, want an invalid-input refusal", name, err)
			}
		})
	}
	if task.ResumeNote != "" || len(repo.commits) != before {
		t.Fatalf("a refused note reached the task (%q) or the ledger (%d commits)", task.ResumeNote, len(repo.commits))
	}
}

// TestSetResumeNoteAcceptsTheExactBound pins the boundary the prompt promises:
// a note of exactly the maximum length is storable, so the input cap and the
// validation gate agree about where the edge is.
func TestSetResumeNoteAcceptsTheExactBound(t *testing.T) {
	svc, repo, task, _ := seedResumeNoteTask(t)
	note := strings.Repeat("é", models.MaxResumeNoteLength)

	if _, err := svc.SetResumeNote(repo.loc("t1"), note); err != nil {
		t.Fatalf("a note of exactly %d characters was refused: %v", models.MaxResumeNoteLength, err)
	}
	if task.ResumeNote != note {
		t.Fatal("the boundary note was not stored verbatim")
	}
}

// TestSetResumeNoteRefusesAnUnresolvableTarget keeps the failure before the
// commit rather than after it, so a stale locator writes nothing anywhere.
func TestSetResumeNoteRefusesAnUnresolvableTarget(t *testing.T) {
	svc, repo, _, _ := seedResumeNoteTask(t)
	before := len(repo.commits)

	if _, err := svc.SetResumeNote(app.TaskLocator{ProjectID: "proj-a", TaskID: "gone"}, "note"); err == nil {
		t.Fatal("a missing task accepted a resume note")
	}
	if len(repo.commits) != before {
		t.Fatalf("a missing task committed: %d", len(repo.commits))
	}
}
