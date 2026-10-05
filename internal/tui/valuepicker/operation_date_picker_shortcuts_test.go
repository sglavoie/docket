package valuepicker

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/util"
)

// dateValueFrom unwraps the payload a committed picker emits, asserting on the
// way that the frozen execution request traveled untouched.
func dateValueFrom(t *testing.T, result tea.Msg) operation.OperationDateValue {
	t.Helper()
	selected, ok := result.(operation.OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("picker result = %#v, want an OperationValueSelectedMsg", result)
	}
	if selected.Request.ID != operation.OperationTaskSetDueDate {
		t.Fatalf("shortcut rewrote the request to %q", selected.Request.ID)
	}
	value, ok := selected.Value.(operation.OperationDateValue)
	if !ok {
		t.Fatalf("committed value = %T, want operation.OperationDateValue", selected.Value)
	}
	return value
}

// enterOnRow is the reference answer every shortcut is measured against: move the
// cursor to a row and press Enter, which is the path the picker already had.
func enterOnRow(t *testing.T, id listsurface.ListItemID) operation.OperationDateValue {
	t.Helper()
	picker := newDatePickerForTest(t, "")
	if !picker.Surface().SetCursorToID(id) {
		t.Fatalf("row %q is not in the combined picker", id)
	}
	dismissed, result := pressKey(picker, "enter")
	if !dismissed {
		t.Fatalf("Enter on %q left the picker open", id)
	}
	return dateValueFrom(t, result)
}

// The whole promise of a shortcut is that it is not a second way to compute a
// date: it applies the row it names, and the row is where the value comes from.
func TestEveryDatePresetKeyAppliesTheSameValueAsEnterOnItsRow(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		for _, preset := range OperationDatePresets(deferred, time.Now()) {
			key, ok := chrome.DatePresetKey(deferred, preset.id)
			if !ok {
				t.Fatalf("deferred=%v preset %q binds no key", deferred, preset.id)
			}
			t.Run(string(DateListItemID(deferred, preset.id)), func(t *testing.T) {
				picker := newDatePickerForTest(t, "")
				if deferred {
					if _, result := pressKey(picker, "tab"); result != nil {
						t.Fatalf("tab into the defer field = %#v", result)
					}
				}
				dismissed, result := pressKey(picker, key)
				if !dismissed {
					t.Fatalf("%q left the picker open: %#v", key, result)
				}
				got := dateValueFrom(t, result)
				want := enterOnRow(t, DateListItemID(deferred, preset.id))
				if got.Deferred != want.Deferred {
					t.Fatalf("%q wrote deferred = %v, want %v", key, got.Deferred, want.Deferred)
				}
				switch {
				case want.Date == nil && got.Date != nil:
					t.Fatalf("%q wrote %s, want the clear row's nil", key, got.Date)
				case want.Date != nil && got.Date == nil:
					t.Fatalf("%q cleared the field, want %s", key, want.Date)
				case want.Date != nil && !sameDateWithinTheSecond(*got.Date, *want.Date):
					t.Fatalf("%q wrote %s, want %s", key, got.Date, want.Date)
				}
			})
		}
	}
}

// The hour rows are recomputed between the two presses, so an equality test
// would be a clock race. A second of tolerance is far tighter than the hour
// these rows are spaced by and far looser than two consecutive key presses.
func sameDateWithinTheSecond(got, want time.Time) bool {
	delta := got.Sub(want)
	if delta < 0 {
		delta = -delta
	}
	return delta < time.Second
}

