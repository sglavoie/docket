package tui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// These are the states the daily-use refinement is about, written down while
// they still behave the old way. Each one is an expectation a named later
// session deliberately replaces, so each test says which session owns it and
// what that session is expected to change it to. They are contracts about the
// arrangement of the screen — which row carries what, where a segment starts,
// how many rows a thing costs, what a width does to it — and never whole-screen
// snapshots, because a snapshot of a state eleven sessions are about to move is
// a snapshot that fails for every reason at once and names none of them.
//
// A test here failing after a daily-use session is not automatically a
// regression. It is that session's own signal that it reached the behavior it
// was aimed at, and the session that reaches it owns rewriting the contract in
// the same change. docs/tui-testing/ux-daily-use-regression-map.md is the index
// of who owns what.
//
// A rewritten one stays in this test family, and stops being a baseline
// when it does: it then states the behavior its session reached and is an
// ordinary contract over it. The map records which session did the rewriting, so
// the two kinds stay tellable apart without reading every doc comment.
//
// Nothing here asserts the initial cursor position by accident. Two guidance tests
// are *about* it, and every other one seats the cursor itself through its
// frame's staging function, because session 11 moves where a launch lands and a
// contract that silently depended on today's landing would fail for a reason it
// never meant to state.

// uxDailyUseMatrixSizes is the review's floor, its two working sizes, and its
// widest: the same four the exported frames are taken at.
var uxDailyUseMatrixSizes = uxPresentationMatrixSizes

// uxDailyUseDailySize is the size this plan optimizes for.
var uxDailyUseDailySize = [2]int{100, 35}

// TestUXDailyUseSingletonGroupsLeadWithTheirProject is this contract after
// one-task project groups merged into their row. Each of the fixture's four
// projects holds one task, so no group draws a `[slug] (count)` header: the
// task row leads with its own `[slug]` instead, and the label stays inline, so
// four projects with four different name lengths leave no padding gap between
// a short label and its title.
//
// Both halves are asserted, because either alone passes on an arrangement this
// is meant to refuse: an inline title alone would still hold if the project
// were simply never named, and a label alone would hold beside a header that
// names the project a second time.
func TestUXDailyUseSingletonGroupsLeadWithTheirProject(t *testing.T) {
	for _, size := range uxDailyUseMatrixSizes {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, _ := uxDailyUseStagedDriver(t, "project-label-widths", size[0], size[1])
			screen := driver.Observe(ObserveOptions{}).PlainScreen
			lines := strings.Split(screen, "\n")

			for _, title := range uxDailyUseMultiProjectTitles {
				index := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, title) })
				if index < 0 {
					t.Fatalf("no screen row carries %q:\n%s", title, screen)
				}
				line := lines[index]
				label := strings.Index(line, "[")
				if label < 0 || label > strings.Index(line, title) {
					t.Fatalf("the row for %q does not lead with its project:\n%s", title, line)
				}
				if index > 0 && strings.Contains(lines[index-1], "] (") {
					t.Fatalf("a project header still opens the one-task group holding %q:\n%s", title, screen)
				}
				if gap := line[strings.Index(line, "]")+1 : strings.Index(line, title)]; strings.Contains(gap, "  ") {
					t.Fatalf("the label for %q is padded away from its title:\n%s", title, line)
				}
			}
		})
	}
}

// uxDailyUseMultiProjectTitles is the `multi-project` fixture's four tasks, one
// per project. The fixture is the only reviewed catalog whose project names have
// four different lengths, which is what makes the row's arithmetic observable.
var uxDailyUseMultiProjectTitles = []string{
	uxDailyUseAlphaTitle,
	"Beta rollout checklist",
	"Renew passport",
	"Finish the distributed systems book",
}

// uxDailyUseScreenLine is the one screen line carrying a fragment. It fails
// rather than returning nothing, because every caller below is asking about the
// arrangement of a row it has already staged and a missing row means the staging
// did not reach the state, not that the arrangement is wrong.
func uxDailyUseScreenLine(t *testing.T, screen, fragment string) string {
	t.Helper()
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, fragment) {
			return line
		}
	}
	t.Fatalf("no screen row carries %q:\n%s", fragment, screen)
	return ""
}

