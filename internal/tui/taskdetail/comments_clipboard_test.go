package taskdetail

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// withFakeClipboard swaps external.CopyText for the duration of the test,
// capturing the copied text (or forcing an error) so clipboard-dependent code
// paths are exercised without touching the real system clipboard. It mirrors
// the shell's own copy in internal/tui/clipboard_test_helpers_test.go, which
// the driver-level clipboard tests still need.
func withFakeClipboard(t *testing.T, err error, captured *string) {
	t.Helper()
	orig := external.CopyText
	t.Cleanup(func() { external.CopyText = orig })
	external.CopyText = func(text string) error {
		if captured != nil {
			*captured = text
		}
		return err
	}
}

func TestFormatNumberedMarkdownList(t *testing.T) {
	got := formatNumberedMarkdownList([]string{
		"first",
		"second line one\nsecond line two",
		"third",
	})
	want := strings.Join([]string{
		"1. first",
		"2. second line one",
		"   second line two",
		"3. third",
	}, "\n")
	if got != want {
		t.Fatalf("numbered markdown mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestCommentsCopyAllIncludesEveryRecord(t *testing.T) {
	project, err := models.NewProject("cd-project", "CD", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewGenericTask("cd-task", 1, "CD task")
	if err != nil {
		t.Fatal(err)
	}
	task.AddComment("first comment")
	task.AddComment("second comment")
	project.Tasks = []*models.Task{task}

	t.Run("comments", func(t *testing.T) {
		var captured string
		withFakeClipboard(t, nil, &captured)
		cm := newTestCommentsPanel(task, project)
		cm.CopyAllCommentsText()
		want := strings.Join([]string{"1. first comment", "2. second comment"}, "\n")
		if captured != want {
			t.Fatalf("comments copy-all mismatch:\n got: %q\nwant: %q", captured, want)
		}
	})
}

func TestCommentsCopyAllEmptyNothingToCopy(t *testing.T) {
	project, _ := models.NewProject("empty-project", "Empty", t.TempDir())
	task, _ := models.NewGenericTask("empty-task", 1, "Empty task")
	project.Tasks = []*models.Task{task}

	called := false
	orig := external.CopyText
	t.Cleanup(func() { external.CopyText = orig })
	external.CopyText = func(string) error { called = true; return nil }

	cm := newTestCommentsPanel(task, project)
	cm.CopyAllCommentsText()
	if called {
		t.Fatal("empty comment list must not invoke the clipboard")
	}
	if cm.StatusMessage != "Nothing to copy" {
		t.Fatalf("expected 'Nothing to copy', got %q", cm.StatusMessage)
	}
}
