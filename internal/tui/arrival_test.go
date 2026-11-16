package tui

import (
	"maps"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRepeatReopensRowsUnder(t *testing.T) {
	for _, route := range []struct {
		name string
		keys []string
	}{
		{"x", []string{"x"}},
		{"stamp", []string{"$", `"done"`, "enter"}},
		{"line", []string{"A", `" $done"`, "esc"}},
	} {
		for _, dated := range []bool{false, true} {
			t.Run(route.name+"/dated="+strconv.FormatBool(dated), func(t *testing.T) {
				h := start(t).wide()
				parent := "Weekly *weekly $open"
				beforeParent, afterParent := "▌ ○ Weekly *1w", "▌ ○ Weekly @@Nov 22 *1w"
				if dated {
					parent = "Weekly @today @@today *weekly $doing"
					beforeParent = "▌ ● Weekly @today @@today *1w"
					afterParent = "▌ ○ Weekly @Nov 22 @@Nov 22 *1w"
				}
				h.pasteText(parent + "\n" + strings.Join([]string{
					"\tDirect @tomorrow @@today $done",
					"\tNotes: $note",
					"\t\tNested $done",
					"\t\t\tDeep @fri @@tomorrow $done",
					"\tCanceled $canceled",
					"\t\tUnder canceled $done",
					"\tParked $parked",
					"\tWaiting $waiting",
					"\tDoing $doing",
					"\tOpen $open",
					"\tRepeating @fri @@tomorrow *daily $open",
					"Outside $done",
				}, "\n"))
				h.press("g", "g")
				before := []string{
					beforeParent,
					"    ✓ Direct @tomorrow @@today",
					"    ▾ Notes:",
					"      ✓ Nested",
					"        ✓ Deep @fri @@tomorrow",
					"    ✕ Canceled",
					"      ✓ Under canceled",
					"    ◌ Parked",
					"    ◔ Waiting",
					"    ● Doing",
					"    ○ Open",
					"    ○ Repeating @fri @@tomorrow *1d",
					"  ✓ Outside",
				}
				h.want(before...)
				after := append([]string(nil), before...)
				after[0] = afterParent
				for _, i := range []int{1, 3, 4, 6} {
					after[i] = strings.Replace(after[i], "✓", "○", 1)
				}

				// Fields verify clocks and untouched dates that the drawn rows
				// cannot fully show; the interaction itself stays on the keys.
				original := map[string]map[string]string{}
				for _, v := range h.m.vis {
					row := h.m.tree.Row(v.id)
					original[row.ID] = maps.Clone(row.Fields)
				}
				parentID, steps := h.m.current(), len(h.m.undo)
				h.clock = h.clock.Add(time.Hour)
				for _, keys := range [][]string{route.keys, {"."}} {
					h.press(keys...)
					h.want(after...)
					if h.status() != "repeats: next Nov 22" {
						t.Fatalf("status = %q", h.status())
					}
					if len(h.m.undo) != steps+1 {
						t.Fatal("closing the checklist must be one undo step")
					}
					for id, fields := range original {
						want := maps.Clone(fields)
						if id == parentID {
							want["start"] = "2026-11-22"
							if dated {
								want["due"] = "2026-11-22"
								want["state"] = "open"
								want["changed"] = strconv.FormatInt(h.clock.UnixMilli(), 10)
							}
						} else if h.m.tree.Contains(parentID, id) && fields["state"] == "done" {
							want["state"] = "open"
							want["changed"] = strconv.FormatInt(h.clock.UnixMilli(), 10)
						}
						if got := h.m.tree.Row(id).Fields; !maps.Equal(got, want) {
							t.Fatalf("%s fields = %v, want %v", fields["text"], got, want)
						}
					}
					h.press("u")
					h.want(before...)
					for id, want := range original {
						if got := h.m.tree.Row(id).Fields; !maps.Equal(got, want) {
							t.Fatalf("undo %s fields = %v, want %v", want["text"], got, want)
						}
					}
				}
			})
		}
	}
}
