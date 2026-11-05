package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
)

func TestInlinePeekMeterReadsTheFraction(t *testing.T) {
	for _, tc := range []struct {
		done, total int
		want        string
	}{
		{0, 5, "▱▱▱▱▱▱▱▱"},
		{5, 5, "▰▰▰▰▰▰▰▰"},
		{1, 100, "▰▱▱▱▱▱▱▱"},  // any progress shows
		{99, 100, "▰▰▰▰▰▰▰▱"}, // unfinished never reads full
		{1, 2, "▰▰▰▰▱▱▱▱"},
	} {
		if got := ansi.Strip(inlinePeekMeter(tc.done, tc.total)); got != tc.want {
			t.Errorf("meter(%d/%d) = %q, want %q", tc.done, tc.total, got, tc.want)
		}
	}
	if inlinePeekMeter(0, 0) != "" {
		t.Error("an empty set drew a meter")
	}
}

func peekTaskInState(id string, state models.WorkflowState) *models.Task {
	task := pagedPeekTask()
	task.ID, task.Title = id, id
	task.WorkflowState = state
	return task
}

func TestSubtaskProgressCountsStoredChildrenButNotCanceled(t *testing.T) {
	parent := peekTaskInState("parent", models.StateBacklog)
	parent.Children = []*models.Task{
		peekTaskInState("a", models.StateDone),
		peekTaskInState("b", models.StateDoing),
		peekTaskInState("c", models.StateCanceled),
	}
	progress := subtaskProgressOf(parent)
	if progress.done != 1 || progress.total != 2 {
		t.Fatalf("progress = %+v, want 1/2", progress)
	}

	doc := inlinePeekDocumentFor(inlinePeekOptions{
		Task: parent, Indent: "  ", Width: 80, Window: testPeekWindow(), Subtasks: progress,
	})
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	if !strings.Contains(body, "Subtasks: 1/2 done ▰▰▰▰▱▱▱▱") {
		t.Fatalf("Peek did not spell the subtask progress:\n%s", body)
	}

	child := parent.Children[1]
	doc = inlinePeekDocumentFor(inlinePeekOptions{
		Task: child, Indent: "  ", Width: 80, Window: testPeekWindow(),
		Breadcrumb: "parent ›", ParentProgress: progress,
	})
	body = ansi.Strip(strings.Join(doc.Body, "\n"))
	if !strings.Contains(body, "Parent: parent · 1/2 done") {
		t.Fatalf("Peek did not follow the parent with its progress:\n%s", body)
	}
}

func TestPeekHistoryDropsAnUpdateThatReadsLikeTheCreation(t *testing.T) {
	now := time.Now()
	task := pagedPeekTask()
	task.CreatedAt = now.Add(-8*24*time.Hour - time.Hour)
	task.UpdatedAt = now.Add(-8 * 24 * time.Hour)
	if line := ansi.Strip(inlinePeekHistoryLine(task, now)); strings.Contains(line, "Updated") {
		t.Fatalf("history repeated an identical age: %q", line)
	}
	task.UpdatedAt = now.Add(-2 * 24 * time.Hour)
	if line := ansi.Strip(inlinePeekHistoryLine(task, now)); !strings.Contains(line, "Updated: 2d ago") {
		t.Fatalf("history dropped a distinct update: %q", line)
	}
}
