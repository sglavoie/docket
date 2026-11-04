package tui

import (
	"strings"
	"testing"
)

func ghostDriver(t *testing.T, motion bool) *UITestDriver {
	t.Helper()
	driver := launchReadmeFixture(t)
	if motion {
		WithMotion()(driver.App())
	}
	for _, key := range []string{"j", "j", "j", "j"} {
		if _, err := driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: key}, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return driver
}

func applyGhostSteps(t *testing.T, driver *UITestDriver, actions ...UITestAction) UITestObservation {
	t.Helper()
	var obs UITestObservation
	for _, action := range actions {
		var err error
		if obs, err = driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
		if len(obs.LayoutIssues) != 0 {
			t.Fatalf("layout issues after %+v: %+v\n%s", action, obs.LayoutIssues, obs.PlainScreen)
		}
	}
	return obs
}

var completeCursorTask = []UITestAction{{Type: UITestActionKey, Key: "w"}, {Type: UITestActionKey, Key: "d"}}

// A completion leaves its row on screen, struck and checked, at the line it
// vacated; the cursor is drawn on its successor one line below, and the ghost
// retires on its own after its phases.
func TestDepartureGhostHoldsTheCompletedRowThenCloses(t *testing.T) {
	driver := ghostDriver(t, true)
	obs := applyGhostSteps(t, driver, completeCursorTask...)
	if !strings.Contains(obs.PlainScreen, "✓   Plan the weekend walk") {
		t.Fatalf("completion left no ghost:\n%s", obs.PlainScreen)
	}
	view := driver.App().crossOutliner
	row, ok := view.CursorScreenRow()
	if !ok {
		t.Fatal("cursor not drawn")
	}
	lines := strings.Split(view.RenderOutlinerTreeContent(view.Width()), "\n")
	if row >= len(lines) || !strings.Contains(lines[row], "Collect feedback") {
		t.Fatalf("CursorScreenRow %d names %q, want the successor row", row, lines[min(row, len(lines)-1)])
	}

	obs = applyGhostSteps(t, driver, UITestAction{Type: UITestActionWait, Duration: "400ms"})
	if strings.Contains(obs.PlainScreen, "   Plan the weekend walk") {
		t.Fatalf("ghost outlived its phases:\n%s", obs.PlainScreen)
	}
}

func TestDepartureGhostRetiresOnTheNextKeystroke(t *testing.T) {
	driver := ghostDriver(t, true)
	applyGhostSteps(t, driver, completeCursorTask...)
	obs := applyGhostSteps(t, driver, UITestAction{Type: UITestActionKey, Key: "k"})
	if strings.Contains(obs.PlainScreen, "   Plan the weekend walk") {
		t.Fatalf("a keystroke left the ghost standing:\n%s", obs.PlainScreen)
	}
}

func TestDepartureGhostNeedsMotion(t *testing.T) {
	driver := ghostDriver(t, false)
	obs := applyGhostSteps(t, driver, completeCursorTask...)
	if strings.Contains(obs.PlainScreen, "   Plan the weekend walk") {
		t.Fatalf("a session without motion drew a ghost:\n%s", obs.PlainScreen)
	}
}
