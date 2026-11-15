package sigil

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// A Sunday.
var today = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

func TestParse(t *testing.T) {
	cases := []struct {
		line  string
		notes bool
		want  Line
	}{
		{"Fix the flaky test", false, Line{Text: "Fix the flaky test", State: Open}},
		{"docket-rewrite", true, Line{Text: "docket-rewrite"}},
		{"Detail under a task", true, Line{Text: "Detail under a task"}},
		{"Subtask $open", true, Line{Text: "Subtask", State: Open}},
		{"Subheading:", true, Line{Text: "Subheading:"}},
		{"Last state wins $done $open", true, Line{Text: "Last state wins", State: Open}},
		{"Storage:", false, Line{Text: "Storage:"}},
		{"Ship it ! @fri #Release #release $doing", false,
			Line{Text: "Ship it", State: Doing, Flag: true, Due: "2026-11-20", Tags: []string{"release"}}},
		{"! Ship @tom it", false, Line{Text: "Ship it", State: Open, Flag: true, Due: "2026-11-16"}},
		{"just a thought $note", false, Line{Text: "just a thought"}},
		{"Top-level task $open", true, Line{Text: "Top-level task", State: Open}},
		{"Call back $w", false, Line{Text: "Call back", State: Waiting}},
		{"old idea $p", true, Line{Text: "old idea", State: Parked}},
		{"Pay rent @12-01 *monthly @@11-28", false,
			Line{Text: "Pay rent", State: Open, Due: "2026-12-01", Start: "2026-11-28", Every: "1m"}},
		{"*emphasis* and *2x and 3 * 4 and @@nope", false, Line{Text: "*emphasis* and *2x and 3 * 4 and @@nope", State: Open}},
		// What the grammar cannot read stays prose.
		{"Fix #123 for @alice at R&D, costs $5 $d wow!", false,
			Line{Text: "Fix #123 for @alice at R&D, costs $5 $d wow!", State: Open}},
		{`Document the \#tag and \@fri sigils \!`, false,
			Line{Text: "Document the #tag and @fri sigils !", State: Open}},
		{`a \plain backslash`, false, Line{Text: `a \plain backslash`, State: Open}},
		{"later wins @mon @tue $done $open", false, Line{Text: "later wins", State: Open, Due: "2026-11-17"}},
	}
	for _, c := range cases {
		if got := Parse(c.line, c.notes, today); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q, notes=%v)\n got %+v\nwant %+v", c.line, c.notes, got, c.want)
		}
	}
}

func TestFormatRoundTrips(t *testing.T) {
	cases := []struct {
		line  Line
		notes bool
		want  string
	}{
		{Line{Text: "Fix it", State: Open}, false, "Fix it"},
		{Line{Text: "Fix it", State: Open}, true, "Fix it $open"},
		{Line{Text: "Storage:"}, false, "Storage:"},
		{Line{Text: "a thought"}, false, "a thought $note"},
		{Line{Text: "project"}, true, "project"},
		{Line{Text: "detail under a task"}, true, "detail under a task"},
		{Line{Text: "Subheading:", State: Open}, true, "Subheading: $open"},
		{Line{Text: "Ship", State: Done, Flag: true, Due: "2026-11-20", Tags: []string{"a", "b"}}, false,
			"Ship ! @2026-11-20 #a #b $done"},
		{Line{Text: "the #tag and @fri sigils !", State: Open}, false, `the \#tag and \@fri sigils \!`},
		{Line{Text: "Water plants", State: Open, Start: "2026-11-20", Every: "3d", Tags: []string{"home"}}, false,
			"Water plants @@2026-11-20 *3d #home"},
		{Line{Text: "do it *2w or @@fri", State: Open}, false, `do it \*2w or \@@fri`},
	}
	for _, c := range cases {
		got := Format(c.line, c.notes, today)
		if got != c.want {
			t.Errorf("Format(%+v, notes=%v) = %q, want %q", c.line, c.notes, got, c.want)
		}
		if back := Parse(got, c.notes, today); !reflect.DeepEqual(back, c.line) {
			t.Errorf("Parse(Format(%+v)) = %+v", c.line, back)
		}
	}
}

