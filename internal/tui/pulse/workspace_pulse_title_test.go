package pulse

import "testing"

func TestWindowTitleNamesNonzeroComponentsInReadoutOrder(t *testing.T) {
	cases := []struct {
		pulse WorkspacePulse
		want  string
	}{
		{WorkspacePulse{}, "Docket"},
		{WorkspacePulse{Done: 4}, "Docket"},
		{WorkspacePulse{Inbox: 2, Today: 3, Overdue: 1, Done: 4}, "Docket · inbox 2 · today 3 · overdue 1"},
		{WorkspacePulse{Overdue: 12}, "Docket · overdue 12"},
	}
	for _, tc := range cases {
		if got := WindowTitle(tc.pulse); got != tc.want {
			t.Errorf("WindowTitle(%+v) = %q, want %q", tc.pulse, got, tc.want)
		}
	}
}
