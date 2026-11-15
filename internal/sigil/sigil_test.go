package sigil

import (
	"reflect"
	"testing"
	"time"
)

// A Sunday.
var today = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

func TestParse(t *testing.T) {
	cases := []struct {
		line string
		top  bool
		want Line
	}{
		{"Fix the flaky test", false, Line{Text: "Fix the flaky test", State: Open}},
		{"docket-rewrite", true, Line{Text: "docket-rewrite"}},
		{"Storage:", false, Line{Text: "Storage:"}},
		{"Ship it ! @fri #Release #release $doing", false,
			Line{Text: "Ship it", State: Doing, Flag: true, Due: "2026-11-20", Tags: []string{"release"}}},
		{"! Ship @tom it", false, Line{Text: "Ship it", State: Open, Flag: true, Due: "2026-11-16"}},
		{"just a thought $note", false, Line{Text: "just a thought"}},
		{"Top-level task $open", true, Line{Text: "Top-level task", State: Open}},
		{"Call back $w", false, Line{Text: "Call back", State: Waiting}},
		{"old idea $p", true, Line{Text: "old idea", State: Parked}},
		// What the grammar cannot read stays prose.
		{"Fix #123 for @alice at R&D, costs $5 $d wow!", false,
			Line{Text: "Fix #123 for @alice at R&D, costs $5 $d wow!", State: Open}},
		{`Document the \#tag and \@fri sigils \!`, false,
			Line{Text: "Document the #tag and @fri sigils !", State: Open}},
		{`a \plain backslash`, false, Line{Text: `a \plain backslash`, State: Open}},
		{"later wins @mon @tue $done $open", false, Line{Text: "later wins", State: Open, Due: "2026-11-17"}},
	}
	for _, c := range cases {
		if got := Parse(c.line, c.top, today); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q, top=%v)\n got %+v\nwant %+v", c.line, c.top, got, c.want)
		}
	}
}

func TestFormatRoundTrips(t *testing.T) {
	cases := []struct {
		line Line
		top  bool
		want string
	}{
		{Line{Text: "Fix it", State: Open}, false, "Fix it"},
		{Line{Text: "Fix it", State: Open}, true, "Fix it $open"},
		{Line{Text: "Storage:"}, false, "Storage:"},
		{Line{Text: "a thought"}, false, "a thought $note"},
		{Line{Text: "project"}, true, "project"},
		{Line{Text: "Ship", State: Done, Flag: true, Due: "2026-11-20", Tags: []string{"a", "b"}}, false,
			"Ship ! @2026-11-20 #a #b $done"},
		{Line{Text: "the #tag and @fri sigils !", State: Open}, false, `the \#tag and \@fri sigils \!`},
	}
	for _, c := range cases {
		got := Format(c.line, c.top, today)
		if got != c.want {
			t.Errorf("Format(%+v, top=%v) = %q, want %q", c.line, c.top, got, c.want)
		}
		if back := Parse(got, c.top, today); !reflect.DeepEqual(back, c.line) {
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
