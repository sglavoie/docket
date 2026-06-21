package tui

import (
	"testing"
)

// TestOutlinerCommands_AllAdvertiseBothOutlinerViews aggregates the
// session-5 cross-project palette parity contract: every command emitted by
// AppModel.outlinerCommands() must declare both ViewOutliner and
// ViewCrossProjectOutliner in its Views slice. The single-mode-only spot
// checks (status keys) live in app_commands_outliner_cross_project_test.go;
// this test catches regressions where a new outliner command forgets the
// ViewCrossProjectOutliner scope and silently drops the palette out of
// parity for the cross-project outliner.
func TestOutlinerCommands_AllAdvertiseBothOutlinerViews(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmds := app.outlinerCommands()
	if len(cmds) == 0 {
		t.Fatal("expected outlinerCommands to return at least one command")
	}
	for _, cmd := range cmds {
		if !containsView(cmd.Views, ViewOutliner) {
			t.Errorf("command %q: missing ViewOutliner in Views=%v", cmd.Name, cmd.Views)
		}
		if !containsView(cmd.Views, ViewCrossProjectOutliner) {
			t.Errorf("command %q: missing ViewCrossProjectOutliner in Views=%v", cmd.Name, cmd.Views)
		}
	}
}

// TestOutlinerCommands_SortShortcutMetadata covers the full sort group's
// shortcut metadata. The direct keymap binds 1-5/0 without a "s" prefix
// (outliner_keys_views.go), so the palette's Shortcut field must mirror that
// to keep the help registry and palette display in lockstep with what users
// actually press. Session 5 fixed priority and reset; the other four sort
// variants must stay consistent for the same reason.
func TestOutlinerCommands_SortShortcutMetadata(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cases := []struct {
		name     string
		shortcut string
	}{
		{"outliner sort by priority", "1"},
		{"outliner sort by status", "2"},
		{"outliner sort by due date", "3"},
		{"outliner sort by created", "4"},
		{"outliner sort by score", "5"},
		{"outliner reset sort", "0"},
	}
	cmds := app.outlinerCommands()
	for _, tc := range cases {
		cmd := findCommand(cmds, tc.name)
		if cmd == nil {
			t.Errorf("missing command %q", tc.name)
			continue
		}
		if cmd.Shortcut != tc.shortcut {
			t.Errorf("command %q: Shortcut = %q, want %q", tc.name, cmd.Shortcut, tc.shortcut)
		}
	}
}