// The accepted table, spelled out as the three worked examples the design named.
func TestAcceptedDatePresetKeysNameTheAcceptedDates(t *testing.T) {
	tomorrow := util.StartOfDay(time.Now()).AddDate(0, 0, 1)

	picker := newDatePickerForTest(t, "")
	dismissed, result := pressKey(picker, "m")
	if !dismissed {
		t.Fatalf("`m` on the due field left the picker open: %#v", result)
	}
	value := dateValueFrom(t, result)
	if value.Deferred {
		t.Fatal("`m` on the due field wrote a deferral")
	}
	if value.Date == nil || !value.Date.Equal(tomorrow) {
		t.Fatalf("`m` wrote %v, want tomorrow at %s", value.Date, tomorrow)
	}

	// The hour rows are frozen when the list is built, and a shortcut applies the
	// row rather than recomputing the offset, so the committed instant is the one
	// the row was already showing.
	picker = newDatePickerForTest(t, "")
	pressKey(picker, "tab")
	var frozen operation.OperationDateValue
	for _, item := range picker.Surface().Items() {
		if item.ID == DateListItemID(true, chrome.DatePresetFourHours) {
			frozen = item.Data.(operation.OperationDateValue)
		}
	}
	if frozen.Date == nil {
		t.Fatal("the defer section has no four-hour row")
	}
	dismissed, result = pressKey(picker, "4")
	if !dismissed {
		t.Fatalf("`4` on the defer field left the picker open: %#v", result)
	}
	value = dateValueFrom(t, result)
	if !value.Deferred {
		t.Fatal("`4` on the defer field wrote a due date")
	}
	if value.Date == nil || !value.Date.Equal(*frozen.Date) {
		t.Fatalf("`4` wrote %v, want the row's own instant %s", value.Date, frozen.Date)
	}
	if !sameDateWithinTheSecond(*value.Date, time.Now().Add(4*time.Hour)) {
		t.Fatalf("`4` wrote %s, want roughly four hours from now", value.Date)
	}

	picker = newDatePickerForTest(t, "")
	pressKey(picker, "tab")
	dismissed, result = pressKey(picker, "9")
	if !dismissed {
		t.Fatalf("`9` on the defer field left the picker open: %#v", result)
	}
	value = dateValueFrom(t, result)
	year, month, day := time.Now().Date()
	wantMorning := time.Date(year, month, day+1, 9, 0, 0, 0, time.Local)
	if value.Date == nil || !value.Date.Equal(wantMorning) {
		t.Fatalf("`9` wrote %v, want tomorrow at local 09:00 (%s)", value.Date, wantMorning)
	}
}

// `0` is bound in both fields and means the same act in each, which is exactly
// why it must never reach across: clearing is the one shortcut that could
// plausibly be read as clearing "the date" rather than this field's date.
func TestClearingAffectsOnlyTheFieldUnderTheCursor(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	_, result := pressKey(picker, "0")
	value := dateValueFrom(t, result)
	if value.Date != nil || value.Deferred {
		t.Fatalf("`0` in the due field = %+v, want a cleared due date", value)
	}

	picker = newDatePickerForTest(t, "")
	pressKey(picker, "tab")
	_, result = pressKey(picker, "0")
	value = dateValueFrom(t, result)
	if value.Date != nil || !value.Deferred {
		t.Fatalf("`0` in the defer field = %+v, want a cleared deferral", value)
	}
}

// A key names a preset, not a row position. A search that drops the rows above
// the target moves it up the list without changing what its key means.
func TestRowPositionDoesNotChangeWhatAPresetKeyMeans(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("one")
	if got := currentDateRowID(t, picker); got != DateListItemID(false, "week") {
		t.Fatalf("filtered opening row = %q, want the due week row", got)
	}
	dismissed, result := pressKey(picker, "n")
	if !dismissed {
		t.Fatalf("`n` under a search left the picker open: %#v", result)
	}
	value := dateValueFrom(t, result)
	want := util.StartOfDay(time.Now()).AddDate(0, 0, 30)
	if value.Deferred || value.Date == nil || !value.Date.Equal(want) {
		t.Fatalf("`n` under a search wrote %+v, want the due month row %s", value, want)
	}
}

// A refused row refuses the shortcut with its own stated reason, exactly as it
// refuses `a` and Enter, and the picker stays open so the reason can be read.
func TestADisabledRowRefusesItsShortcutAndKeepsThePickerOpen(t *testing.T) {
	const reason = "cannot defer a done task"
	picker := newDatePickerForTest(t, reason)
	pressKey(picker, "tab")

	dismissed, result := pressKey(picker, "4")
	if dismissed {
		t.Fatal("`4` on a disabled defer row closed the picker")
	}
	message, ok := result.(status.StatusMsg)
	if !ok || !strings.Contains(string(message), reason) {
		t.Fatalf("`4` on a disabled row = %#v, want a refusal naming %q", result, reason)
	}
}

