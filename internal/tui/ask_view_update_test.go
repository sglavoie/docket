package tui

import (
	"errors"
	"testing"
)

// TestAskStreamDoneCallsCancelFn ensures that when streaming completes, the
// context cancel function is invoked rather than just dropped. Dropping it
// without calling cancel leaks the context's child goroutine until the parent
// is canceled — and the parent here is context.Background(), which never is.
func TestAskStreamDoneCallsCancelFn(t *testing.T) {
	m := NewAskViewModel(nil, "")

	called := false
	m.cancelFn = func() { called = true }

	if _, _ = m.Update(AskStreamDoneMsg{}); !called {
		t.Fatal("expected cancelFn to be called when AskStreamDoneMsg is handled (success path)")
	}

	// Same expectation on the error path.
	called = false
	m.cancelFn = func() { called = true }
	if _, _ = m.Update(AskStreamDoneMsg{Err: errors.New("boom")}); !called {
		t.Fatal("expected cancelFn to be called when AskStreamDoneMsg is handled (error path)")
	}

	if m.cancelFn != nil {
		t.Fatal("expected m.cancelFn to be cleared after handling AskStreamDoneMsg")
	}
}
