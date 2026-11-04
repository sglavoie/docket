package tui

import (
	"strings"
	"testing"
)

func typeCapture(t *testing.T, driver *UITestDriver, text string) UITestObservation {
	t.Helper()
	obs, err := driver.ApplyAction(UITestAction{Type: UITestActionText, Text: text}, ObserveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return obs
}

// The selected completion's untyped rest is drawn after the caret, and Tab
// inserts exactly what it suggested.
func TestCaptureSuggestsTheRestOfTheSelectedCompletion(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "a")
	obs := typeCapture(t, driver, "fix login %web")
	if !strings.Contains(obs.PlainScreen, "fix login %web site") {
		t.Fatalf("no inline suggestion after the caret:\n%s", obs.PlainScreen)
	}
	if len(obs.LayoutIssues) != 0 {
		t.Fatalf("layout issues: %+v", obs.LayoutIssues)
	}
	obs = pressKeys(t, driver, "tab")
	if !strings.Contains(obs.PlainScreen, "fix login %website") {
		t.Fatalf("Tab did not insert the suggested rest:\n%s", obs.PlainScreen)
	}
}

// A caret inside the draft suggests nothing: the rest would not be inserted
// where it is drawn.
func TestCaptureSuggestsNothingAwayFromTheEnd(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "a")
	typeCapture(t, driver, "fix login %web")
	obs := pressKeys(t, driver, "left")
	if strings.Contains(obs.PlainScreen, "%we b site") || strings.Contains(obs.PlainScreen, "%web site") {
		t.Fatalf("a caret inside the draft drew a suggestion:\n%s", obs.PlainScreen)
	}
}
