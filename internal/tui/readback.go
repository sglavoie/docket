package tui

import (
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/sigil"
)

// A filter is terse so that it is quick to type. While it is typed the
// status line reads it back in words, so a line such as -_$open|doing never
// has to be remembered: it says what it asks for.

// dayWords is a day as a row shows it, without its sigil.
func dayWords(day string, today time.Time) string {
	label, _ := sigil.DueLabel(day, today)
	return strings.TrimPrefix(label, "@")
}

// words reads one reading of a term.
func (c cond) words(today time.Time) string {
	switch c.kind {
	case 't':
		if c.loose {
			// Drawn in the warning colour as it is typed; read back, it says
			// why it finds what it does and how the sigil is written.
			return `"` + c.value + `" as text: ` + sigilForm(c.value)
		}
		if c.line {
			return `"` + c.value + `" on the row's line`
		}
		return `"` + c.value + `"`
	case '!':
		return "flagged"
	case '#':
		if c.value == "" {
			return "tagged"
		}
		return "#" + c.value
	case 'c':
		if c.value == "" {
			return "with a context"
		}
		return "##" + c.value
	case 'l':
		return "#" + c.value + " at most " + c.arg
	case 'g':
		return "#" + c.value + " at least " + c.arg
	case '$':
		return c.value
	case 'S':
		return "a task"
	case 'n':
		return "its project's next action"
	case 'f':
		return "first workable row under its parent"
	case 'T':
		return "in Today"
	case 'M':
		return "carried into Today from an earlier day"
	case 'P':
		return "a project"
	case '^':
		return "pinned"
	case 'B':
		return "waiting on a linked task still to do"
	case 'U':
		return "a task with a deadline and no check written"
	case '@':
		switch c.value {
		case "":
			return "has a deadline"
		case "overdue":
			return "overdue"
		case "week":
			return "due within a week"
		}
		return "due " + dayWords(c.value, today)
	case '<':
		return "due by " + dayWords(c.value, today)
	case '>':
		return "due from " + dayWords(c.value, today)
	case 's':
		switch c.value {
		case "":
			return "has a start day"
		case "later":
			return "starts later"
		}
		return "starts " + dayWords(c.value, today)
	case '*':
		if c.value == "" {
			return "repeats"
		}
		return "repeats every " + c.value
	case '~':
		return "changed since " + dayWords(c.value, today)
	case 'e':
		return "edited since " + dayWords(c.value, today)
	case '&':
		switch c.value {
		case "":
			return "links to a row"
		case ".":
			return "links to this row"
		}
		return `links to "` + c.value + `"`
	}
	return ""
}

// words reads a term: its readings, on the row or on the rows around it.
func (t term) words(today time.Time) string {
	var any []string
	for _, c := range t.any {
		any = append(any, c.words(today))
	}
	what := strings.Join(any, " or ")
	if len(any) == 1 && t.any[0].kind == 'S' && t.neg && !t.above && !t.below && !t.parent && !t.child {
		return "a note or heading"
	}
	where := ""
	switch {
	case t.above && t.neg:
		where = "no row above it: "
	case t.above:
		where = "a row above it: "
	case t.below && t.neg:
		where = "no row under it: "
	case t.below:
		where = "a row under it: "
	case t.parent && t.neg:
		where = "its parent not: "
	case t.parent:
		where = "its parent: "
	case t.child && t.neg:
		where = "no row directly under it: "
	case t.child:
		where = "a row directly under it: "
	case t.neg:
		where = "not "
		if len(any) > 1 {
			what = "(" + what + ")"
		}
	}
	return where + what
}

// words reads a group of terms: runs that must all hold, any one of which
// will do.
func (g *group) words(terms []term, today time.Time, top bool) string {
	if g.alts == nil {
		return terms[g.leaf].words(today)
	}
	var runs []string
	for _, run := range g.alts {
		var parts []string
		for i := range run {
			parts = append(parts, run[i].words(terms, today, false))
		}
		runs = append(runs, strings.Join(parts, " and "))
	}
	out := strings.Join(runs, ", or ")
	switch {
	case g.neg:
		return "not (" + out + ")"
	case !top && (len(runs) > 1 || len(g.alts[0]) > 1):
		return "(" + out + ")"
	}
	return out
}

// readBack says in words what a filter asks of a row. It is empty while the
// line holds nothing readable yet.
func readBack(q string, today time.Time) string {
	parsed := parseFilter(q, today)
	if len(parsed.terms) == 0 {
		return ""
	}
	if parsed.expr != nil {
		return parsed.expr.words(parsed.terms, today, true)
	}
	var parts []string
	for _, t := range parsed.terms {
		parts = append(parts, t.words(today))
	}
	return strings.Join(parts, " and ")
}
