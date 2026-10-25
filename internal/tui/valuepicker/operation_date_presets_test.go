package valuepicker

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/util"
)

// The clock is a Saturday, so the weekend row names the next one rather than
// today, and Monday is two days out.
func TestDueDatePresetsStayCalendarDays(t *testing.T) {
	now := time.Date(2026, 3, 14, 15, 26, 53, 589, time.Local)
	midnight := util.StartOfDay(now)

	want := []struct {
		id    string
		label string
		clear bool
		value time.Time
	}{
		{id: "clear", label: "Clear", clear: true},
		{id: "today", label: "Today", value: midnight},
		{id: "tomorrow", label: "Tomorrow", value: midnight.AddDate(0, 0, 1)},
		{id: "weekend", label: "This weekend", value: midnight.AddDate(0, 0, 7)},
		{id: "monday", label: "Start of next week", value: midnight.AddDate(0, 0, 2)},
		{id: "week", label: "In one week", value: midnight.AddDate(0, 0, 7)},
		{id: "month", label: "In 30 days", value: midnight.AddDate(0, 0, 30)},
	}

	presets := OperationDatePresets(false, now)
	if len(presets) != len(want) {
		t.Fatalf("due presets = %d rows, want %d", len(presets), len(want))
	}
	for i, expected := range want {
		got := presets[i]
		if got.id != expected.id || got.label != expected.label {
			t.Errorf("due preset %d = %q/%q, want %q/%q", i, got.id, got.label, expected.id, expected.label)
		}
		if got.clear != expected.clear {
			t.Errorf("due preset %q clear = %v, want %v", got.id, got.clear, expected.clear)
		}
		if expected.clear {
			if got.value != nil {
				t.Errorf("due preset %q carries a value", got.id)
			}
			continue
		}
		if value := got.value(); !value.Equal(expected.value) {
			t.Errorf("due preset %q value = %s, want %s", got.id, value, expected.value)
		}
	}
}

func TestDeferPresetsOfferHoursMorningAndDays(t *testing.T) {
	now := time.Date(2026, 3, 14, 15, 26, 53, 589, time.Local)
	midnight := util.StartOfDay(now)

	want := []struct {
		id    string
		label string
		clear bool
		value time.Time
	}{
		{id: "clear", label: "Clear defer", clear: true},
		{id: "1h", label: "In 1 hour", value: now.Add(time.Hour)},
		{id: "4h", label: "In 4 hours", value: now.Add(4 * time.Hour)},
		{id: "8h", label: "In 8 hours", value: now.Add(8 * time.Hour)},
		{id: "morning", label: "Tomorrow morning", value: time.Date(2026, 3, 15, 9, 0, 0, 0, time.Local)},
		{id: "tomorrow", label: "Until tomorrow", value: midnight.AddDate(0, 0, 1)},
		{id: "weekend", label: "Until the weekend", value: midnight.AddDate(0, 0, 7)},
		{id: "monday", label: "Until next week", value: midnight.AddDate(0, 0, 2)},
		{id: "week", label: "For one week", value: midnight.AddDate(0, 0, 7)},
		{id: "month", label: "For 30 days", value: midnight.AddDate(0, 0, 30)},
	}

	presets := OperationDatePresets(true, now)
	if len(presets) != len(want) {
		t.Fatalf("defer presets = %d rows, want %d", len(presets), len(want))
	}
	for i, expected := range want {
		got := presets[i]
		if got.id != expected.id || got.label != expected.label {
			t.Errorf("defer preset %d = %q/%q, want %q/%q", i, got.id, got.label, expected.id, expected.label)
		}
		if expected.clear {
			if !got.clear || got.value != nil {
				t.Errorf("defer preset %q is not the clear row", got.id)
			}
			continue
		}
		if value := got.value(); !value.Equal(expected.value) {
			t.Errorf("defer preset %q value = %s, want %s", got.id, value, expected.value)
		}
	}
}

