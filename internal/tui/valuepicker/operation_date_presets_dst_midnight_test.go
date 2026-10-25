package valuepicker

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/util"
)

// In a zone whose daylight-saving change skips midnight, every day-valued
// preset row must still land on the start of the date it names: stepping a
// local day-start with AddDate answers 23:00 the day before, or 01:00 on the
// following date, which a due date would then read as a time of day.
func TestDayPresetsLandOnTheStartOfTheirDateWhereMidnightIsSkipped(t *testing.T) {
	cases := []struct {
		zone string
		now  time.Time
		due  map[string]string
	}{
		{
			zone: "America/Havana",
			now:  time.Date(2026, time.March, 7, 15, 0, 0, 0, time.UTC),
			due: map[string]string{
				chrome.DatePresetToday:      "2026-03-07",
				chrome.DatePresetTomorrow:   "2026-03-08",
				chrome.DatePresetWeekend:    "2026-03-14",
				chrome.DatePresetNextMonday: "2026-03-09",
				chrome.DatePresetWeek:       "2026-03-14",
				chrome.DatePresetMonth:      "2026-04-06",
			},
		},
		{
			zone: "America/Havana",
			now:  time.Date(2026, time.March, 8, 15, 0, 0, 0, time.UTC),
			due: map[string]string{
				chrome.DatePresetToday:      "2026-03-08",
				chrome.DatePresetTomorrow:   "2026-03-09",
				chrome.DatePresetWeekend:    "2026-03-14",
				chrome.DatePresetNextMonday: "2026-03-09",
				chrome.DatePresetWeek:       "2026-03-15",
				chrome.DatePresetMonth:      "2026-04-07",
			},
		},
		{
			zone: "America/Santiago",
			now:  time.Date(2026, time.September, 5, 15, 0, 0, 0, time.UTC),
			due: map[string]string{
				chrome.DatePresetToday:      "2026-09-05",
				chrome.DatePresetTomorrow:   "2026-09-06",
				chrome.DatePresetWeekend:    "2026-09-12",
				chrome.DatePresetNextMonday: "2026-09-07",
				chrome.DatePresetWeek:       "2026-09-12",
				chrome.DatePresetMonth:      "2026-10-05",
			},
		},
	}
	for _, tc := range cases {
		loc, err := time.LoadLocation(tc.zone)
		if err != nil {
			t.Skipf("zone database unavailable for %s: %v", tc.zone, err)
		}
		now := tc.now.In(loc)
		for _, deferred := range []bool{false, true} {
			for _, preset := range OperationDatePresets(deferred, now) {
				want, ok := tc.due[preset.id]
				if !ok || preset.value == nil {
					continue
				}
				if deferred && preset.id == chrome.DatePresetToday {
					continue
				}
				got := preset.value()
				if got.Format(util.DateFormat) != want || !util.IsStartOfCalendarDate(got) {
					t.Errorf("%s %s deferred=%v %s = %s, want the start of %s", tc.zone, now.Format(util.DateFormat), deferred, preset.id, got, want)
				}
			}
		}
	}
}
