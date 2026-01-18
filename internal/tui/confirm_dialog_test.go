package tui

import "testing"

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

	if c.Prompt() != "Are you sure? (y/n)" {
		t.Errorf("expected default prompt, got '%s'", c.Prompt())
	}
}

func TestConfirmDialog_Render(t *testing.T) {
	var c ConfirmDialog

	// Inactive - empty render
	if c.Render() != "" {
		t.Error("expected empty render when inactive")
	}

	// Active - should render something
	c.Show("delete", "Delete?")
	rendered := c.Render()
	if rendered == "" {
		t.Error("expected non-empty render when active")
	}
}
