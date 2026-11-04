package pulse

import "testing"

func TestQuitSummary(t *testing.T) {
	cases := []struct {
		pulse WorkspacePulse
		want  string
	}{
		{WorkspacePulse{}, ""},
		{WorkspacePulse{Inbox: 3}, ""},
		{WorkspacePulse{Done: 4}, "docket: 4 done today · Today is clear"},
		{WorkspacePulse{Done: 2, Today: 3, Overdue: 1}, "docket: 2 done today · 3 still in Today · 1 overdue"},
		{WorkspacePulse{Today: 1}, "docket: 1 still in Today"},
	}
	for _, c := range cases {
		if got := QuitSummary(c.pulse); got != c.want {
			t.Errorf("QuitSummary(%+v) = %q, want %q", c.pulse, got, c.want)
		}
	}
}
