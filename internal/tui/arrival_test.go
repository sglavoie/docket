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
