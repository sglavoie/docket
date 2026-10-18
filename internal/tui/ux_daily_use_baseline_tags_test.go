package tui

import (
	"strings"
	"testing"
)

// TestUXDailyUseTagPickerLeadsWithAppliedValues is session 8's rewrite of the
// baseline that recorded the opposite: the tags-and-context picker used to open
// on the fixed semantic vocabulary, none of it applied, with the values the task
// actually carries six rows below it.
//
// It now states the three things that order is worth: the applied values lead,
// a value checked while the picker is open does not jump up to join them, and
// neither does a query rearrange what leads. The second is the freeze — the
// order is computed once, out of the membership the picker opened with, so the
// row a reader is looking at stays where they found it.
func TestUXDailyUseTagPickerLeadsWithAppliedValues(t *testing.T) {
	driver, view := uxDailyUseDriver(t, FixtureUXAcceptance, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("#")...)

	opened := uxDailyUsePickerRows(t, driver)
	applied := uxDailyUsePickerRowsMarked(opened, "[x]")
	unapplied := uxDailyUsePickerRowsMarked(opened, "[ ]")
	if len(applied) == 0 || len(unapplied) == 0 {
		t.Fatalf("the picker shows %d applied and %d unapplied rows; the state under test was not reached",
			len(applied), len(unapplied))
	}
	if last, first := applied[len(applied)-1], unapplied[0]; last.index > first.index {
		t.Fatalf("applied %q is drawn below unapplied %q:\n%s",
			last.value, first.value, uxDailyUsePickerOrder(opened))
	}

	// The jump lands on the SEMANTIC section, which is unapplied by definition
	// here, so what is toggled is a row the freeze has to leave where it is.
	uxClarityPress(t, driver, uxClarityKeys("s")...)
	uxClarityPress(t, driver, uxClarityKeys("space")...)
	toggled := uxDailyUsePickerRows(t, driver)
	if uxDailyUsePickerOrder(toggled) != uxDailyUsePickerOrder(opened) {
		t.Fatalf("a toggle reordered the open picker:\nbefore\n%s\nafter\n%s",
			uxDailyUsePickerOrder(opened), uxDailyUsePickerOrder(toggled))
	}
	checked := uxDailyUsePickerRowsMarked(toggled, "[x]")
	if len(checked) != len(applied)+1 {
		t.Fatalf("the toggle checked %d rows, want the %d applied rows plus one:\n%s",
			len(checked), len(applied), uxDailyUsePickerOrder(toggled))
	}

	// What leads is what the picker opened applied, not what is checked now: the
	// row toggled above is checked and still sits in the vocabulary it was drawn
	// in, which is the freeze, so the query is asked about the promoted values.
	promoted := make(map[string]bool, len(applied))
	for _, row := range applied {
		promoted[row.value] = true
	}
	uxClarityPress(t, driver, uxClarityKeys("/")...)
	uxClarityPress(t, driver, uxClarityKeys("i")...)
	queried := uxDailyUsePickerRows(t, driver)
	matchedPromoted, matchedRest := 0, 0
	for _, row := range queried {
		if !promoted[row.value] {
			matchedRest++
			continue
		}
		matchedPromoted++
		// Ranking reorders the rows a query matched, and a promoted row is not
		// ranked back down into the vocabulary underneath it.
		if matchedRest > 0 {
			t.Fatalf("under a query promoted %q is drawn below an unpromoted row:\n%s",
				row.value, uxDailyUsePickerOrder(queried))
		}
	}
	if matchedPromoted == 0 || matchedRest == 0 {
		t.Fatalf("the query left %d promoted and %d other rows; it narrowed too far to say anything:\n%s",
			matchedPromoted, matchedRest, uxDailyUsePickerOrder(queried))
	}

	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	if cleared := uxDailyUsePickerRows(t, driver); uxDailyUsePickerOrder(cleared) != uxDailyUsePickerOrder(toggled) {
		t.Fatalf("clearing the query rebuilt the order:\nbefore\n%s\nafter\n%s",
			uxDailyUsePickerOrder(toggled), uxDailyUsePickerOrder(cleared))
	}
}

