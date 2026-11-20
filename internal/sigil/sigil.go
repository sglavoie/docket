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
	FieldPin     = "pin" // drawn before its unpinned siblings
	FieldDue     = "due"
	FieldTags    = "tags"
	FieldChanged = "changed" // when the state last changed, unix milliseconds
	FieldToday   = "today"   // the day the row was picked for Today
	FieldStart   = "start"   // not before this day
	FieldRepeat  = "repeat"  // the interval a closed row comes back after
	FieldColor   = "color"   // a project's colour: a name or #rrggbb
	FieldContext = "context" // where the work is done, one name: home, office
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
	Text    string
	State   string
	Flag    bool
	Pin     bool
	Due     string // YYYY-MM-DD, a hard deadline
	Start   string // YYYY-MM-DD, not before this day
	Every   string // a count and a unit, as in 2w: the row repeats
	Color   string // a colour name, or #rrggbb in small letters
	Context string // one name, in small letters, as a tag without a value
	Tags    []string
}

// Later reports work that is not to be started yet.
func (l Line) Later(today time.Time) bool {
	return l.Start > civil(today).Format(isoDay) && !l.Closed()
}

// FollowsUp reports a follow-up: a waiting row that repeats, with a start
// day and no deadline. Its start day steps on by the interval while it
// waits, bringing it back to mind until what it waits for comes, and closing
// it closes it for good.
func (l Line) FollowsUp() bool {
	return l.State == Waiting && l.Every != "" && l.Start != "" && l.Due == ""
}

// StartOn is the start day as it stands today. A follow-up's start, once
// reached, steps on by its interval, so it is the first such day not before
// today; every other row's start is the day it names. Nothing is written as
// the days pass: the stored start stays the first.
func (l Line) StartOn(today time.Time) string {
	day := civil(today).Format(isoDay)
	if !l.FollowsUp() || l.Start >= day {
		return l.Start
	}
	return NextDate(l.Start, l.Every, civil(today).AddDate(0, 0, -1))
}

// Closed reports a state that needs nothing further.
func (l Line) Closed() bool { return l.State == Done || l.State == Canceled }

// DefaultState is the state a row has when its line names none. A place that
// defaults to notes (top-level), or text ending in a colon, makes a note;
// every other row is an open task.
func DefaultState(notes bool, text string) string {
	if notes || strings.HasSuffix(text, ":") {
		return ""
	}
	return Open
}

