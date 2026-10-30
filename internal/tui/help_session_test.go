package tui

import (
	"testing"
)

func TestHelpSessionReopensFilteredReferenceThroughPublicKeys(t *testing.T) {
	driver, _ := menuBarDriver(t)
	// Esc finishes typing; Enter now runs the selected action.
	pressMenuBarKey(t, driver, "?", "/", "w", "o", "r", "k", "f", "l", "o", "w", "esc")
	before := driver.Observe(ObserveOptions{}).PlainScreen
	if !driver.App().helpView.HasActiveSearch() {
		t.Fatal("Help search did not activate")
	}
	pressMenuBarKey(t, driver, "?", "?")
	if !driver.App().helpView.HasActiveSearch() {
		t.Fatal("closing and reopening Help discarded its query")
	}
	if after := driver.Observe(ObserveOptions{}).PlainScreen; after != before {
		t.Fatalf("reopening Help changed the reference:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	pressMenuBarKey(t, driver, "backspace")
	if driver.App().helpView.HasActiveSearch() {
		t.Fatal("Backspace did not clear the restored search")
	}
}
