package tui

import (
	"slices"
	"testing"
)

func uitestCell(n int) *int { return &n }

func TestUITestPointerActionsValidateTheirTarget(t *testing.T) {
	valid := []UITestAction{
		{Type: UITestActionClick, X: uitestCell(0), Y: uitestCell(0)},
		{Type: UITestActionClick, On: "Inbox", Occurrence: 2, Button: "right", Count: 2},
		{Type: UITestActionWheel, On: "Inbox", Delta: -3},
	}
	for _, action := range valid {
		if err := action.validate(); err != nil {
			t.Errorf("%+v refused: %v", action, err)
		}
	}
	invalid := []UITestAction{
		{Type: UITestActionClick},
		{Type: UITestActionClick, X: uitestCell(1)},
		{Type: UITestActionClick, X: uitestCell(1), Y: uitestCell(1), On: "Inbox"},
		{Type: UITestActionClick, X: uitestCell(-1), Y: uitestCell(1)},
		{Type: UITestActionClick, X: uitestCell(1), Y: uitestCell(1), Occurrence: 1},
		{Type: UITestActionClick, On: "Inbox", Button: "middle"},
		{Type: UITestActionClick, On: "Inbox", Count: 4},
		{Type: UITestActionWheel, On: "Inbox"},
		{Type: UITestActionWheel, On: "Inbox", Delta: 1, Button: "left"},
	}
	for _, action := range invalid {
		if err := action.validate(); err == nil {
			t.Errorf("%+v was accepted", action)
		}
	}
}

func TestUITestScenarioRefusesAnUnknownLaunchFlagAndMouseMode(t *testing.T) {
	sc := UITestScenario{Launch: []string{"--no-such-flag"}, Actions: []UITestAction{{Type: UITestActionWait}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("an unknown launch flag was accepted")
	}
	sc = UITestScenario{Actions: []UITestAction{{Type: UITestActionAssert, Assert: "mouse_mode", Value: "all"}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("an unknown mouse mode was accepted")
	}
}

func TestUITestPointerCellFindsTheNamedOccurrence(t *testing.T) {
	driver := pointerDriver(t)
	first, err := driver.pointerCellFor(t, "rich-project", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := driver.pointerCellFor(t, "rich-project", 2)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("occurrences 1 and 2 resolved to one cell %v", first)
	}
	if _, err := driver.pointerCellFor(t, "rich-project", 99); err == nil {
		t.Fatal("an occurrence past the last match resolved")
	}
	if _, err := driver.pointerCellFor(t, "no such text anywhere", 1); err == nil {
		t.Fatal("an absent literal resolved")
	}
}

func (d *UITestDriver) pointerCellFor(t *testing.T, on string, occurrence int) ([2]int, error) {
	t.Helper()
	x, y, err := d.pointerCell(UITestAction{On: on, Occurrence: occurrence})
	return [2]int{x, y}, err
}

func TestUITestClickCountTwoIsADoubleClickAndAWaitSeparatesClicks(t *testing.T) {
	driver := pointerDriver(t)
	probe := mountPointerProbe(driver)
	run := func(a UITestAction) {
		t.Helper()
		if err := driver.runAction(a); err != nil {
			t.Fatal(err)
		}
	}
	run(UITestAction{Type: UITestActionClick, X: uitestCell(2), Y: uitestCell(3), Count: 2})
	run(UITestAction{Type: UITestActionClick, X: uitestCell(2), Y: uitestCell(3)})
	run(UITestAction{Type: UITestActionWait, Duration: "1s"})
	run(UITestAction{Type: UITestActionClick, X: uitestCell(2), Y: uitestCell(3)})
	run(UITestAction{Type: UITestActionWheel, X: uitestCell(2), Y: uitestCell(3), Delta: -2})

	var clicks, deltas []int
	for _, event := range probe.events {
		if event.IsWheel() {
			deltas = append(deltas, event.WheelDelta)
			continue
		}
		clicks = append(clicks, event.Clicks)
	}
	if want := []int{1, 2, 1, 1}; !slices.Equal(clicks, want) {
		t.Fatalf("clicks counted %v, want %v", clicks, want)
	}
	if want := []int{-1, -1}; !slices.Equal(deltas, want) {
		t.Fatalf("wheel steps %v, want %v", deltas, want)
	}
}