// A preset the current query hid is unavailable, and saying so is the whole
// response: the query that hid it stands, and no other row is applied in its
// place.
func TestAFilteredOutPresetReportsItselfUnavailableAndAppliesNothing(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("today")
	before := currentDateRowID(t, picker)

	dismissed, result := pressKey(picker, "m")
	if dismissed {
		t.Fatal("a filtered-out preset closed the picker")
	}
	message, ok := result.(status.StatusMsg)
	if !ok || string(message) != "Tomorrow is filtered out" {
		t.Fatalf("`m` with Tomorrow filtered out = %#v, want an unavailable refusal", result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("the refusal moved the cursor to %q, want it left on %q", got, before)
	}
	if got := picker.Surface().Query().Raw; got != "today" {
		t.Fatalf("the refusal wrote query %q, want it preserved", got)
	}
	if ids := picker.Surface().SelectedIDs(); len(ids) != 0 {
		t.Fatalf("the refusal selected %v, want nothing", ids)
	}
}

func TestAPresetKeyWithNoMatchesAtAllSaysSo(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	picker.Surface().SetQuery("zzzz")
	dismissed, result := pressKey(picker, "m")
	if dismissed {
		t.Fatal("a preset key closed a picker with no matches")
	}
	message, ok := result.(status.StatusMsg)
	if !ok || string(message) != "No matching date values" {
		t.Fatalf("`m` with no matches = %#v, want the no-values refusal", result)
	}
}

// The two fields hold separate vocabularies, and the separation is the point: a
// key one field does not bind explains its owner without applying a value.
func TestKeysUnboundInTheActiveFieldExplainTheirOwner(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	before := currentDateRowID(t, picker)
	for _, key := range []string{"1", "4", "8", "9"} {
		dismissed, result := pressKey(picker, key)
		if dismissed || result != status.StatusMsg(key+" is a Defer shortcut") {
			t.Fatalf("`%s` in the due field = dismissed %v, message %#v", key, dismissed, result)
		}
		if got := currentDateRowID(t, picker); got != before {
			t.Fatalf("`%s` in the due field moved the cursor to %q", key, got)
		}
	}

	picker = newDatePickerForTest(t, "")
	pressKey(picker, "tab")
	before = currentDateRowID(t, picker)
	dismissed, result := pressKey(picker, "t")
	if dismissed || result != status.StatusMsg("t is a Due shortcut") {
		t.Fatalf("`t` in the defer field = dismissed %v, message %#v", dismissed, result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("`t` in the defer field moved the cursor to %q", got)
	}
}

// The due-only projection has due shortcuts and no deferral section switch.
func TestTheDueOnlyPickerAnswersTheDueShortcutsAlone(t *testing.T) {
	picker := newDueOnlyPickerForTest(t)
	dismissed, result := pressKey(picker, "t")
	if !dismissed {
		t.Fatalf("`t` on the due-only picker left it open: %#v", result)
	}
	value := dateValueFrom(t, result)
	want := util.StartOfDay(time.Now())
	if value.Deferred || value.Date == nil || !value.Date.Equal(want) {
		t.Fatalf("`t` on the due-only picker wrote %+v, want today %s", value, want)
	}

	picker = newDueOnlyPickerForTest(t)
	before := currentDateRowID(t, picker)
	if dismissed, result := pressKey(picker, "4"); dismissed || result != status.StatusMsg("4 is a Defer shortcut") {
		t.Fatalf("`4` on the due-only picker = dismissed %v, message %#v", dismissed, result)
	}
	if got := currentDateRowID(t, picker); got != before {
		t.Fatalf("`4` on the due-only picker moved the cursor to %q", got)
	}
}

// Search editing owns every key it is given, so a preset key types rather than
// applies while a query is being written — and the same holds for the typed-date
// input, which is the one place a bare digit is a date and not a shortcut.
func TestPresetKeysTypeRatherThanApplyWhileSearchingOrTypingADate(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	pressKey(picker, "/")
	if dismissed, result := pressKey(picker, "m"); dismissed || result != nil {
		t.Fatalf("`m` while searching = dismissed %v, message %#v", dismissed, result)
	}
	if got := picker.Surface().Query().Raw; got != "m" {
		t.Fatalf("`m` while searching wrote query %q, want %q", got, "m")
	}

	picker = newDatePickerForTest(t, "")
	pressKey(picker, "a")
	if dismissed, result := pressKey(picker, "0"); dismissed || result != nil {
		t.Fatalf("`0` while typing a date = dismissed %v, message %#v", dismissed, result)
	}
	if got := picker.dateAdder.AddDateInput.Value(); got != "0" {
		t.Fatalf("`0` while typing a date left the input at %q, want %q", got, "0")
	}
}

// The shortcuts are a date picker's, not every picker's. A picker that is not a
// date picker keeps whatever those letters already meant to it — here, nothing.
func TestDatePresetKeysStayOutOfOtherPickers(t *testing.T) {
	picker := newFilterTagPickerForTest(t, []query.TagInfo{{Name: "backend", TaskCount: 1}})
	before, _ := picker.Surface().CurrentItem()
	for _, key := range []string{"0", "m", "n", "9"} {
		if _, dismissed, msg := picker.HandleKey(keyPress(key)); dismissed || msg != nil {
			t.Fatalf("the tag picker consumed `%s`: dismissed = %v, msg = %#v", key, dismissed, msg)
		}
	}
	if after, _ := picker.Surface().CurrentItem(); after.ID != before.ID {
		t.Fatalf("a date shortcut moved the tag picker's cursor to %q", after.ID)
	}
}

// `t` and `w` are tag-category jumps on the pickers that have categories to jump
// to. Claiming them for dates had to leave that alone.
func TestTagCategoryJumpKeysSurviveTheDateShortcuts(t *testing.T) {
	picker := newFilterTagPickerForTest(t, []query.TagInfo{{Name: "backend", TaskCount: 1}})
	if _, dismissed, msg := picker.HandleKey(keyPress("t")); dismissed || msg != nil {
		t.Fatalf("`t` on the tag picker = dismissed %v, msg %#v, want a silent jump", dismissed, msg)
	}
	item, ok := picker.Surface().CurrentItem()
	if !ok || item.CategoryID != TagDescriptiveCategory {
		t.Fatalf("`t` landed on %+v, want the descriptive tag section", item)
	}
}

// The hint is chrome, not vocabulary: it says how to apply a row without
// becoming a word that finds one.
func TestDatePresetHintsRideTheMarkerAndStayOutOfTheSearchableText(t *testing.T) {
	for _, item := range OperationDateListSource("").Items {
		value, isDate := item.Data.(operation.OperationDateValue)
		if !isDate {
			continue
		}
		presetID := strings.TrimPrefix(strings.TrimPrefix(string(item.ID), "date:due:"), "date:defer:")
		key, ok := chrome.DatePresetKey(value.Deferred, presetID)
		if !ok {
			t.Fatalf("row %q names no shortcut key", item.ID)
		}
		if got := ansi.Strip(item.PrimaryMarker); got != key+" " {
			t.Errorf("row %q marker = %q, want %q", item.ID, got, key+" ")
		}
		if strings.Contains(item.PrimaryText, key) && item.PrimaryText != "Clear" {
			// Labels may legitimately contain the letter; what they may not do is
			// carry the hint's bracketed or standalone form.
			if strings.Contains(item.PrimaryText, "["+key+"]") {
				t.Errorf("row %q prints its key inside the primary text %q", item.ID, item.PrimaryText)
			}
		}
		for _, alias := range item.Aliases {
			if alias == key {
				t.Errorf("row %q answers a search for its shortcut key %q", item.ID, key)
			}
		}
	}
}

// The footer says the keys are live; the rows say which key is which. Both are
// optional hints that a narrow footer may drop, but neither may displace an
// answer.
func TestTheDatePickerFooterAdvertisesThePresetKeys(t *testing.T) {
	rendered := ansi.Strip(newDatePickerForTest(t, "").Render(100))
	for _, want := range []string{"preset key pick", "Enter choose", "Esc back"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("picker guidance is missing %q:\n%s", want, rendered)
		}
	}
	if !strings.Contains(ansi.Strip(newDueOnlyPickerForTest(t).Render(100)), "preset key pick") {
		t.Fatal("the due-only picker does not advertise its preset keys")
	}
}

// The shortcut table and the row table are two readings of one vocabulary, so
// neither may name a preset the other does not.
func TestEveryDatePresetIsKeyedAndEveryKeyNamesAPreset(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		keyed := make(map[string]string)
		for _, preset := range OperationDatePresets(deferred, time.Now()) {
			key, ok := chrome.DatePresetKey(deferred, preset.id)
			if !ok {
				t.Errorf("deferred=%v preset %q has no key", deferred, preset.id)
				continue
			}
			if owner, clash := keyed[key]; clash {
				t.Errorf("deferred=%v key %q is bound by both %q and %q", deferred, key, owner, preset.id)
			}
			keyed[key] = preset.id
			if got, ok := chrome.DatePresetIDForKey(deferred, key); !ok || got != preset.id {
				t.Errorf("deferred=%v key %q resolves to %q, want %q", deferred, key, got, preset.id)
			}
		}
	}

	// `t` is Today, and a deferral has no Today row. An hour is an instant, and a
	// due date is a calendar day. Neither crosses.
	if _, ok := chrome.DatePresetIDForKey(true, "t"); ok {
		t.Error("`t` is bound in the defer field")
	}
	for _, key := range []string{"1", "4", "8", "9"} {
		if _, ok := chrome.DatePresetIDForKey(false, key); ok {
			t.Errorf("`%s` is bound in the due field", key)
		}
	}
}
