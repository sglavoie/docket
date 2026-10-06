package taskform

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/util"
)

func TestFormDueDatePresetsShowResolvedDatesAtMinimumWidth(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 1, 31, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 14, 0, 0, 0, time.UTC),
	} {
		picker := newFormDueDatePicker(nil)
		rendered := picker.renderAt(chrome.MinTerminalWidth, nil, now)
		plain := ansi.Strip(rendered)
		for _, preset := range formDueDatePresets {
			if preset.isClear {
				continue
			}
			want := preset.label + " [" + preset.key() + "] · " + util.StartOfDay(now).AddDate(0, 0, preset.daysFromNow).Format("Mon Jan 2, 2006")
			if !strings.Contains(plain, want) {
				t.Errorf("missing resolved preset %q:\n%s", want, plain)
			}
		}
		if lipgloss.Width(rendered) > chrome.MinTerminalWidth {
			t.Fatalf("picker overflows minimum width: %d", lipgloss.Width(rendered))
		}
		if strings.Contains(plain, "clear [0] ·") {
			t.Fatal("Clear displayed a date")
		}
	}
}

// The form prints the key it answers to, and both readings come from the shared
// table, so a row can neither advertise a key nothing applies nor answer a key
// nothing advertises.
func TestFormDueDatePresetsAreAllKeyed(t *testing.T) {
	want := map[string]string{
		chrome.DatePresetToday:    "t",
		chrome.DatePresetTomorrow: "m",
		chrome.DatePresetWeek:     "w",
		chrome.DatePresetMonth:    "n",
		chrome.DatePresetClear:    "0",
	}
	if len(formDueDatePresets) != len(want) {
		t.Fatalf("the form lists %d presets, want %d", len(formDueDatePresets), len(want))
	}
	for _, preset := range formDueDatePresets {
		expected, named := want[preset.presetID]
		if !named {
			t.Errorf("the form lists preset %q, which the due table does not name", preset.presetID)
			continue
		}
		if got := preset.key(); got != expected {
			t.Errorf("form preset %q answers %q, want the shared due key %q", preset.presetID, got, expected)
		}
	}
}

// The form's order and its day arithmetic are its own; only the keyboard is
// shared. Taking the keys from the table must not have reordered the list or
// moved a date.
func TestFormDueDatePresetsKeepTheirOrderAndDistances(t *testing.T) {
	want := []struct {
		label string
		days  int
		clear bool
	}{
		{label: "today", days: 0},
		{label: "tomorrow", days: 1},
		{label: "+week", days: 7},
		{label: "+30 days", days: 30},
		{label: "clear", clear: true},
	}
	for i, expected := range want {
		got := formDueDatePresets[i]
		if got.label != expected.label || got.isClear != expected.clear {
			t.Errorf("form preset %d = %q/clear=%v, want %q/clear=%v",
				i, got.label, got.isClear, expected.label, expected.clear)
		}
		if !expected.clear && got.daysFromNow != expected.days {
			t.Errorf("form preset %q = %d days, want %d", got.label, got.daysFromNow, expected.days)
		}
	}
}

// Every shared key still reaches the draft alone. The creation form commits on
// save, so a preset key here moves a value in memory and nothing on disk.
func TestFormDueDatePresetKeysChangeOnlyTheDraft(t *testing.T) {
	today := util.StartOfDay(time.Now())
	for _, testCase := range []struct {
		key  string
		want *time.Time
	}{
		{key: "t", want: pointerTo(today)},
		{key: "m", want: pointerTo(today.AddDate(0, 0, 1))},
		{key: "w", want: pointerTo(today.AddDate(0, 0, 7))},
		{key: "n", want: pointerTo(today.AddDate(0, 0, 30))},
		{key: "0", want: nil},
	} {
		t.Run(testCase.key, func(t *testing.T) {
			m := newTestTaskForm()
			m = navigateToScheduleField(m)
			m = sendFormKey(m, syntheticCtrlKey('d'))
			m = sendFormKey(m, syntheticKey(rune(testCase.key[0])))
			if m.activeOverlay != formOverlayNone {
				t.Fatalf("%q left the preset overlay open", testCase.key)
			}
			switch {
			case testCase.want == nil && m.draft.dueDate != nil:
				t.Fatalf("%q set the draft due date to %s, want it cleared", testCase.key, m.draft.dueDate)
			case testCase.want != nil && m.draft.dueDate == nil:
				t.Fatalf("%q cleared the draft due date, want %s", testCase.key, testCase.want)
			case testCase.want != nil && !m.draft.dueDate.Equal(*testCase.want):
				t.Fatalf("%q set the draft due date to %s, want %s", testCase.key, m.draft.dueDate, testCase.want)
			}
			if testCase.want != nil && !m.hasChanges {
				t.Fatalf("%q left the form clean, want an unsaved change", testCase.key)
			}
		})
	}
}

// A key the due field does not bind is not the form's either: the hour keys
// belong to a deferral, and the form has no deferral to write.
func TestFormDueDatePickerIgnoresTheDeferOnlyKeys(t *testing.T) {
	for _, key := range []rune{'1', '4', '8', '9'} {
		m := newTestTaskForm()
		m = navigateToScheduleField(m)
		m = sendFormKey(m, syntheticCtrlKey('d'))
		m = sendFormKey(m, syntheticKey(key))
		if m.activeOverlay != formOverlayDue {
			t.Fatalf("%q closed the form's due picker", string(key))
		}
		if m.draft.dueDate != nil {
			t.Fatalf("%q set the draft due date to %s", string(key), m.draft.dueDate)
		}
	}
}

func pointerTo(value time.Time) *time.Time { return &value }
