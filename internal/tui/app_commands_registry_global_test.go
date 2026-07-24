package tui

import (
	"testing"
)

// TestCommandRegistry_RegisterAllGroupsSucceeds is the global guard against
// duplicate command registration silently dropping later commands. The
// production code path (registerGlobalCommands) tolerates RegisterBatch errors
// with `_ =`, which means a single overlapping (Name, Views) pair can hide
// every subsequent command in the same batch and every later batch's first
// duplicate. By replaying the exact batch list here against a fresh registry
// and asserting no error, we surface any new duplicate as a test failure
// instead of a runtime silence. Rebuild the registry from m.commandGroups()
// so this assertion automatically picks up new batches without drifting from
// app_init.go.
func TestCommandRegistry_RegisterAllGroupsSucceeds(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for i, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d] failed: %v", i, err)
		}
	}
}

// TestCommandRegistry_CrossProjectOutlinerPaletteParity is the registry-level
// completion contract for Sessions 1-4. It registers every command batch from
// commandGroups() and asserts each planned cross-project outliner palette
// entry is reachable through GetForView(ViewCrossProjectOutliner) with the
// shortcut that matches the direct keymap. View-specific entries beat global
// fallbacks here, so any future change that mis-scopes one of these commands
// (or removes ViewCrossProjectOutliner from its Views slice) lands as a
// concrete failure on the offending name rather than as a silent palette
// regression.
func TestCommandRegistry_CrossProjectOutlinerPaletteParity(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for _, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch failed: %v", err)
		}
	}

	cases := []struct {
		name     string
		shortcut string
	}{
		// Status (Session 1 helper routing, outlinerStatusHandler).
		{"start task", ",s"},
		{"complete task", ",d"},
		{"block task", ",b"},
		{"standby task", ",S"},
		{"cancel task", ",x"},
		{"reopen task", ",o"},
		{"review task", ",r"},
		{"maybe task", ",m"},
		// Sort (direct keymap binds the digit keys with no prefix).
		{"outliner sort by priority", "1"},
		{"outliner sort by status", "2"},
		{"outliner sort by due date", "3"},
		{"outliner sort by created", "4"},
		{"outliner sort by score", "5"},
		{"outliner reset sort", "0"},
		// View / read.
		{"view task details", "o"},
		{"edit task", "e"},
		{"rename task", "tr"},
		{"edit content", "to"},
		// Session 2 task creation parity.
		{"new task (no template)", "tn"},
		{"new task (with template picker)", "tN"},
		{"create linked subtask", "cs"},
		// Copy / mutation.
		{"delete task", "X"},
		{"copy title", "y"},
		{"copy details", "Y"},
		{"copy with description", "ctrl+y"},
		{"score editor", "S"},
		{"multi-select mode", "v"},
		{"task comments", "tc"},
		{"task attachments", "ta"},
		{"navigate linked tasks", "tl"},
		{"manage dependencies", "bm"},
		{"manage decisions", "td"},
		{"dim/undim task", "th"},
		{"move to project", "tm"},
		// Session 3 read/view/capture palette parity.
		{"blockers view", "B"},
		{"inbox processor", "W"},
		{"create note", "}"},
		{"bulk capture", "I"},
		{"edit last task", "ctrl+l"},
		{"task activity", "gh"},
		{"project recent activity", "gl"},
		// Session 4 mutation / project-operation palette parity.
		{"archive completed tasks", ""},
		{"archive project", "ctrl+a"},
		{"paste clipboard", "ctrl+v"},
		{"quick note", "ctrl+."},
	}

	commands := registry.GetForView(ViewCrossProjectOutliner)
	for _, tc := range cases {
		cmd := findCommand(commands, tc.name)
		if cmd == nil {
			t.Errorf("%q should be reachable from ViewCrossProjectOutliner via GetForView", tc.name)
			continue
		}
		if cmd.Shortcut != tc.shortcut {
			t.Errorf("%q shortcut = %q, want %q", tc.name, cmd.Shortcut, tc.shortcut)
		}
	}

	// `quick capture` is registered as a global command (empty Views), so
	// GetForView falls back to the global entry from ViewCrossProjectOutliner.
	// Task Composer is globally available through Ctrl+N; search results do not
	// advertise a second navigation shortcut.
	if cmd := findCommand(commands, "quick capture"); cmd == nil {
		t.Error("'quick capture' should be reachable from ViewCrossProjectOutliner via global fallback")
	} else if cmd.Shortcut != "" {
		t.Errorf("'quick capture' shortcut = %q, want no retired alias", cmd.Shortcut)
	}
}

// TestCanonicalOutlinerProjectCommandsRemainReachable locks FR16-003: the
// retained per-project commands `project metrics` and `reload project` were
// scoped only to the retired single-project ViewOutliner, so they vanished from
// the sole live root (ViewCrossProjectOutliner). They must now be reachable from
// that root, and the operation ledger's reachability classification for each must
// agree with the command actually exposed there.
func TestCanonicalOutlinerProjectCommandsRemainReachable(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for _, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch failed: %v", err)
		}
	}

	commands := registry.GetForView(ViewCrossProjectOutliner)

	cases := []struct {
		name         string
		shortcut     string
		reachability CommandReachability
	}{
		{"project metrics", "", CommandReachPaletteAlias},
		{"reload project", "ctrl+r", CommandReachContextAction},
	}
	for _, tc := range cases {
		cmd := findCommand(commands, tc.name)
		if cmd == nil {
			t.Errorf("%q should be reachable from ViewCrossProjectOutliner via GetForView", tc.name)
			continue
		}
		if cmd.Shortcut != tc.shortcut {
			t.Errorf("%q shortcut = %q, want %q", tc.name, cmd.Shortcut, tc.shortcut)
		}
		if cmd.Reachability != tc.reachability {
			t.Errorf("%q reachability = %q, want %q", tc.name, cmd.Reachability, tc.reachability)
		}
		// The ledger's classification must agree with the command exposed from
		// the sole root: a table entry pins the reachability, and its absence
		// must leave the exposed command at the default contextual-action class.
		if classification, ok := commandClassificationTable[cmd.OperationID]; ok {
			if classification.Reachability != cmd.Reachability {
				t.Errorf("%q ledger table reachability = %q, exposed reachability = %q; they must agree",
					tc.name, classification.Reachability, cmd.Reachability)
			}
		} else if cmd.Reachability != CommandReachContextAction {
			t.Errorf("%q has no ledger entry, so exposed reachability must default to %q, got %q",
				tc.name, CommandReachContextAction, cmd.Reachability)
		}
	}
}