// Tomorrow morning is a wall clock, not an elapsed duration: in a zone that
// shifts, nine hours after midnight reads 10:00 on the spring-forward day and
// 08:00 on the fall-back day.
func TestDeferPresetsOfferHoursMorningAndDaysAcrossDSTTransitions(t *testing.T) {
	zone, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load zone: %v", err)
	}

	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{name: "spring forward", now: time.Date(2026, 3, 7, 15, 26, 53, 589, zone), want: "2026-03-08 09:00"},
		{name: "fall back", now: time.Date(2026, 10, 31, 15, 26, 53, 589, zone), want: "2026-11-01 09:00"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var value time.Time
			found := false
			for _, preset := range OperationDatePresets(true, testCase.now) {
				if preset.id == "morning" {
					value = preset.value()
					found = true
				}
			}
			if !found {
				t.Fatal("the defer presets carry no morning row")
			}
			if got := value.In(zone).Format("2006-01-02 15:04"); got != testCase.want {
				t.Errorf("morning preset = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestDatePresetSecondaryTextNamesTheDayAndOnlyClocksAnInstant(t *testing.T) {
	midnight := time.Date(2026, 3, 14, 0, 0, 0, 0, time.Local)
	if got, want := OperationDateSecondaryText(midnight, midnight), "Sat Mar 14"; got != want {
		t.Errorf("midnight secondary = %q, want %q", got, want)
	}
	if got, want := OperationDateSecondaryText(midnight.Add(20*time.Hour+15*time.Minute), midnight), "Sat Mar 14 20:15"; got != want {
		t.Errorf("intra-day secondary = %q, want %q", got, want)
	}
}

func TestRelativeDateSelectedIDHighlightsFirstReproduciblePreset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Keep hour-based instants distinct from midnight and 09:00 presets in
		// every local time zone, even though this test freezes time exactly.
		afternoon := util.StartOfDay(time.Now()).AddDate(0, 0, 1).Add(15*time.Hour + 26*time.Minute)
		time.Sleep(time.Until(afternoon))
		// Cover every weekday with a frozen clock. Presets can be aliases:
		// on Friday, tomorrow and weekend both mean Saturday. The first row
		// owns the highlight because the stored date retains no preset ID.
		for range 7 {
			now := time.Now()
			for _, deferred := range []bool{false, true} {
				first := make(map[time.Time]listsurface.ListItemID)
				for _, preset := range OperationDatePresets(deferred, now) {
					if preset.clear || preset.id == "1h" || preset.id == "4h" || preset.id == "8h" {
						continue
					}
					value := preset.value()
					if _, seen := first[value]; !seen {
						first[value] = DateListItemID(deferred, preset.id)
					}
					got, want := RelativeDateSelectedID(&value, deferred), first[value]
					if len(got) != 1 || got[0] != want {
						t.Errorf("%s deferred=%v preset %q highlighted %v, want [%s]", now.Weekday(), deferred, preset.id, got, want)
					}
				}
			}
			time.Sleep(24 * time.Hour)
		}
	})
}

func TestRelativeDateSelectedIDRefusesToApproximateAnIntraDayDefer(t *testing.T) {
	// Tomorrow 09:30 is half an hour past the "Tomorrow morning" preset and is
	// unreachable by any "+Nh" row, whose value would have to be more than a day
	// out to land there. Nothing may highlight.
	probe := util.StartOfDay(time.Now()).AddDate(0, 0, 1).Add(9*time.Hour + 30*time.Minute)
	if got := RelativeDateSelectedID(&probe, true); got != nil {
		t.Errorf("a near-miss instant highlighted %v, want no preset", got)
	}

	todayMidnight := util.StartOfDay(time.Now())
	if got := RelativeDateSelectedID(&todayMidnight, true); got != nil {
		t.Errorf("today highlighted %v, but the defer list has no Today row", got)
	}
	if got := RelativeDateSelectedID(&todayMidnight, false); len(got) != 1 || got[0] != "date:due:today" {
		t.Errorf("today highlighted %v on the due list, want [date:due:today]", got)
	}
}

// The combined picker gives every scheduling section its own row identity:
// "tomorrow" means a date value in Due or Defer, while Check-in is a typed
// editor action and cannot masquerade as either.
func TestDateListItemIDsSeparateTheThreeSections(t *testing.T) {
	source := OperationDateListSource("", bothDateFieldsSet)
	seen := make(map[listsurface.ListItemID]bool, len(source.Items))
	deferredRows := 0
	checkInRows := 0
	for _, item := range source.Items {
		if seen[item.ID] {
			t.Errorf("date list repeats row ID %q", item.ID)
		}
		seen[item.ID] = true
		value, isDate := item.Data.(operation.OperationDateValue)
		if !isDate {
			if _, isAction := item.Data.(operation.OperationCheckInEditorAction); !isAction || item.ID != dateCheckInEditorItemID || item.CategoryID != dateListCategoryCheckIn {
				t.Fatalf("non-date row %q carries %T in %q", item.ID, item.Data, item.CategoryID)
			}
			checkInRows++
			continue
		}
		if value.Deferred {
			deferredRows++
			if item.CategoryID != dateListCategoryDefer {
				t.Errorf("deferred row %q sits in category %q", item.ID, item.CategoryID)
			}
			continue
		}
		if item.CategoryID != dateListCategoryDue {
			t.Errorf("due row %q sits in category %q", item.ID, item.CategoryID)
		}
	}
	if want := len(OperationDatePresets(true, time.Now())); deferredRows != want {
		t.Errorf("defer section = %d rows, want %d", deferredRows, want)
	}
	if checkInRows != 1 {
		t.Errorf("check-in section = %d rows, want 1", checkInRows)
	}
}

// A terminal task keeps the due section reachable: only the defer rows carry
// the refusal, which is what lets one key answer both fields on a Done task.
func TestTerminalTaskDisablesOnlyTheDeferSection(t *testing.T) {
	source := OperationDateListSource("terminal tasks cannot be deferred", bothDateFieldsSet)
	for _, item := range source.Items {
		value, isDate := item.Data.(operation.OperationDateValue)
		if !isDate {
			if !item.Enabled() {
				t.Errorf("check-in action %q is disabled", item.ID)
			}
			continue
		}
		if value.Deferred == item.Enabled() {
			t.Errorf("row %q enabled = %v with deferred = %v", item.ID, item.Enabled(), value.Deferred)
		}
	}
	for _, item := range OperationDueDateListSource(true).Items {
		if !item.Enabled() {
			t.Errorf("due-only row %q is disabled", item.ID)
		}
		if item.CategoryID != "" {
			t.Errorf("due-only row %q carries category %q, want an unsectioned list", item.ID, item.CategoryID)
		}
	}
}
