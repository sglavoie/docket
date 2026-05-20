package external

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// withStdin replaces os.Stdin with the contents of s for the duration of fn,
// then restores the original stdin. Returns any error encountered while
// configuring the pipe.
func withStdin(t *testing.T, s string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = orig
		_ = r.Close()
	}()
	if _, err := io.WriteString(w, s); err != nil {
		_ = w.Close()
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	fn()
}

func TestFallbackSelectTask_EOFReturnsCanceled(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Title: "first"},
		{ShortID: 2, Title: "second"},
	}
	withStdin(t, "", func() {
		_, err := FallbackSelectTask(tasks, "pick:")
		if !errors.Is(err, tterrors.ErrCanceled) {
			t.Errorf("expected ErrCanceled on EOF stdin, got %v", err)
		}
	})
}

func TestFallbackSelectTask_BlankLineReturnsCanceled(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Title: "first"},
	}
	withStdin(t, "\n", func() {
		_, err := FallbackSelectTask(tasks, "pick:")
		if !errors.Is(err, tterrors.ErrCanceled) {
			t.Errorf("expected ErrCanceled on blank input, got %v", err)
		}
	})
}

func TestFallbackSelectTask_ValidInput(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 10, Title: "alpha"},
		{ShortID: 20, Title: "beta"},
	}
	withStdin(t, "2\n", func() {
		got, err := FallbackSelectTask(tasks, "pick:")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ShortID != 20 {
			t.Errorf("expected ShortID 20, got %d", got.ShortID)
		}
	})
}

func TestFallbackSelectTask_OutOfRange(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Title: "first"},
	}
	withStdin(t, "99\n", func() {
		_, err := FallbackSelectTask(tasks, "pick:")
		if err == nil {
			t.Fatal("expected error for out-of-range selection")
		}
		if errors.Is(err, tterrors.ErrCanceled) {
			t.Errorf("out-of-range should not be ErrCanceled, got %v", err)
		}
	})
}

func TestFallbackMultiSelectTasks_EOFReturnsCanceled(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Title: "first"},
	}
	withStdin(t, "", func() {
		_, err := FallbackMultiSelectTasks(tasks, "pick:")
		if !errors.Is(err, tterrors.ErrCanceled) {
			t.Errorf("expected ErrCanceled on EOF stdin, got %v", err)
		}
	})
}

func TestFallbackMultiSelectTasks_BlankReturnsCanceled(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Title: "first"},
		{ShortID: 2, Title: "second"},
	}
	withStdin(t, "\n", func() {
		_, err := FallbackMultiSelectTasks(tasks, "pick:")
		if !errors.Is(err, tterrors.ErrCanceled) {
			t.Errorf("expected ErrCanceled on blank input, got %v", err)
		}
	})
}

func TestFallbackMultiSelectTasks_CommaSeparated(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Title: "first"},
		{ShortID: 2, Title: "second"},
		{ShortID: 3, Title: "third"},
	}
	withStdin(t, "1,3\n", func() {
		got, err := FallbackMultiSelectTasks(tasks, "pick:")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 selected, got %d", len(got))
		}
		if got[0].ShortID != 1 || got[1].ShortID != 3 {
			t.Errorf("expected ShortIDs [1,3], got [%d,%d]", got[0].ShortID, got[1].ShortID)
		}
	})
}
