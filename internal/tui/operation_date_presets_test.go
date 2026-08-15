package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

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
		{id: "week", label: "In one week", value: midnight.AddDate(0, 0, 7)},
		{id: "month", label: "In one month", value: midnight.AddDate(0, 0, 30)},
	}

	presets := operationDatePresets(false, now)
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
		{id: "week", label: "For one week", value: midnight.AddDate(0, 0, 7)},
		{id: "month", label: "For one month", value: midnight.AddDate(0, 0, 30)},
	}

	presets := operationDatePresets(true, now)
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
			for _, preset := range operationDatePresets(true, testCase.now) {
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
	if got, want := operationDateSecondaryText(midnight), "Sat Mar 14"; got != want {
		t.Errorf("midnight secondary = %q, want %q", got, want)
	}
	if got, want := operationDateSecondaryText(midnight.Add(20*time.Hour+15*time.Minute)), "Sat Mar 14 20:15"; got != want {
		t.Errorf("intra-day secondary = %q, want %q", got, want)
	}
}

func TestRelativeDateSelectedIDHighlightsEveryReproduciblePreset(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		for _, preset := range operationDatePresets(deferred, time.Now()) {
			if preset.clear || preset.id == "1h" || preset.id == "4h" || preset.id == "8h" {
				continue
			}
			value := preset.value()
			got := relativeDateSelectedID(&value, deferred)
			want := ListItemID("date:" + preset.id)
			if len(got) != 1 || got[0] != want {
				t.Errorf("deferred=%v preset %q highlighted %v, want [%s]", deferred, preset.id, got, want)
			}
		}
	}
}

func TestRelativeDateSelectedIDRefusesToApproximateAnIntraDayDefer(t *testing.T) {
	// Tomorrow 09:30 is half an hour past the "Tomorrow morning" preset and is
	// unreachable by any "+Nh" row, whose value would have to be more than a day
	// out to land there. Nothing may highlight.
	probe := util.StartOfDay(time.Now()).AddDate(0, 0, 1).Add(9*time.Hour + 30*time.Minute)
	if got := relativeDateSelectedID(&probe, true); got != nil {
		t.Errorf("a near-miss instant highlighted %v, want no preset", got)
	}

	todayMidnight := util.StartOfDay(time.Now())
	if got := relativeDateSelectedID(&todayMidnight, true); got != nil {
		t.Errorf("today highlighted %v, but the defer list has no Today row", got)
	}
	if got := relativeDateSelectedID(&todayMidnight, false); len(got) != 1 || got[0] != "date:today" {
		t.Errorf("today highlighted %v on the due list, want [date:today]", got)
	}
}
