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
		"Set Due, Defer, or Check-In",
		"Task",
		OperationDateListSource(deferReason, bothDateFieldsSet),
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
		OperationDueDateListSource(true),
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

// bothDateFieldsSet keeps both Clear rows committable, which is the picker a
// task holding a due date and a deferral opens.
var bothDateFieldsSet = DateFieldsSet{Due: true, Defer: true}

// The combined picker always opens in Due, so only the due field may decide
// which row the cursor starts on. A deferral the task happens to hold is not an
// answer to "which due row is this?". No due date opens on Today, so a reflexive
// Enter writes one instead of clearing nothing; a value a preset cannot name, or
// a selection that disagrees, has something to replace and opens on Clear.
func TestOperationDateSelectedIDReadsTheDueFieldAlone(t *testing.T) {
	today := util.StartOfDay(time.Now())
	arbitrary := today.AddDate(0, 0, 3)
	for util.NextWeekdayAfter(today, time.Saturday).Equal(arbitrary) ||
		util.NextWeekdayAfter(today, time.Monday).Equal(arbitrary) {
		arbitrary = arbitrary.AddDate(0, 0, 1)
	}
	dueClear := DateListItemID(false, "clear")

	cases := []struct {
		name   string
		due    *time.Time
		shared bool
		want   listsurface.ListItemID
	}{
		{name: "no due date", due: nil, shared: true, want: DateListItemID(false, "today")},
		{name: "a due preset", due: &today, shared: true, want: DateListItemID(false, "today")},
		{name: "a due date no preset reproduces", due: &arbitrary, shared: true, want: dueClear},
		{name: "a selection disagreeing about due", due: nil, shared: false, want: dueClear},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := OperationDateSelectedID(testCase.due, testCase.shared)
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
		OperationDateSelectedID(nil, true),
		OperationDateSelectedID(nil, false),
		{DateListItemID(false, "today")},
	} {
		picker := newDatePickerForTest(t, "", selected...)
		section, ok := picker.dateSectionUnderCursor()
		if !ok || section != datePickerSectionDue {
			t.Fatalf("selected %v opened in section %v (resolved %v), want Due", selected, section, ok)
		}
	}
}

func TestTabAndShiftTabCycleTheThreeSchedulingSections(t *testing.T) {
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
	if _, result := pressKey(picker, "tab"); result != nil {
		t.Fatalf("tab to Check-in message = %#v, want none", result)
	}
	if got := currentDateRowID(t, picker); got != dateCheckInEditorItemID {
		t.Fatalf("row after second tab = %q, want %q", got, dateCheckInEditorItemID)
	}
	if _, result := pressKey(picker, "tab"); result != nil {
		t.Fatalf("tab wrapping to Due message = %#v, want none", result)
	}
	if got, want := currentDateRowID(t, picker), DateListItemID(false, "clear"); got != want {
		t.Fatalf("row after wrapped tab = %q, want %q", got, want)
	}

	if _, result := pressKey(picker, "shift+tab"); result != nil {
		t.Fatalf("shift+tab message = %#v, want none", result)
	}
	if got := currentDateRowID(t, picker); got != dateCheckInEditorItemID {
		t.Fatalf("row after shift+tab = %q, want %q", got, dateCheckInEditorItemID)
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
	if section, ok := picker.dateSectionUnderCursor(); !ok || section != datePickerSectionDefer {
		t.Fatalf("disabled row resolved section = %v (%v), want Defer", section, ok)
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

// A single matching section keeps its exact cursor and query, quietly.
func TestTabStaysInTheOnlyMatchingSection(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("in one week")
	before := currentDateRowID(t, picker)

	_, result := pressKey(picker, "tab")
	if result != nil {
		t.Fatalf("tab with only one matching section = %#v, want no message", result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("refused tab moved the cursor to %q, want it left on %q", got, before)
	}
	if got := picker.Surface().Query().Raw; got != "in one week" {
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
	if !ok || string(message) != "No matching date values" {
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

	pressKey(picker, "tab")
	rendered = ansi.Strip(picker.Render(100))
	if strings.Contains(rendered, "a set") || strings.Contains(rendered, "preset key pick") {
		t.Fatalf("Check-in action row advertises single-date controls:\n%s", rendered)
	}

	// Enter and Esc are required hints: an optional switch hint may never cost
	// either of them their slot.
	for _, want := range []string{"Enter choose", "Esc back"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("guidance dropped the required hint %q:\n%s", want, rendered)
		}
	}
}

func TestCheckInRowOpensTheTypedEditorRequestAndClaimsDateKeys(t *testing.T) {
	context := operation.CommandContext{Entity: operation.EntityContext{ProjectID: "project", TaskID: "task"}}
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetDueDate, Context: context},
		operation.ValueProviderDate,
		"Set Due, Defer, or Check-In",
		"Task",
		OperationDateListSource("", bothDateFieldsSet),
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	pressKey(picker, "shift+tab")
	if got := currentDateRowID(t, picker); got != dateCheckInEditorItemID {
		t.Fatalf("shift+tab from Due = %q, want Check-in", got)
	}
	for _, key := range []string{"t", "a"} {
		dismissed, result := pressKey(picker, key)
		var expected tea.Msg
		if key == "t" {
			expected = status.StatusMsg("Press Enter to edit check-in")
		}
		if dismissed || result != expected || currentDateRowID(t, picker) != dateCheckInEditorItemID {
			t.Fatalf("%q on Check-in dismissed=%v result=%#v row=%q", key, dismissed, result, currentDateRowID(t, picker))
		}
	}

	_, dismissed, result := picker.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	request, ok := result.(operation.OperationValueRequestMsg)
	if !dismissed || !ok {
		t.Fatalf("Enter dismissed=%v result=%T, want editor request", dismissed, result)
	}
	if request.Provider != operation.ValueProviderCheckIn || request.Request.ID != operation.OperationTaskSetCheckIn {
		t.Fatalf("editor request = %+v", request)
	}
	if request.Request.Context.Entity.ProjectID != "project" || request.Request.Context.Entity.TaskID != "task" {
		t.Fatalf("editor request lost frozen context: %+v", request.Request.Context.Entity)
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

func TestDateSectionSwitchSkipsFilteredSectionsBothWays(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("month")
	for _, key := range []string{"shift+tab", "tab", "tab", "shift+tab"} {
		before := currentDateRowID(t, picker)
		if dismissed, result := pressKey(picker, key); dismissed || result != nil {
			t.Fatalf("%s dismissed=%v, message=%#v", key, dismissed, result)
		}
		want := DateListItemID(true, "month")
		if before == want {
			want = DateListItemID(false, "month")
		}
		if got := currentDateRowID(t, picker); got != want {
			t.Fatalf("%s landed on %s, want %s", key, got, want)
		}
		if picker.Surface().Query().Raw != "month" || len(picker.Surface().SelectedIDs()) != 0 {
			t.Fatal("switch changed the query or selected a value")
		}
	}
}
