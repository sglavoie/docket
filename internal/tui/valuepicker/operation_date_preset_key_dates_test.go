package valuepicker

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

// The preset keyboard is only as trustworthy as the instants it commits, and
// the two days a year a wall clock jumps are where a relative distance stops
// being arithmetic. Every key in both fields is resolved here against an
// explicit zoned instant, so the table is held to a date rather than to
// whatever today happens to be — which is also why these expectations are
// spelled out instead of recomputed from the same call they check.
func TestEveryPresetKeyCommitsAStatedInstantAcrossADSTTransition(t *testing.T) {
	zone, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load zone: %v", err)
	}

	cases := []struct {
		name      string
		now       time.Time
		dueKeys   map[string]string
		deferKeys map[string]string
	}{
		{
			name: "spring forward",
			now:  time.Date(2026, 3, 7, 15, 26, 53, 589, zone),
			dueKeys: map[string]string{
				"t": "2026-03-07 00:00",
				"m": "2026-03-08 00:00",
				"w": "2026-03-14 00:00",
				"n": "2026-04-06 00:00",
			},
			deferKeys: map[string]string{
				"1": "2026-03-07 16:26",
				"4": "2026-03-07 19:26",
				"8": "2026-03-07 23:26",
				"9": "2026-03-08 09:00",
				"m": "2026-03-08 00:00",
				"w": "2026-03-14 00:00",
				"n": "2026-04-06 00:00",
			},
		},
		{
			name: "fall back",
			now:  time.Date(2026, 10, 31, 15, 26, 53, 589, zone),
			dueKeys: map[string]string{
				"t": "2026-10-31 00:00",
				"m": "2026-11-01 00:00",
				"w": "2026-11-07 00:00",
				"n": "2026-11-30 00:00",
			},
			deferKeys: map[string]string{
				"1": "2026-10-31 16:26",
				"4": "2026-10-31 19:26",
				"8": "2026-10-31 23:26",
				"9": "2026-11-01 09:00",
				"m": "2026-11-01 00:00",
				"w": "2026-11-07 00:00",
				"n": "2026-11-30 00:00",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, field := range []struct {
				deferred bool
				want     map[string]string
			}{{deferred: false, want: testCase.dueKeys}, {deferred: true, want: testCase.deferKeys}} {
				label := DateSectionLabel(field.deferred)
				presets := OperationDatePresets(field.deferred, testCase.now)
				keyed := 0
				for _, preset := range presets {
					key, bound := chrome.DatePresetKey(field.deferred, preset.id)
					if !bound {
						t.Errorf("%s preset %q carries no key", label, preset.id)
						continue
					}
					if preset.clear {
						// The clear row names no instant, so it is the one keyed
						// preset with nothing for a transition to move.
						if key != "0" {
							t.Errorf("%s clear row key = %q, want 0", label, key)
						}
						continue
					}
					keyed++
					want, stated := field.want[key]
					if !stated {
						t.Errorf("%s key %q (%s) has no stated instant for this transition", label, key, preset.id)
						continue
					}
					if resolved, ok := chrome.DatePresetIDForKey(field.deferred, key); !ok || resolved != preset.id {
						t.Errorf("%s key %q resolves to %q, want %q", label, key, resolved, preset.id)
						continue
					}
					if got := preset.value().In(zone).Format("2006-01-02 15:04"); got != want {
						t.Errorf("%s key %q (%s) = %q, want %q", label, key, preset.id, got, want)
					}
				}
				if keyed != len(field.want) {
					t.Errorf("%s stated %d instants for %d dated keys", label, len(field.want), keyed)
				}
			}
		})
	}
}