// TestUXDailyUseCursorRowWearsOneBandUnderItsContent is this contract after
// session 3 rewrote it. The cursor row now carries a background across its whole
// width, and the claim is about what that costs: the band goes *under* the row,
// so every foreground the row earned is still on the line, and the one mark that
// is not a color survives a terminal that paints none.
//
// One task's row is captured twice — once as the cursor row and once with the
// cursor a row below it — and the difference between the two captures is what
// every assertion reads. The frame carries a base background on every cell, so
// "the cursor row sets a background" is true of every row on screen and would be
// a contract about the compositor; comparing one row against itself is a
// contract about the cursor.
//
// The foregrounds are checked as a set taken off the unfocused capture rather
// than named one at a time, because the row's own signals are what it happens to
// draw at this size and this fixture, and a named list would quietly stop
// covering one the moment the row dropped it. The per-signal claim — state,
// priority, deadline, review, dependencies, and a search match, each legible on
// the band in both themes — is stated at the row level in
// outliner_cursor_colors_test.go, where all six can be put on one task.
// outliner_cursor_highlight_test.go covers the flash and the visual selection,
// which are gestures rather than frames.
func TestUXDailyUseCursorRowWearsOneBandUnderItsContent(t *testing.T) {
	driver, view := uxDailyUseDriver(t, FixtureMultiProject, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	seatCursorOnTaskTitled(t, view, uxDailyUseAlphaTitle)
	focused := uxDailyUseScreenLine(t, driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, uxDailyUseAlphaTitle)

	uxClarityPress(t, driver, uxClarityKeys("j")...)
	unfocused := uxDailyUseScreenLine(t, driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, uxDailyUseAlphaTitle)

	if ansi.Strip(focused) == focused {
		t.Fatalf("the row carries no styling at all, so the capture is not the styled frame:\n%q", focused)
	}

	focusedContent := uxDailyUseRowContent(t, focused, uxDailyUseAlphaTitle)
	unfocusedContent := uxDailyUseRowContent(t, unfocused, uxDailyUseAlphaTitle)
	band := backgroundParams(t, theme.ColorCursorRowBg)
	if !strings.Contains(focusedContent, band) {
		t.Fatalf("the cursor row's content wears no band:\n%q", focusedContent)
	}
	if strings.Contains(unfocusedContent, band) {
		t.Fatalf("a row the cursor left still wears the band:\n%q", unfocusedContent)
	}

	for _, foreground := range uxDailyUseForegrounds(t, unfocusedContent) {
		if !strings.Contains(focusedContent, foreground) {
			t.Fatalf("the band swallowed the %s the row draws when unfocused:\nfocused   %q\nunfocused %q",
				foreground, focusedContent, unfocusedContent)
		}
	}

	mono := uxClarityMonochrome(t, focused)
	if !strings.Contains(mono, "\x1b[7m") {
		t.Fatalf("the cursor row keeps no mark on a terminal without color:\n%q", mono)
	}
	if other := uxClarityMonochrome(t, unfocused); strings.Contains(other, "\x1b[7m") {
		t.Fatalf("a row the cursor left is inverted too, so the mark says nothing:\n%q", other)
	}
}

// uxDailyUseForegroundPattern matches one truecolor foreground parameter run
// wherever it sits in an SGR sequence, since a piece that also arms a background
// spells both in one sequence.
var uxDailyUseForegroundPattern = regexp.MustCompile(`38;2;\d+;\d+;\d+`)

// uxDailyUseForegrounds is the set of foreground colors a run of styled text
// paints, in first-seen order so a failure names them the way the row draws them.
func uxDailyUseForegrounds(t *testing.T, styled string) []string {
	t.Helper()
	var found []string
	seen := map[string]bool{}
	for _, match := range uxDailyUseForegroundPattern.FindAllString(styled, -1) {
		if seen[match] {
			continue
		}
		seen[match] = true
		found = append(found, match)
	}
	if len(found) == 0 {
		t.Fatalf("the row paints no foreground at all, so nothing is being held to the band:\n%q", styled)
	}
	return found
}

// uxDailyUseRowContent is the styled bytes of a task row between its gutter and
// the end of its title: everything the cursor is not supposed to touch, with the
// first column and the trailing fill — the two parts it is — left outside.
func uxDailyUseRowContent(t *testing.T, line, title string) string {
	t.Helper()
	start := strings.Index(line, uxDailyUseRowGutter)
	end := strings.Index(line, title)
	if start < 0 || end < start {
		t.Fatalf("the row carries no state badge before %q:\n%q", title, line)
	}
	return line[start : end+len(title)]
}

// uxDailyUseRowGutter is where the compared run starts: the Backlog state badge
// the alpha row draws first. Every `multi-project` task is its project's only
// one, so its group merges into the row and no tree connector precedes it.
const uxDailyUseRowGutter = "🞑"

// TestUXDailyUseSelectionCountsAreStatedOnce is this contract after session 4
// rewrote it. A visual selection used to be counted in two places at once — the
// context row under the title and the selection line the status channel held
// sticky — and the context row is now the whole of it.
//
// The frame it reads is the one where the counts are load-bearing rather than
// decorative: every selected task folded out of view, so nothing on screen
// carries a checkbox and the summary is the only thing that knows the gesture
// has five targets.
//
// Four claims, and each of them fails on its own. The summary appears exactly
// once on screen, so a second statement anywhere is a finding. The channel that
// used to carry the second one says nothing about a selection, which is what
// keeps it free for the feedback a bulk gesture is about to write there. The
// hidden count survives into both spellings the row can draw, the compact one
// included, because that is the number a bulk gesture must not be surprised by
// and shedding it is exactly what a narrow terminal would otherwise do. And `Q`
// still carries all three numbers, since the visible half is written down
// nowhere else.
func TestUXDailyUseSelectionCountsAreStatedOnce(t *testing.T) {
	for _, size := range uxDailyUseMatrixSizes {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, view := uxDailyUseStagedDriver(t, "hidden-selection", size[0], size[1])
			counts := view.SelectionCounts()
			if counts.Total == 0 || counts.Hidden != counts.Total {
				t.Fatalf("the staged selection is not the hidden one: %+v", counts)
			}

			screen := driver.Observe(ObserveOptions{}).PlainScreen
			if stated := strings.Count(screen, "Multi-select"); stated != 1 {
				t.Fatalf("the selection is stated %d times, want exactly one:\n%s", stated, screen)
			}
			summary := uxDailyUseScreenLine(t, screen, "Multi-select")
			if !strings.Contains(summary, fmt.Sprintf("(%d hidden)", counts.Hidden)) {
				t.Fatalf("the summary drops the hidden half of %+v:\n%s", counts, summary)
			}

			// The context row is above the list, so the row carrying the summary is
			// the one directly under the title rather than anything the shell pins at
			// the bottom.
			rows := strings.Split(screen, "\n")
			if len(rows) < 2 || !strings.Contains(rows[1], "Multi-select") {
				t.Fatalf("the summary is not the context row under the title:\n%s", screen)
			}
			if footer := uxClarityFooter(driver); strings.Contains(footer, "Visual selection") {
				t.Fatalf("the status channel still states the selection:\n%s", footer)
			}
			if view.StatusMessage != "" {
				t.Fatalf("entering a selection wrote the status line: %q", view.StatusMessage)
			}

			// Both spellings are asked for directly, because whether this terminal is
			// narrow enough to compact the row is the renderer's decision and the
			// claim is about the compact form itself.
			entry, ok := uxDailyUseSelectionEntry(view, size[0])
			if !ok {
				t.Fatal("the live axes carry no selection entry")
			}
			hidden := fmt.Sprintf("(%d hidden)", counts.Hidden)
			if !strings.Contains(ansi.Strip(entry.Row), hidden) ||
				!strings.Contains(ansi.Strip(entry.RowCompact), hidden) {
				t.Fatalf("a spelling sheds the hidden count: full %q compact %q",
					ansi.Strip(entry.Row), ansi.Strip(entry.RowCompact))
			}

			uxClarityPress(t, driver, uxClarityKeys("Q")...)
			readout := driver.Observe(ObserveOptions{}).PlainScreen
			breakdown := fmt.Sprintf("%d total · %d visible · %d hidden", counts.Total, counts.Visible, counts.Hidden)
			if !strings.Contains(readout, breakdown) {
				t.Fatalf("the readout does not carry %q:\n%s", breakdown, readout)
			}
		})
	}
}

// uxDailyUseSelectionEntry is the selection axis as the context row and the
// readout both read it, taken from the model's own description rather than
// recovered from a rendered row.
func uxDailyUseSelectionEntry(view *outliner.OutlinerModel, width int) (outliner.ViewContextEntry, bool) {
	for _, entry := range view.BuildViewContextEntries(width) {
		if entry.Label == "selection" {
			return entry, true
		}
	}
	return outliner.ViewContextEntry{}, false
}
