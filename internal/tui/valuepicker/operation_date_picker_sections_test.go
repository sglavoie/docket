package valuepicker

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/util"
)

func newDatePickerForTest(t *testing.T, deferReason string, selected ...listsurface.ListItemID) *OperationValuePickerModel {
	t.Helper()
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetDueDate},
		operation.ValueProviderDate,
		"Set Due or Defer",
		"Task",
		OperationDateListSource(deferReason),
		false,
		selected,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	return picker
}

func newDueOnlyPickerForTest(t *testing.T) *OperationValuePickerModel {
	t.Helper()
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetDueDate},
		operation.ValueProviderDueDate,
		"Set Due Date",
		"Task",
		OperationDueDateListSource(),
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	return picker
}

func currentDateRowID(t *testing.T, picker *OperationValuePickerModel) listsurface.ListItemID {
	t.Helper()
	item, ok := picker.Surface().CurrentItem()
	if !ok {
		t.Fatal("the picker has no row under its cursor")
	}
	return item.ID
}

func pressKey(picker *OperationValuePickerModel, key string) (bool, tea.Msg) {
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	switch key {
	case "tab":
	case "shift+tab":
		msg.Mod = tea.ModShift
	default:
		msg = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	_, dismissed, result := picker.HandleKey(msg)
	return dismissed, result
}

// The combined picker always opens in Due, so only the due field may decide
// which row the cursor starts on. A deferral the task happens to hold is not an
// answer to "which due row is this?".
func TestOperationDateSelectedIDReadsTheDueFieldAlone(t *testing.T) {
	today := util.StartOfDay(time.Now())
	arbitrary := today.AddDate(0, 0, 3)
	dueClear := DateListItemID(false, "clear")

	cases := []struct {
		name string
		due  *time.Time
		want listsurface.ListItemID
	}{
		{name: "no due date", due: nil, want: dueClear},
		{name: "a due preset", due: &today, want: DateListItemID(false, "today")},
		{name: "a due date no preset reproduces", due: &arbitrary, want: dueClear},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := OperationDateSelectedID(testCase.due)
			if len(got) != 1 || got[0] != testCase.want {
				t.Fatalf("OperationDateSelectedID = %v, want [%s]", got, testCase.want)
			}
		})
	}
}

// Opening the picker is not an edit, so the highlight it starts on must be a
// due row whatever the task's deferral says.
func TestCombinedDatePickerOpensOnADueRow(t *testing.T) {
	for _, selected := range [][]listsurface.ListItemID{
		nil,
		OperationDateSelectedID(nil),
		{DateListItemID(false, "today")},
	} {
		picker := newDatePickerForTest(t, "", selected...)
		deferred, ok := picker.dateFieldUnderCursor()
		if !ok || deferred {
			t.Fatalf("selected %v opened on deferred = %v (resolved %v)", selected, deferred, ok)
		}
	}
}

func TestTabCrossesBetweenTheTwoDateSections(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	if got, want := currentDateRowID(t, picker), DateListItemID(false, "clear"); got != want {
		t.Fatalf("opening row = %q, want %q", got, want)
	}

	dismissed, result := pressKey(picker, "tab")
	if dismissed || result != nil {
		t.Fatalf("tab = dismissed %v, message %#v, want the picker to stay open silently", dismissed, result)
	}
	if got, want := currentDateRowID(t, picker), DateListItemID(true, "clear"); got != want {
		t.Fatalf("row after tab = %q, want the first defer row %q", got, want)
	}

	if _, result := pressKey(picker, "shift+tab"); result != nil {
		t.Fatalf("shift+tab message = %#v, want none", result)
	}
	if got, want := currentDateRowID(t, picker), DateListItemID(false, "clear"); got != want {
		t.Fatalf("row after shift+tab = %q, want the first due row %q", got, want)
	}
}

// Tab crosses sections; it never answers the picker. Nothing is committed and
// no selection is left behind.
func TestTabSelectsNoDateValue(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	pressKey(picker, "tab")
	if ids := picker.Surface().SelectedIDs(); len(ids) != 0 {
		t.Fatalf("tab left %v selected, want nothing", ids)
	}
	if got := picker.Surface().Query().Raw; got != "" {
		t.Fatalf("tab wrote query %q, want none", got)
	}
}

// A search narrows both sections at once, so the switch must land on a row that
// is actually on screen rather than the section's first row.
func TestTabLandsOnTheFirstMatchingRowOfTheOtherSection(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("month")
	if got, want := currentDateRowID(t, picker), DateListItemID(false, "month"); got != want {
		t.Fatalf("filtered opening row = %q, want %q", got, want)
	}
	if _, result := pressKey(picker, "tab"); result != nil {
		t.Fatalf("tab message = %#v, want none", result)
	}
	if got, want := currentDateRowID(t, picker), DateListItemID(true, "month"); got != want {
		t.Fatalf("row after tab = %q, want %q", got, want)
	}
}

