package taskform

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
	formBasicsFieldCount   = 8 // Title, Description, Priority, Parent, State, Flags, Tags, Context
	formScheduleFieldCount = 3 // Due, Next check-in, Repeat
)

// formBasicsContextField is the expanded Basics-tab focus index of the Context
// row. It is named because focus routing, rendering, help, and save-time error
// placement all have to agree on it.
const formBasicsContextField = 7

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
