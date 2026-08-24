package tui

// Task form tab constants (D3).
const (
	formTabBasics   = 0
	formTabSchedule = 1
	formTabChildren = 2
	formTabExtras   = 3
	formTabCount    = 4
)

var formTabNames = []string{"Basics", "Schedule", "Children", "Extras"}

// Static field counts per tab (dynamic items like checklist are added on top).
const (
	formBasicsFieldCount   = 7 // Title, Description, Priority, Parent, State, Flags, Tags
	formScheduleFieldCount = 1 // Due
)

// formStaticFieldCount returns the number of static focusable fields in a tab.
// The Children and Extras tabs return 0 here; both counts are computed
// dynamically from their collapsible sections.
func formStaticFieldCount(tab int) int {
	switch tab {
	case formTabBasics:
		return formBasicsFieldCount
	case formTabSchedule:
		return formScheduleFieldCount
	case formTabChildren:
		return 0
	case formTabExtras:
		return 0
	}
	return 0
}