// StateNames are the words a $ sigil accepts, in the order they are offered.
var StateNames = []string{Open, Doing, Waiting, Done, Canceled, Parked, "note"}

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
		for _, name := range StateNames {
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

// ParseTag accepts a word starting with a letter, so #123 stays prose. A tag
// may carry a value after an equals sign, as in est=2h: the name is stored
// lowercase and the value as typed.
func ParseTag(s string) (string, bool) {
	name, value, valued := strings.Cut(s, "=")
	for i, r := range name {
		switch {
		case unicode.IsLetter(r):
		case i > 0 && (unicode.IsDigit(r) || r == '-' || r == '_'):
		default:
			return "", false
		}
	}
	if name == "" || (valued && value == "") {
		return "", false
	}
	if valued {
		return strings.ToLower(name) + "=" + value, true
	}
	return strings.ToLower(name), true
}

// ParseContext accepts a context: a tag's name, with no value.
func ParseContext(s string) (string, bool) {
	name, ok := ParseTag(s)
	if !ok || strings.Contains(name, "=") {
		return "", false
	}
	return name, true
}

// TagName is a tag without its value.
func TagName(tag string) string {
	name, _, _ := strings.Cut(tag, "=")
	return name
}

// TagValue is the value a tag carries, or nothing.
func TagValue(tag string) string {
	_, value, _ := strings.Cut(tag, "=")
	return value
}

// PutTag adds a tag to a list. A name is held once: a tag already there
// under the same name is replaced where it sits, so its value changes.
func PutTag(tags []string, tag string) []string {
	for i, have := range tags {
		if TagName(have) == TagName(tag) {
			tags[i] = tag
			return tags
		}
	}
	return append(tags, tag)
}

// IsSigil reports a word the row grammar reads as a sigil.
func IsSigil(w string, today time.Time) bool {
	if w == "!" || w == "^" {
		return true
	}
	if len(w) < 2 {
		return false
	}
	ok := false
	switch w[0] {
	case '#':
		if rest, ctx := strings.CutPrefix(w, "##"); ctx {
			_, ok = ParseContext(rest)
			break
		}
		_, ok = ParseTag(w[1:])
	case '*':
		_, ok = ParseRepeat(w[1:])
	case '@':
		if rest, start := strings.CutPrefix(w, "@@"); start {
			_, ok = ParseDate(rest, today)
			break
		}
		_, ok = ParseDate(w[1:], today)
	case '$':
		_, ok = ParseState(w[1:])
	case '~':
		_, ok = ParseColor(w[1:])
	}
	return ok
}

// NoteMark is the word that, leading a line, makes the row a note.
const NoteMark = "//"

// Parse reads a typed line. A sigil is a whole word; one the grammar cannot
// read stays in the text. A line led by NoteMark is a note. Before a
// recognized sigil, or a leading NoteMark, a run of backslashes keeps the
// token literal and loses exactly one backslash.
func Parse(s string, notes bool, today time.Time) Line {
	var l Line
	var words []string
	state, named := "", false
	for i, w := range strings.Fields(s) {
		if i == 0 && w == NoteMark {
			state, named = "", true
			continue
		}
		if len(words) == 0 && strings.HasPrefix(w, `\`) && strings.TrimLeft(w, `\`) == NoteMark {
			words = append(words, w[1:])
			continue
		}
		if strings.HasPrefix(w, `\`) && IsSigil(strings.TrimLeft(w, `\`), today) {
			words = append(words, w[1:])
			continue
		}
		if !IsSigil(w, today) {
			words = append(words, w)
			continue
		}
		switch w[0] {
		case '!':
			l.Flag = true
		case '^':
			l.Pin = true
		case '#':
			if rest, ctx := strings.CutPrefix(w, "##"); ctx {
				l.Context, _ = ParseContext(rest)
				break
			}
			tag, _ := ParseTag(w[1:])
			l.Tags = PutTag(l.Tags, tag)
		case '*':
			l.Every, _ = ParseRepeat(w[1:])
		case '@':
			if rest, start := strings.CutPrefix(w, "@@"); start {
				l.Start, _ = ParseDate(rest, today)
			} else {
				l.Due, _ = ParseDate(w[1:], today)
			}
		case '$':
			state, _ = ParseState(w[1:])
			named = true
		case '~':
			l.Color, _ = ParseColor(w[1:])
		}
	}
	l.Text = strings.Join(words, " ")
	l.State = DefaultState(notes, l.Text)
	if named {
		l.State = state
	}
	return l
}

// Format renders the line Parse reads back to the same value. The state is
// spelled only where it differs from the row's default, a note by leading
// the line with NoteMark.
func Format(l Line, notes bool, today time.Time) string {
	var words []string
	for i, w := range strings.Fields(l.Text) {
		if IsSigil(strings.TrimLeft(w, `\`), today) || i == 0 && strings.TrimLeft(w, `\`) == NoteMark {
			w = `\` + w
		}
		words = append(words, w)
	}
	if l.Flag {
		words = append(words, "!")
	}
	if l.Pin {
		words = append(words, "^")
	}
	if l.Due != "" {
		words = append(words, "@"+l.Due)
	}
	if l.Start != "" {
		words = append(words, "@@"+l.Start)
	}
	if l.Every != "" {
		words = append(words, "*"+l.Every)
	}
	for _, tag := range l.Tags {
		words = append(words, "#"+tag)
	}
	if l.Context != "" {
		words = append(words, "##"+l.Context)
	}
	if l.Color != "" {
		words = append(words, "~"+l.Color)
	}
	if l.State != DefaultState(notes, l.Text) {
		if l.State == "" {
			words = append([]string{NoteMark}, words...)
		} else {
			words = append(words, "$"+l.State)
		}
	}
	return strings.Join(words, " ")
}

// FromFields reads a row's stored fields.
func FromFields(f map[string]string) Line {
	return Line{
		Text:    f[FieldText],
		State:   f[FieldState],
		Flag:    f[FieldFlag] != "",
		Pin:     f[FieldPin] != "",
		Due:     f[FieldDue],
		Start:   f[FieldStart],
		Every:   f[FieldRepeat],
		Color:   f[FieldColor],
		Context: f[FieldContext],
		Tags:    strings.Fields(f[FieldTags]),
	}
}

// Fields is the stored form, with the empty string for every unset field.
func (l Line) Fields() map[string]string {
	flag, pin := "", ""
	if l.Flag {
		flag = "1"
	}
	if l.Pin {
		pin = "1"
	}
	return map[string]string{
		FieldText:    l.Text,
		FieldState:   l.State,
		FieldFlag:    flag,
		FieldPin:     pin,
		FieldDue:     l.Due,
		FieldStart:   l.Start,
		FieldRepeat:  l.Every,
		FieldColor:   l.Color,
		FieldContext: l.Context,
		FieldTags:    strings.Join(l.Tags, " "),
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
// today), +3d, +2w, MM-DD (the next such day), or YYYY-MM-DD. It also reads a
// day as the row shows it, by the month's name: dec4 or dec-4 (the next
// such day), jan5-2027 in a named year, eow, the end of this week (Sunday), and eom,
// the last day of this month.
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
	if s == "eow" {
		// The end of this week: Sunday, today when it is one.
		return day.AddDate(0, 0, (7-int(day.Weekday()))%7).Format(isoDay), true
	}
	if s == "eom" {
		// The last day of this month.
		return time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format(isoDay), true
	}
	if d, ok := monthDay(s, day); ok {
		return d, true
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
	case t.Year() != day.Year():
		return fmt.Sprintf("@%s %d %d", t.Month().String()[:3], t.Day(), t.Year()), days
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

var repeatWords = map[string]string{"daily": "1d", "weekly": "1w", "monthly": "1m", "yearly": "1y"}

// ParseRepeat reads an interval: a count and d, w, m or y, or one of daily,
// weekly, monthly, yearly.
func ParseRepeat(s string) (string, bool) {
	s = strings.ToLower(s)
	if every, ok := repeatWords[s]; ok {
		return every, true
	}
	if len(s) < 2 || !strings.ContainsRune("dwmy", rune(s[len(s)-1])) {
		return "", false
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 1 || n > 999 || s[0] == '+' {
		return "", false
	}
	return strconv.Itoa(n) + s[len(s)-1:], true
}

// ColorNames are the words a ~ sigil accepts, in the order they are offered.
// Each is drawn in a shade of its own in either palette, so a name reads on
// a dark and a light background alike; a hex value is drawn as it is.
var ColorNames = []string{
	"red", "orange", "yellow", "lime", "green", "teal", "cyan",
	"blue", "indigo", "purple", "magenta", "pink", "brown", "gray",
}

var colorAliases = map[string]string{"grey": "gray", "violet": "purple"}

// ParseColor reads a colour: one of ColorNames or an alias, or a hex value
// as #rgb or #rrggbb, kept as #rrggbb in small letters.
func ParseColor(s string) (string, bool) {
	s = strings.ToLower(s)
	if a, ok := colorAliases[s]; ok {
		return a, true
	}
	if slices.Contains(ColorNames, s) {
		return s, true
	}
	hex, ok := strings.CutPrefix(s, "#")
	if !ok || (len(hex) != 3 && len(hex) != 6) {
		return "", false
	}
	for _, r := range hex {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return "", false
		}
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	return "#" + hex, true
}

// NextDate is the first day after today reached by stepping from a day by
// an interval. With no day to step from, it steps once from today. A month
// step that overshoots a short month lands on its last day.
func NextDate(from, every string, today time.Time) string {
	day := civil(today)
	at := day
	if t, err := time.Parse(isoDay, from); err == nil {
		at = t
	}
	n, _ := strconv.Atoi(every[:len(every)-1])
	step := func(t time.Time, k int) time.Time {
		switch every[len(every)-1] {
		case 'd':
			return t.AddDate(0, 0, k*n)
		case 'w':
			return t.AddDate(0, 0, 7*k*n)
		case 'y':
			k *= 12
		}
		first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, k*n, 0)
		last := first.AddDate(0, 1, -1).Day()
		return first.AddDate(0, 0, min(t.Day(), last)-1)
	}
	if n < 1 {
		return from
	}
	// Steps are counted from the original day, so the 31st stays the 31st
	// in the months that have one.
	for k := 1; ; k++ {
		if next := step(at, k); next.After(day) {
			return next.Format(isoDay)
		}
	}
}

// ParseShift reads how far a stamp moves a day a row already has: ++3d or
// --2w, a count and d, w, m or y. The count is negative for an earlier day.
func ParseShift(s string) (n int, unit byte, ok bool) {
	if len(s) < 4 || (s[:2] != "++" && s[:2] != "--") {
		return 0, 0, false
	}
	unit = s[len(s)-1] | 0x20
	n, err := strconv.Atoi(s[2 : len(s)-1])
	if err != nil || n < 1 || n > 3650 || !strings.ContainsRune("dwmy", rune(unit)) {
		return 0, 0, false
	}
	if s[0] == '-' {
		n = -n
	}
	return n, unit, true
}

// ShiftDate moves a day by a count of days, weeks, months or years. A month
// step keeps the day, landing on the last day of a shorter month. A day it
// cannot read is returned as it is.
func ShiftDate(day string, n int, unit byte) string {
	t, err := time.Parse(isoDay, day)
	if err != nil {
		return day
	}
	switch unit {
	case 'd':
		return t.AddDate(0, 0, n).Format(isoDay)
	case 'w':
		return t.AddDate(0, 0, 7*n).Format(isoDay)
	case 'y':
		n *= 12
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, n, 0)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(t.Day(), last)-1).Format(isoDay)
}

var months = map[string]time.Month{
	"jan": time.January, "january": time.January,
	"feb": time.February, "february": time.February,
	"mar": time.March, "march": time.March,
	"apr": time.April, "april": time.April,
	"may": time.May,
	"jun": time.June, "june": time.June,
	"jul": time.July, "july": time.July,
	"aug": time.August, "august": time.August,
	"sep": time.September, "sept": time.September, "september": time.September,
	"oct": time.October, "october": time.October,
	"nov": time.November, "november": time.November,
	"dec": time.December, "december": time.December,
}

// monthDay reads a month's name and a day, as dec4 or dec-4, which is the
// next such day from today, or with a year after it, as jan5-2027.
func monthDay(s string, day time.Time) (string, bool) {
	i := strings.IndexFunc(s, func(r rune) bool { return r < 'a' || r > 'z' })
	if i <= 0 {
		return "", false
	}
	month, ok := months[s[:i]]
	if !ok {
		return "", false
	}
	rest := strings.TrimPrefix(s[i:], "-")
	year := 0
	if d, y, found := strings.Cut(rest, "-"); found {
		n, err := strconv.Atoi(y)
		if err != nil || len(y) != 4 {
			return "", false
		}
		rest, year = d, n
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 1 || n > 31 || len(rest) > 2 {
		return "", false
	}
	if year != 0 {
		d := time.Date(year, month, n, 0, 0, 0, 0, time.UTC)
		if d.Month() != month {
			return "", false
		}
		return d.Format(isoDay), true
	}
	// The next such day; a leap day waits for a leap year.
	for y := day.Year(); y < day.Year()+9; y++ {
		d := time.Date(y, month, n, 0, 0, 0, 0, time.UTC)
		if d.Month() == month && !d.Before(day) {
			return d.Format(isoDay), true
		}
	}
	return "", false
}
