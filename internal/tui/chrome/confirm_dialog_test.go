package chrome

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestConfirmDialog_Lifecycle(t *testing.T) {
	var c ConfirmDialog

	// Initially inactive
	if c.IsActive() {
		t.Error("expected dialog to be inactive initially")
	}

	// Show dialog
	c.Show("delete", "Delete item?")
	if !c.IsActive() {
		t.Error("expected dialog to be active after Show")
	}
	if c.Action() != "delete" {
		t.Errorf("expected action 'delete', got '%s'", c.Action())
	}
	if c.Prompt() != "Delete item?" {
		t.Errorf("expected prompt 'Delete item?', got '%s'", c.Prompt())
	}

	// Cancel dialog
	c.Cancel()
	if c.IsActive() {
		t.Error("expected dialog to be inactive after Cancel")
	}
}

func TestConfirmDialog_HandleKey_Confirm(t *testing.T) {
	var c ConfirmDialog
	c.Show("delete", "Delete?")

	confirmed, handled := c.HandleKey("y")
	if !confirmed {
		t.Error("expected confirmed=true for 'y'")
	}
	if !handled {
		t.Error("expected handled=true for 'y'")
	}
	if c.IsActive() {
		t.Error("expected dialog to be inactive after confirm")
	}
}

func TestConfirmDialog_HandleKey_Cancel(t *testing.T) {
	tests := []string{"n", "N", "esc"}
	for _, key := range tests {
		var c ConfirmDialog
		c.Show("delete", "Delete?")

		confirmed, handled := c.HandleKey(key)
		if confirmed {
			t.Errorf("expected confirmed=false for '%s'", key)
		}
		if !handled {
			t.Errorf("expected handled=true for '%s'", key)
		}
		if c.IsActive() {
			t.Errorf("expected dialog to be inactive after '%s'", key)
		}
	}
}

func TestConfirmDialog_HandleKey_OtherKeys(t *testing.T) {
	var c ConfirmDialog
	c.Show("delete", "Delete?")

	// Other keys should be blocked but not confirm/cancel
	confirmed, handled := c.HandleKey("x")
	if confirmed {
		t.Error("expected confirmed=false for 'x'")
	}
	if !handled {
		t.Error("expected handled=true for 'x' (blocked)")
	}
	if !c.IsActive() {
		t.Error("expected dialog to remain active for 'x'")
	}
}

func TestConfirmDialog_HandleKey_Inactive(t *testing.T) {
	var c ConfirmDialog
	// Dialog not active

	confirmed, handled := c.HandleKey("y")
	if confirmed {
		t.Error("expected confirmed=false when inactive")
	}
	if handled {
		t.Error("expected handled=false when inactive")
	}
}

func TestConfirmDialog_DefaultPrompt(t *testing.T) {
	var c ConfirmDialog
	c.Show("test", "")

	if c.Prompt() != "Are you sure?" {
		t.Errorf("expected default prompt, got '%s'", c.Prompt())
	}
}

func TestConfirmDialog_Render(t *testing.T) {
	var c ConfirmDialog

	// Render() always returns empty (inline rendering is disabled)
	c.Show("delete", "Delete?")
	if c.Render() != "" {
		t.Error("expected empty render from Render()")
	}

	// RenderOverlay returns empty when inactive
	c.Cancel()
	if c.RenderOverlay(80) != "" {
		t.Error("expected empty overlay when inactive")
	}

	// RenderOverlay returns non-empty when active
	c.Show("delete", "Delete?")
	rendered := c.RenderOverlay(80)
	if rendered == "" {
		t.Error("expected non-empty overlay when active")
	}
}

func TestConfirmDialog_RenderOverlay_NoWrapping(t *testing.T) {
	var c ConfirmDialog
	c.ShowWithDetail("delete", "Delete this task?", &ConfirmDetail{
		Items:    []string{"#1 A task with a longer title (task)"},
		UndoHint: true,
	})

	rendered := c.RenderOverlay(120)
	removedShortcut := "Ctrl" + "+Z"
	if strings.Contains(rendered, removedShortcut) {
		t.Errorf("undo hint should not mention removed shortcut:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Undo available (u)") {
		t.Errorf("undo hint missing u shortcut:\n%s", rendered)
	}

	// Count lines: halo, border, title, prompt, blank, item, undo hint, help,
	// border, halo = 10.
	lines := strings.Split(rendered, "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines (no wrapping), got %d:\n%s", len(lines), rendered)
	}
}

// TestConfirmDialog_DiscardAnswersLikeItsFooter holds the unsaved-draft box to
// the line the footer under it draws: its "no" keeps editing rather than
// cancelling, so the two must not offer the same keys under different words.
func TestConfirmDialog_DiscardAnswersLikeItsFooter(t *testing.T) {
	var state DiscardConfirmState
	state.ConfirmDiscard = true
	rendered := ansi.Strip(state.GetConfirmDialog().RenderOverlay(120))
	if !strings.Contains(rendered, "Y discard") || !strings.Contains(rendered, "N/Esc keep editing") {
		t.Fatalf("discard box does not answer like its footer:\n%s", rendered)
	}
	if strings.Contains(rendered, "n/Esc cancel") {
		t.Fatalf("discard box still offers the generic cancel:\n%s", rendered)
	}

	var destructive ConfirmDialog
	destructive.Show("delete", "Delete this task?")
	if rendered := ansi.Strip(destructive.RenderOverlay(120)); !strings.Contains(rendered, "y confirm") ||
		!strings.Contains(rendered, "n/Esc cancel") {
		t.Fatalf("a destructive confirmation lost its generic answers:\n%s", rendered)
	}
}
