package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

// A text-field round trip that fails is reported, never silently dropped, and
// a buffer that cannot be read back stays on disk where the result names it.
func TestTextEditorExternalFailureIsReported(t *testing.T) {
	t.Run("editor failure removes the buffer and says so", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "docket-text-1.md")
		if err := os.WriteFile(path, []byte("draft"), 0o600); err != nil {
			t.Fatal(err)
		}
		msg := textEditorExternalDone("Description", path)(errors.New("exit status 1")).(chrome.TextEditorExternalDoneMsg)
		if !msg.Canceled || msg.Err == nil || !strings.Contains(msg.Err.Error(), "editor failed") {
			t.Fatalf("result = %+v, want a canceled result carrying the editor failure", msg)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("buffer after editor failure: %v, want removed", err)
		}
	})

	t.Run("an unreadable buffer is kept and named", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "docket-text-2.md")
		if err := os.Mkdir(path, 0o700); err != nil { // a directory cannot be read as a file
			t.Fatal(err)
		}
		msg := textEditorExternalDone("Description", path)(nil).(chrome.TextEditorExternalDoneMsg)
		if !msg.Canceled || msg.Err == nil || !strings.Contains(msg.Err.Error(), path) {
			t.Fatalf("result = %+v, want a canceled result naming %s", msg, path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unreadable buffer was removed: %v", err)
		}
	})

	t.Run("a successful round trip returns the text and removes the buffer", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "docket-text-3.md")
		if err := os.WriteFile(path, []byte("edited"), 0o600); err != nil {
			t.Fatal(err)
		}
		msg := textEditorExternalDone("Description", path)(nil).(chrome.TextEditorExternalDoneMsg)
		if msg.Canceled || msg.Err != nil || msg.Value != "edited" {
			t.Fatalf("result = %+v, want the edited text", msg)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("buffer after success: %v, want removed", err)
		}
	})

	t.Run("the shell reports the failure instead of clearing the status", func(t *testing.T) {
		model, _ := setupTestModel(t, []*models.Task{setupTestTask(t, 1, "one")})
		model.setInfoStatus("Opening in editor…")
		handled, cmd := model.handleEditorMessages(chrome.TextEditorExternalDoneMsg{
			FieldName: "Description", Canceled: true, Err: errors.New("editor failed: exit status 1"),
		})
		if !handled || cmd == nil {
			t.Fatalf("handled=%v cmd=%v, want an error command", handled, cmd != nil)
		}
	})
}