// TestUXDailyUseBulkTagPickerLeadsWithMixedValues is session 8's rewrite of the
// bulk half. It stays a separate contract from the single-task one because the
// two reach the picker through different membership computations: universally
// applied and partially applied values both have to lead, and the difference
// between them has to survive the promotion.
//
// The clear-context row is the counterexample the bulk case needs. Every target
// without a context agrees on it, so it reads as mixed exactly like a value the
// selection partly carries — and it is not a value at all, so it stays in
// CONTEXTS rather than leading a list of what the tasks are tagged with.
//
// Session 8 also owns not changing this anywhere else: the filter vocabularies,
// the project-tag picker, and the dependency picker share this infrastructure
// and keep their own order.
func TestUXDailyUseBulkTagPickerLeadsWithMixedValues(t *testing.T) {
	driver, _ := uxDailyUseStagedDriver(t, "bulk-tag-picker", uxDailyUseDailySize[0], uxDailyUseDailySize[1])

	rows := uxDailyUsePickerRows(t, driver)
	unapplied := uxDailyUsePickerRowsMarked(rows, "[ ]")
	mixed := uxDailyUsePickerRowsMarked(rows, "[-]")
	if len(mixed) == 0 || len(unapplied) == 0 {
		t.Fatalf("the picker shows %d mixed and %d unapplied rows; the state under test was not reached",
			len(mixed), len(unapplied))
	}

	clear := mixed[len(mixed)-1]
	if !strings.HasPrefix(clear.value, "No context") {
		t.Fatalf("the last mixed row is %q, want the clear-context row every untagged target agrees on:\n%s",
			clear.value, uxDailyUsePickerOrder(rows))
	}
	if clear.index != rows[len(rows)-1].index {
		t.Fatalf("the clear-context row was promoted out of CONTEXTS:\n%s", uxDailyUsePickerOrder(rows))
	}
	for _, row := range mixed[:len(mixed)-1] {
		if row.index > unapplied[0].index {
			t.Fatalf("partially applied %q is drawn below unapplied %q:\n%s",
				row.value, unapplied[0].value, uxDailyUsePickerOrder(rows))
		}
	}
}

// uxDailyUsePickerRow is one checkbox row of the overlay: where it is drawn,
// which of the three states it carries, and the text beside it.
type uxDailyUsePickerRow struct {
	index int
	mark  string
	value string
}

// uxDailyUsePickerRows is the picker's checkbox rows in the order they are
// drawn, from one observation, so any two of them are always comparable.
//
// Only what is inside the picker's own box is read. The outline underneath is
// still rendered and, under a visual selection, every one of its task rows draws
// a checkbox of its own in the gutter beside the box — so a scan of whole screen
// lines answers about the selection as often as about the picker.
func uxDailyUsePickerRows(t *testing.T, driver *UITestDriver) []uxDailyUsePickerRow {
	t.Helper()
	rows := make([]uxDailyUsePickerRow, 0, 16)
	for index, line := range strings.Split(driver.Observe(ObserveOptions{}).PlainScreen, "\n") {
		inside, ok := uxDailyUseInsidePickerBox(line)
		if !ok {
			continue
		}
		for _, mark := range []string{"[x]", "[-]", "[ ]"} {
			at := strings.Index(inside, mark+" ")
			if at < 0 {
				continue
			}
			rows = append(rows, uxDailyUsePickerRow{
				index: index,
				mark:  mark,
				value: strings.TrimSpace(inside[at+len(mark):]),
			})
			break
		}
	}
	return rows
}

// uxDailyUsePickerRowsMarked narrows those rows to one checkbox state.
func uxDailyUsePickerRowsMarked(rows []uxDailyUsePickerRow, mark string) []uxDailyUsePickerRow {
	marked := make([]uxDailyUsePickerRow, 0, len(rows))
	for _, row := range rows {
		if row.mark == mark {
			marked = append(marked, row)
		}
	}
	return marked
}

// uxDailyUsePickerOrder is the sequence of values alone, which is what a freeze
// is a claim about: the checkbox beside a row is free to change without the row
// moving, so the marks are deliberately not part of it.
func uxDailyUsePickerOrder(rows []uxDailyUsePickerRow) string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.value)
	}
	return strings.Join(values, "\n")
}

// uxDailyUseInsidePickerBox is the part of a screen line the overlay's own
// borders enclose, and whether the line crosses the box at all.
func uxDailyUseInsidePickerBox(line string) (string, bool) {
	const border = "│"
	open := strings.Index(line, border)
	close := strings.LastIndex(line, border)
	if open < 0 || close <= open {
		return "", false
	}
	return line[open+len(border) : close], true
}