// The refusal belongs to activation, not to navigation: a terminal task's defer
// rows stay reachable so their stated reason can be read, and `a` keeps
// refusing them there.
func TestTabReachesDisabledDeferRowsThatActivationStillRefuses(t *testing.T) {
	const reason = "cannot defer a done task"
	picker := newDatePickerForTest(t, reason)
	if _, result := pressKey(picker, "tab"); result != nil {
		t.Fatalf("tab onto the disabled section = %#v, want no refusal", result)
	}
	item, ok := picker.Surface().CurrentItem()
	if !ok || item.Enabled() {
		t.Fatalf("row after tab = %+v, want a disabled defer row", item)
	}
	if deferred, ok := picker.dateFieldUnderCursor(); !ok || !deferred {
		t.Fatalf("disabled row resolved deferred = %v (%v), want the defer field", deferred, ok)
	}

	_, result := pressKey(picker, "a")
	message, isStatus := result.(status.StatusMsg)
	if !isStatus || !strings.Contains(string(message), reason) {
		t.Fatalf("a on the disabled row = %#v, want a refusal naming %q", result, reason)
	}
	if _, err := picker.SelectedValue(); err == nil {
		t.Fatal("Enter accepted a disabled defer row")
	}
}

// A switch that finds nowhere to go changes nothing and says which section it
// could not reach.
func TestTabExplainsAnUnreachableSectionWithoutMovingOrClearingTheQuery(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("today")
	before := currentDateRowID(t, picker)

	_, result := pressKey(picker, "tab")
	message, ok := result.(status.StatusMsg)
	if !ok || string(message) != "No matching Defer values" {
		t.Fatalf("tab with no defer match = %#v, want a Defer refusal", result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("refused tab moved the cursor to %q, want it left on %q", got, before)
	}
	if got := picker.Surface().Query().Raw; got != "today" {
		t.Fatalf("refused tab wrote query %q, want it preserved", got)
	}
}

func TestTabReportsThatNoDateValuesMatchAtAll(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("zzzz")
	if len(picker.Surface().Matches()) != 0 {
		t.Fatalf("query left %d matches, want none", len(picker.Surface().Matches()))
	}
	_, result := pressKey(picker, "tab")
	message, ok := result.(status.StatusMsg)
	if !ok || string(message) != "No date values match" {
		t.Fatalf("tab with no matches = %#v, want the no-values refusal", result)
	}
}

// Search editing owns every key it is given, so Tab types rather than switches
// while a query is being written.
func TestTabTypesRatherThanSwitchesWhileSearchingOrTypingADate(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	pressKey(picker, "/")
	if !picker.Searching() {
		t.Fatal("/ did not start search editing")
	}
	before := currentDateRowID(t, picker)
	if _, result := pressKey(picker, "tab"); result != nil {
		t.Fatalf("tab while searching = %#v, want the search to swallow it", result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("tab while searching moved the cursor to %q, want %q", got, before)
	}

	picker = newDatePickerForTest(t, "")
	pressKey(picker, "a")
	before = currentDateRowID(t, picker)
	if _, result := pressKey(picker, "tab"); result != nil {
		t.Fatalf("tab while typing a date = %#v, want the input to swallow it", result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("tab while typing a date moved the cursor to %q, want %q", got, before)
	}
}

// One field has no other section to reach, so the due-only projection gains no
// switch — and neither does a picker that is not a date picker at all.
func TestTabIsInertOnPickersWithOneSection(t *testing.T) {
	picker := newDueOnlyPickerForTest(t)
	before := currentDateRowID(t, picker)
	dismissed, result := pressKey(picker, "tab")
	if dismissed || result != nil {
		t.Fatalf("tab on the due-only picker = dismissed %v, message %#v", dismissed, result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("tab on the due-only picker moved the cursor to %q, want %q", got, before)
	}
}

// The one `a` binding writes whichever field the cursor names, so the guidance
// has to say which field that is now, and the key that changes it has to be
// advertised beside it.
func TestCombinedDatePickerGuidanceNamesTheActiveFieldAndTheSwitch(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	rendered := ansi.Strip(picker.Render(100))
	for _, want := range []string{"Tab switch field", "a set due date"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("due-section guidance is missing %q:\n%s", want, rendered)
		}
	}

	pressKey(picker, "tab")
	rendered = ansi.Strip(picker.Render(100))
	if !strings.Contains(rendered, "a set defer date") {
		t.Fatalf("defer-section guidance does not name the defer field:\n%s", rendered)
	}
	if strings.Contains(rendered, "a set due date") {
		t.Fatalf("defer-section guidance still names the due field:\n%s", rendered)
	}

	// Enter and Esc are required hints: an optional switch hint may never cost
	// either of them their slot.
	for _, want := range []string{"Enter choose", "Esc back"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("guidance dropped the required hint %q:\n%s", want, rendered)
		}
	}
}

// The due-only picker has one field, so its hint keeps the plain noun and never
// advertises a switch.
func TestDueOnlyPickerGuidanceKeepsThePlainDateNoun(t *testing.T) {
	rendered := ansi.Strip(newDueOnlyPickerForTest(t).Render(100))
	if !strings.Contains(rendered, "a set date") {
		t.Fatalf("due-only guidance does not advertise typed entry:\n%s", rendered)
	}
	if strings.Contains(rendered, "switch field") {
		t.Fatalf("due-only guidance advertises a section switch:\n%s", rendered)
	}
}
