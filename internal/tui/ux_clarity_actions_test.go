package tui

import "testing"

// This file declares no test on purpose. uxClarityPress and uxClarityKeys are
// how every UX clarity test spells a keystroke, and by the end of the plan more
// than four files outside their original home read them —
// TestSharedTestHelpersLiveInFilesThatDeclareNoTest is the gate that noticed,
// and its rule is that a helper with that fan-out must not hide behind a test,
// because Go resolves it by package and nothing in the filename would warn a
// later session that moving that file orphans its readers.

// uxClarityPress replays one production action and fails on the first refusal.
// Every UX clarity test drives the product through the same public action
// vocabulary the committed scenarios use, so a gesture a test can make is a
// gesture a user can make.
func uxClarityPress(t *testing.T, driver *UITestDriver, actions ...UITestAction) {
	t.Helper()
	for index, action := range actions {
		if _, err := driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatalf("action %d (%s %q%q): %v", index, action.Type, action.Key, action.Command, err)
		}
	}
}

// uxClarityKeys is the shorthand for a run of plain key presses, which is what
// most of those gestures are.
func uxClarityKeys(keys ...string) []UITestAction {
	actions := make([]UITestAction, 0, len(keys))
	for _, key := range keys {
		actions = append(actions, UITestAction{Type: UITestActionKey, Key: key})
	}
	return actions
}
