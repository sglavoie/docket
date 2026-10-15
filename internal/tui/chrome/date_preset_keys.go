package chrome

// The date preset shortcut table lives here because more than one date surface
// reads it: the value picker `@` opens, its deadline-only projection, and the
// creation form's inline due picker. One table is
// what keeps a key from meaning one distance on one of them and another distance
// on the next, and it is the same table the rows print their hints from, so a
// surface cannot advertise a key its dispatch does not honor.
//
// The two fields keep separate vocabularies on purpose. `t` is Today and a
// deferral has no Today row, so `t` is unbound under Defer rather than quietly
// landing on something near it; the hour keys are unbound under Due for the same
// reason, since a due date is a calendar day. Only `0`, `m`, `s`, `o`, `w`, and
// `n` are shared, and each names the same relative distance in both fields: `s`
// the coming Saturday and `o` the coming Monday, both strictly after today.

// The preset IDs. The picker's rows and the form's rows are both built from
// these, so a shortcut and the row it applies cannot drift apart into two
// spellings of one preset.
const (
	DatePresetClear      = "clear"
	DatePresetToday      = "today"
	DatePresetTomorrow   = "tomorrow"
	DatePresetWeek       = "week"
	DatePresetMonth      = "month"
	DatePresetOneHour    = "1h"
	DatePresetFourHours  = "4h"
	DatePresetEightHours = "8h"
	DatePresetMorning    = "morning"
	DatePresetWeekend    = "weekend"
	DatePresetNextMonday = "monday"
)

var (
	dueDatePresetKeys = map[string]string{
		DatePresetClear:      "0",
		DatePresetToday:      "t",
		DatePresetTomorrow:   "m",
		DatePresetWeekend:    "s",
		DatePresetNextMonday: "o",
		DatePresetWeek:       "w",
		DatePresetMonth:      "n",
	}
	deferDatePresetKeys = map[string]string{
		DatePresetClear:      "0",
		DatePresetOneHour:    "1",
		DatePresetFourHours:  "4",
		DatePresetEightHours: "8",
		DatePresetMorning:    "9",
		DatePresetTomorrow:   "m",
		DatePresetWeekend:    "s",
		DatePresetNextMonday: "o",
		DatePresetWeek:       "w",
		DatePresetMonth:      "n",
	}
)

func datePresetKeys(deferred bool) map[string]string {
	if deferred {
		return deferDatePresetKeys
	}
	return dueDatePresetKeys
}

// DatePresetKey names the key that applies one preset in one field. It is what a
// row's hint prints, so an unkeyed preset reads as an absent hint rather than as
// a key nothing answers.
func DatePresetKey(deferred bool, presetID string) (string, bool) {
	key, ok := datePresetKeys(deferred)[presetID]
	return key, ok
}

// DatePresetIDForKey resolves a pressed key inside one field. The field is the
// caller's to decide — the cursor's row names it on the combined picker — and a
// key the field does not bind resolves to nothing rather than to the other
// field's row.
func DatePresetIDForKey(deferred bool, key string) (string, bool) {
	for presetID, bound := range datePresetKeys(deferred) {
		if bound == key {
			return presetID, true
		}
	}
	return "", false
}

// DatePresetShortcutKey reports whether a key names a preset in either field. A
// date surface claims all of them whichever field its cursor sits in, so a key
// bound only in the other field stays silent instead of falling through to
// whatever the surrounding browse grammar happens to spell with the same letter.
func DatePresetShortcutKey(key string) bool {
	if _, ok := DatePresetIDForKey(false, key); ok {
		return true
	}
	_, ok := DatePresetIDForKey(true, key)
	return ok
}
