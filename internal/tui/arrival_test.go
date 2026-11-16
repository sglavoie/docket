package tui

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/outline"
)

// observeCommits makes the absence of a log write observable without
// reaching into SQLite; every stored operation passes through Commit.
type observeCommits struct {
	Backend
	calls int
}

func (b *observeCommits) Commit(ops []outline.Op, now int64) error {
	b.calls++
	return b.Backend.Commit(ops, now)
}

func TestGlOpensAddress(t *testing.T) {
	const address = "https://example.com/first"
	fixture := func(t *testing.T) (*harness, *[]string) {
		t.Helper()
		h := start(t).wide()
		h.pasteText("Project\n\tRead " + address + ".,;:!?) http://example.org/second $open\n\tDestination $open")
		h.press("g", "g", "j")
		opened := []string{}
		h.m.openURL = func(url string) error {
			opened = append(opened, url)
			return nil
		}
		return h, &opened
	}
	for _, place := range []string{"outline", "list", "filter"} {
		t.Run(place, func(t *testing.T) {
			h, opened := fixture(t)
			switch place {
			case "list":
				h.press("t", "f", `"Today"`, "enter")
			case "filter":
				h.press("/", `"Read"`, "tab", "enter")
			}
			// Leave both a remembered change and redo history to preserve.
			h.press("!", "u")
			rows, header := h.rows(), h.header()
			gen, undo, redo := h.m.tree.Gen(), len(h.m.undo), len(h.m.redo)
			older, newer, repeat := slices.Clone(h.m.older), slices.Clone(h.m.newer), h.m.repeat
			backend := &observeCommits{Backend: h.m.backend}
			h.m.backend = backend
			h.press("g", "l")
			if !slices.Equal(*opened, []string{address}) || h.status() != "opened "+address {
				t.Fatalf("opened = %v, status = %q", *opened, h.status())
			}
			if h.rows() != rows || h.header() != header || h.m.tree.Gen() != gen ||
				len(h.m.undo) != undo || len(h.m.redo) != redo || backend.calls != 0 ||
				!slices.Equal(h.m.older, older) || !slices.Equal(h.m.newer, newer) || h.m.repeat != repeat {
				t.Fatal("opening an address changed rows, history, the log, the place or dot repeat")
			}
			h.press(".")
			if len(*opened) != 1 || h.m.tree.Row(h.m.current()).Fields["flag"] != "1" {
				t.Fatal("dot did not retain the earlier flag change")
			}
		})
	}
	t.Run("opener error", func(t *testing.T) {
		h, opened := fixture(t)
		h.m.openURL = func(url string) error {
			*opened = append(*opened, url)
			return errors.New("opener unavailable")
		}
		rows, steps, gen := h.rows(), len(h.m.undo), h.m.tree.Gen()
		h.press("g", "l")
		if !slices.Equal(*opened, []string{address}) || h.status() != "opener unavailable" ||
			h.rows() != rows || len(h.m.undo) != steps || h.m.tree.Gen() != gen {
			t.Fatalf("failed opener changed rows or lost its error: %q", h.status())
		}
	})
	t.Run("row link wins even when gone", func(t *testing.T) {
		h, opened := fixture(t)
		h.press("L", `"Destination"`, "enter")
		rows, steps := h.rows(), len(h.m.undo)
		h.press("g", "l")
		h.want("  ○ Read "+address+".,;:!?) http://example.org/second", "    → Destination", "▌ ○ Destination")
		if len(*opened) != 0 || len(h.m.undo) != steps {
			t.Fatal("following a row link opened an address or made an undo step")
		}
		h.press("ctrl+o")
		if h.rows() != rows {
			t.Fatal("the row link did not leave a jump back")
		}
		h.press("g", "l", "d", "d", "ctrl+o", "g", "l")
		if len(*opened) != 0 || h.status() != "the linked row is gone" {
			t.Fatalf("gone link fell through to the address: %q", h.status())
		}
	})
	t.Run("selection and no address", func(t *testing.T) {
		h, opened := fixture(t)
		h.press("v")
		rows, steps := h.rows(), len(h.m.undo)
		h.press("g", "l")
		if !strings.HasPrefix(h.status(), "that works on one row: press Esc to clear the selection") ||
			h.rows() != rows || len(h.m.undo) != steps || len(*opened) != 0 {
			t.Fatalf("selection was not refused: %q", h.status())
		}
		h.press("esc", "j", "g", "l")
		if h.status() != "this row has no link: L makes one" || len(*opened) != 0 {
			t.Fatalf("row without a link or address = %q", h.status())
		}
	})
}

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
					"    ▾ Notes: 2/2",
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
				after[2] = "    ▾ Notes: 0/2"
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

