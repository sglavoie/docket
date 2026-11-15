// Package sigil is the typed form of a row: the line a person edits, and the
// fields it stands for. Fields are the truth; a line is parsed into them when
// an edit ends and rendered from them when one begins.
package sigil

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Field names a Line maps to.
const (
	FieldText    = "text"
	FieldState   = "state"
	FieldFlag    = "flag"
	FieldDue     = "due"
	FieldTags    = "tags"
	FieldChanged = "changed" // when the state last changed, unix milliseconds
	FieldToday   = "today"   // the day the row was picked for Today
)

// States. The empty state is a note or a heading.
const (
	Open     = "open"
	Doing    = "doing"
	Waiting  = "waiting"
	Done     = "done"
	Canceled = "canceled"
	Parked   = "parked" // not now: kept, and out of every list
)

// Line is a row's typed content.
type Line struct {
	Text  string
	State string
	Flag  bool
	Due   string // YYYY-MM-DD, a hard deadline
	Tags  []string
}

// Closed reports a state that needs nothing further.
func (l Line) Closed() bool { return l.State == Done || l.State == Canceled }

// DefaultState is the state a row has when its line names none: a top-level
// row is a project and a row ending in a colon is a heading, so both are
// notes; every other row is an open task.
func DefaultState(top bool, text string) string {
	if top || strings.HasSuffix(text, ":") {
		return ""
	}
	return Open
}

var stateNames = []string{Open, Doing, Waiting, Done, Canceled, Parked, "note"}

var stateAliases = map[string]string{"todo": Open, "wait": Waiting, "cancelled": Canceled}

// ParseState accepts a state name, an alias, or an unambiguous prefix.
func ParseState(s string) (string, bool) {
	s = strings.ToLower(s)
	if s == "" {
		return "", false
	}
	found, n := "", 0
	if a, ok := stateAliases[s]; ok {
		found, n = a, 1
	} else {
		for _, name := range stateNames {
			if strings.HasPrefix(name, s) {
				found, n = name, n+1
			}
		}
	}
	if n != 1 {
		return "", false
	}
	if found == "note" {
		found = ""
	}
	return found, true
}

// ParseTag accepts a word starting with a letter, so #123 stays prose.
func ParseTag(s string) (string, bool) {
	for i, r := range s {
		switch {
		case unicode.IsLetter(r):
		case i > 0 && (unicode.IsDigit(r) || r == '-' || r == '_'):
		default:
			return "", false
		}
	}
	return strings.ToLower(s), s != ""
}

func isSigil(w string, today time.Time) bool {
	if w == "!" {
		return true
	}
	if len(w) < 2 {
		return false
	}
	ok := false
	switch w[0] {
	case '#':
		_, ok = ParseTag(w[1:])
	case '@':
		_, ok = ParseDate(w[1:], today)
	case '$':
		_, ok = ParseState(w[1:])
	}
	return ok
}