func TestFieldsRoundTrip(t *testing.T) {
	l := Line{Text: "Ship", State: Waiting, Flag: true, Due: "2026-11-20", Tags: []string{"a", "b"}}
	if got := FromFields(l.Fields()); !reflect.DeepEqual(got, l) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseDate(t *testing.T) {
	cases := map[string]string{
		"today":      "2026-11-15",
		"TOM":        "2026-11-16",
		"sun":        "2026-11-22", // never today
		"mon":        "2026-11-16",
		"saturday":   "2026-11-21",
		"+0d":        "2026-11-15",
		"+3d":        "2026-11-18",
		"+2w":        "2026-11-29",
		"11-15":      "2026-11-15",
		"11-14":      "2027-11-14",
		"02-29":      "2028-02-29",
		"2027-01-05": "2027-01-05",
	}
	for in, want := range cases {
		if got, ok := ParseDate(in, today); !ok || got != want {
			t.Errorf("ParseDate(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "alice", "+3", "+d", "+3m", "13-40", "2026-02-30", "5"} {
		if got, ok := ParseDate(in, today); ok {
			t.Errorf("ParseDate(%q) = %q, want a refusal", in, got)
		}
	}
}

func TestDueLabel(t *testing.T) {
	cases := []struct {
		due   string
		label string
		days  int
	}{
		{"2026-11-15", "@today", 0},
		{"2026-11-16", "@tomorrow", 1},
		{"2026-11-20", "@fri", 5},
		{"2026-11-22", "@Nov 22", 7},
		{"2026-11-12", "@Nov 12", -3},
		{"2027-01-05", "@2027-01-05", 51},
	}
	for _, c := range cases {
		if label, days := DueLabel(c.due, today); label != c.label || days != c.days {
			t.Errorf("DueLabel(%s) = %q, %d; want %q, %d", c.due, label, days, c.label, c.days)
		}
	}
}

func TestParseRepeat(t *testing.T) {
	for in, want := range map[string]string{"1d": "1d", "02W": "2w", "3m": "3m", "1y": "1y", "daily": "1d", "Weekly": "1w", "monthly": "1m", "yearly": "1y"} {
		if got, ok := ParseRepeat(in); !ok || got != want {
			t.Errorf("ParseRepeat(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "d", "0d", "2x", "+2d", "-1w", "1000d", "emphasis*", "2"} {
		if got, ok := ParseRepeat(in); ok {
			t.Errorf("ParseRepeat(%q) = %q, want a refusal", in, got)
		}
	}
}

func TestNextDate(t *testing.T) {
	cases := []struct{ from, every, want string }{
		{"", "3d", "2026-11-18"},           // nothing to step from: once from today
		{"2026-11-20", "1w", "2026-11-27"}, // a date ahead still moves on
		{"2026-11-15", "1d", "2026-11-16"},
		{"2026-10-01", "1w", "2026-11-19"}, // missed weeks are skipped, the weekday kept
		{"2026-01-31", "1m", "2026-11-30"}, // the 31st in a 30-day month
		{"2026-10-31", "2m", "2026-12-31"},
		{"2024-02-29", "1y", "2027-02-28"},
	}
	for _, c := range cases {
		if got := NextDate(c.from, c.every, today); got != c.want {
			t.Errorf("NextDate(%q, %s) = %s, want %s", c.from, c.every, got, c.want)
		}
	}
}

func TestLiteralBackslashesBeforeSigils(t *testing.T) {
	for _, notes := range []bool{false, true} {
		for _, token := range []string{"$open", "!", "@2026-11-15", "@@2026-11-15", "*2w", "#tag"} {
			for n := 0; n <= 2; n++ {
				for _, prefix := range []string{"", "prefix "} {
					line := Line{Text: prefix + strings.Repeat(`\`, n) + token,
						State: Doing, Flag: true, Due: "2026-11-15", Start: "2026-11-15",
						Every: "2w", Tags: []string{"real"}}
					want := prefix + strings.Repeat(`\`, n+1) + token + " ! @2026-11-15 @@2026-11-15 *2w #real $doing"
					if got := Format(line, notes, today); got != want {
						t.Fatalf("Format(%q) = %q, want %q", line.Text, got, want)
					}
					if got := Parse(want, notes, today); !reflect.DeepEqual(got, line) {
						t.Fatalf("Parse(%q) = %+v, want %+v", want, got, line)
					}
				}
			}
		}
		for _, text := range []string{`\plain`, `\\plain`, `prefix \#123`, `prefix \\@alice`} {
			line := Line{Text: text, State: DefaultState(notes, text)}
			if got := Format(line, notes, today); got != text {
				t.Fatalf("non-sigil changed: %q -> %q", text, got)
			}
			if got := Parse(text, notes, today); !reflect.DeepEqual(got, line) {
				t.Fatalf("non-sigil parsed as %+v, want %+v", got, line)
			}
		}
	}
}