func TestHeaderCountsAndSums(t *testing.T) {
	t.Run("held rows count themselves, not subtrees or context", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Parent #est=1 $open\n\tChild #est=30m $open")
		h.press("g", "g", "t", "f", `"Today"`, "enter")
		if got := h.header(); got != "Today  · 1 to do · 1h" {
			t.Fatalf("list counted descendants: %q", got)
		}
		h.press("f", `"Projects"`, "enter", "/", `"Child"`, "enter")
		if got := h.header(); got != "Projects  · 1 to do · 30m  / Child" {
			t.Fatalf("filter counted context ancestor: %q", got)
		}
		h.press("esc", "/", `"#est"`, "enter")
		if got := h.header(); got != "Projects  · 2 to do · 1h30m  / #est" {
			t.Fatalf("filter counted descendants twice: %q", got)
		}
		h.press("esc", "g", "g", "enter")
		if got := h.header(); got != "Parent  · 1 to do · 30m" {
			t.Fatalf("outline counted its root: %q", got)
		}
		h.press("#", `"est=2"`, "enter")
		if got := h.header(); got != "Parent  · 1 to do · 2h" {
			t.Fatalf("estimate edit left stale totals: %q", got)
		}
		h.press("u")
		if got := h.header(); got != "Parent  · 1 to do · 30m" {
			t.Fatalf("undo left stale totals: %q", got)
		}
	})
	h := start(t).wide()
	h.pasteText("Project\n\tStorage:\n\t\tBuild #est=1.5h $doing\n\t\tWait #est=30m $waiting\n\t\tReview #est=2d $open\n\t\tBare #est=2 $open\n\t\tUnreadable #est=oops $open\n\t\tDone #est=99h $done\n\t\tCanceled #est=99h $canceled\n\t\tParked #est=99h $parked\n\t\tNote #est=99h $note\n\tOutside $open")
	wantHeader := func(want string) {
		t.Helper()
		if got := h.header(); got != want {
			t.Fatalf("header = %q, want %q", got, want)
		}
	}
	wantHeader("Projects  · 6 to do · 20h")
	h.press("g", "g", "enter", "h")
	wantHeader("Project  · 6 to do · 20h")
	if !strings.Contains(h.rows(), "▸ Storage: 1/6") || strings.Contains(h.rows(), "Build") {
		t.Fatalf("folded outline =\n%s", h.rows())
	}
	h.press("enter")
	wantHeader("Project › Storage:  · 5 to do · 20h")
	h.press("g", "g", "t", "j", "t", "f", `"Today"`, "enter")
	wantHeader("Today  · 2 to do · 2h")
	h.press("g", "g", "x")
	wantHeader("Today  · 1 to do · 30m")
	h.press("j", "x")
	wantHeader("Today") // Held closed rows contribute neither count nor estimate.
	h.press("f", `"Projects"`, "enter", "/", `"#est"`, "enter")
	wantHeader("Projects  · 3 to do · 18h  / #est")
	h.press("/", "tab", "enter")
	wantHeader("Projects  · 3 to do · 18h  / #est  (flat)")
	h.press("esc", "/", `"Unreadable"`, "enter")
	wantHeader("Projects  · 1 to do  / Unreadable  (flat)")
	h.press("x")
	wantHeader("Projects  / Unreadable  (flat)")

	for _, tc := range []struct{ value, sum string }{
		{"30m", "30m"}, {"1.5h", "1h30m"}, {"2d", "16h"}, {"2", "2h"},
		{"0", "0m"}, {"0.01h", "1m"}, {"oops", ""}, {"NaN", ""}, {"Inf", ""}, {"-2h", ""},
	} {
		t.Run(tc.value, func(t *testing.T) {
			h := start(t).wide()
			h.pasteText("Work #est=" + tc.value + " $open\nOther $note")
			want := "Projects  · 1 to do"
			if tc.sum != "" {
				want += " · " + tc.sum
			}
			if got := h.header(); got != want {
				t.Fatalf("header = %q, want %q", got, want)
			}
		})
	}
}

func TestHeadingProgress(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Project\n\tStorage:\n\t\tBuild $open\n\t\tNested:\n\t\t\tDone $done\n\t\t\tWait $waiting\n\t\t\tCanceled $canceled\n\t\t\tParked $parked\n\tEmpty:\n\t\tNote $note")
	h.press("g", "g", "enter")
	wantProgress := func(storage, nested string) {
		t.Helper()
		rows := h.rows()
		for _, want := range []string{"Storage: " + storage, "Nested: " + nested, "Empty:"} {
			if !strings.Contains(rows, want) {
				t.Fatalf("missing %q in\n%s", want, rows)
			}
		}
		if strings.Contains(rows, "Empty: 0/") {
			t.Fatalf("empty heading has progress:\n%s", rows)
		}
	}
	wantProgress("1/3", "1/2")
	if !strings.Contains(h.m.View().Content, sgrDim+"1/3"+sgrReset) {
		t.Fatal("heading progress must be dimmed")
	}
	h.press("j", "x")
	wantProgress("2/3", "1/2")
	h.press("j", "j", "x")
	wantProgress("1/3", "0/2")
	h.press("u")
	wantProgress("2/3", "1/2")
	h.press("g", "g", "h")
	if rows := h.rows(); !strings.Contains(rows, "▸ Storage: 2/3") || strings.Contains(rows, "Nested:") {
		t.Fatalf("folded progress:\n%s", rows)
	}
	h.press("l")
	wantProgress("2/3", "1/2")
}