// Parse reads a typed line. A sigil is a whole word; one the grammar cannot
// read stays in the text, and a backslash before a sigil keeps it literal.
func Parse(s string, top bool, today time.Time) Line {
	var l Line
	var words []string
	state, named := "", false
	for _, w := range strings.Fields(s) {
		if rest, ok := strings.CutPrefix(w, `\`); ok && isSigil(rest, today) {
			words = append(words, rest)
			continue
		}
		if !isSigil(w, today) {
			words = append(words, w)
			continue
		}
		switch w[0] {
		case '!':
			l.Flag = true
		case '#':
			if tag, _ := ParseTag(w[1:]); !slices.Contains(l.Tags, tag) {
				l.Tags = append(l.Tags, tag)
			}
		case '@':
			l.Due, _ = ParseDate(w[1:], today)
		case '$':
			state, _ = ParseState(w[1:])
			named = true
		}
	}
	l.Text = strings.Join(words, " ")
	l.State = DefaultState(top, l.Text)
	if named {
		l.State = state
	}
	return l
}

// Format renders the line Parse reads back to the same value. The state is
// spelled only where it differs from the row's default.
func Format(l Line, top bool, today time.Time) string {
	var words []string
	for _, w := range strings.Fields(l.Text) {
		if isSigil(w, today) {
			w = `\` + w
		}
		words = append(words, w)
	}
	if l.Flag {
		words = append(words, "!")
	}
	if l.Due != "" {
		words = append(words, "@"+l.Due)
	}
	for _, tag := range l.Tags {
		words = append(words, "#"+tag)
	}
	if l.State != DefaultState(top, l.Text) {
		if l.State == "" {
			words = append(words, "$note")
		} else {
			words = append(words, "$"+l.State)
		}
	}
	return strings.Join(words, " ")
}

// FromFields reads a row's stored fields.
func FromFields(f map[string]string) Line {
	return Line{
		Text:  f[FieldText],
		State: f[FieldState],
		Flag:  f[FieldFlag] != "",
		Due:   f[FieldDue],
		Tags:  strings.Fields(f[FieldTags]),
	}
}

// Fields is the stored form, with the empty string for every unset field.
func (l Line) Fields() map[string]string {
	flag := ""
	if l.Flag {
		flag = "1"
	}
	return map[string]string{
		FieldText:  l.Text,
		FieldState: l.State,
		FieldFlag:  flag,
		FieldDue:   l.Due,
		FieldTags:  strings.Join(l.Tags, " "),
	}
}

const isoDay = "2006-01-02"

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// civil is today's calendar day, at midnight UTC so day arithmetic never
// meets a daylight-saving change.
func civil(today time.Time) time.Time {
	return time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseDate reads a deadline: today, tomorrow, a weekday (the next one, never
// today), +3d, +2w, MM-DD (the next such day), or YYYY-MM-DD.
func ParseDate(s string, today time.Time) (string, bool) {
	s = strings.ToLower(s)
	day := civil(today)
	switch s {
	case "today", "tod":
		return day.Format(isoDay), true
	case "tomorrow", "tom":
		return day.AddDate(0, 0, 1).Format(isoDay), true
	}
	if wd, ok := weekdays[s]; ok {
		n := (int(wd) - int(day.Weekday()) + 7) % 7
		if n == 0 {
			n = 7
		}
		return day.AddDate(0, 0, n).Format(isoDay), true
	}
	if len(s) > 2 && s[0] == '+' {
		n, err := strconv.Atoi(s[1 : len(s)-1])
		if err != nil || n < 0 || n > 3650 {
			return "", false
		}
		switch s[len(s)-1] {
		case 'd':
			return day.AddDate(0, 0, n).Format(isoDay), true
		case 'w':
			return day.AddDate(0, 0, 7*n).Format(isoDay), true
		}
		return "", false
	}
	if t, err := time.Parse(isoDay, s); err == nil {
		return t.Format(isoDay), true
	}
	if t, err := time.Parse("01-02", s); err == nil {
		// The next such day; a leap day waits for a leap year.
		for year := day.Year(); year < day.Year()+9; year++ {
			d := time.Date(year, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			if d.Month() == t.Month() && !d.Before(day) {
				return d.Format(isoDay), true
			}
		}
	}
	return "", false
}

// DueLabel is how a deadline reads on a row, and how many days away it is.
func DueLabel(due string, today time.Time) (string, int) {
	t, err := time.Parse(isoDay, due)
	if err != nil {
		return "@" + due, 0
	}
	day := civil(today)
	days := int(t.Sub(day).Hours() / 24)
	switch {
	case days == 0:
		return "@today", days
	case days == 1:
		return "@tomorrow", days
	case days > 1 && days < 7:
		return "@" + strings.ToLower(t.Weekday().String()[:3]), days
	case t.Year() == day.Year():
		return fmt.Sprintf("@%s %d", t.Month().String()[:3], t.Day()), days
	}
	return "@" + due, days
}
