package tui

import (
	"testing"
)

// TestNoGhostBindings walks each registered view and verifies that every
// view-scoped registry entry has a corresponding entry in HandledKeys().
// Globals (Views=[]) are skipped because they are handled by handleGlobalKeyPress,
// not by individual views.
//
// A "ghost binding" is a key that appears in the help/registry but does nothing
// when pressed — a real UX defect. This test caught two such cases:
//   - Settings missing "esc" in HandledKeys() (handler existed, but the
//     bidirectional sync claim was incomplete).
//   - Settings registry key "shift+←/→" was not expanded correctly by
//     expandRegistryKey because the embedded "←" wasn't normalized inside
//     a modifier expression.
func TestNoGhostBindings(t *testing.T) {
	allViews := append(session1SyncViews(), session2SyncViews()...)
	allViews = append(allViews, session3SyncViews()...)
	allViews = append(allViews, session4SyncViews()...)
	allViews = append(allViews, session6SyncViews()...)
	allViews = append(allViews, session8SyncViews()...)
	allViews = append(allViews, session9SyncViews()...)

	reg := GetKeybindingRegistry()
	for _, tc := range allViews {
		t.Run(tc.view.String(), func(t *testing.T) {
			handled := make(map[string]bool)
			for _, k := range tc.handler.HandledKeys() {
				handled[k] = true
			}

			registered := make(map[string]bool)
			for _, b := range reg.bindings {
				if len(b.Views) == 0 {
					continue
				}
				inView := false
				for _, v := range b.Views {
					if v == tc.view {
						inView = true
						break
					}
				}
				if !inView {
					continue
				}
				for _, k := range expandRegistryKey(b.Key) {
					registered[k] = true
				}
			}

			for k := range registered {
				if !handled[k] {
					t.Errorf("ghost binding %q in view %s: registered but not handled", k, tc.view)
				}
			}
		})
	}
}
