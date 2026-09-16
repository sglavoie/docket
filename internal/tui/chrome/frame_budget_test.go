package chrome

import "testing"

// TestShellFrameMetricsZeroValueIsTheBaselineFooter is the contract every view
// that is never told anything depends on. A footer with nothing to say is still
// a divider and a row, which is the height the shell has always sized views
// against, so a view holding the zero value reserves nothing and behaves exactly
// as it did before there was anything to tell it.
func TestShellFrameMetricsZeroValueIsTheBaselineFooter(t *testing.T) {
	var zero ShellFrameMetrics
	if got := zero.FooterRows(); got != ShellFooterBaselineRows {
		t.Fatalf("zero metrics measure %d footer rows, want the baseline %d", got, ShellFooterBaselineRows)
	}
	if got := zero.ExcessRows(); got != 0 {
		t.Fatalf("zero metrics reserve %d rows, want none", got)
	}
}

// TestShellFrameMetricsChargeEveryRowBeyondTheBaselineOnce walks the shapes the
// footer actually takes. The excess is what a view subtracts from its own
// content, so it has to be the difference and nothing else: a footer that says
// two things is one row taller than one that says either, and one that says
// nothing is not shorter than the baseline the view was already sized for.
func TestShellFrameMetricsChargeEveryRowBeyondTheBaselineOnce(t *testing.T) {
	for _, test := range []struct {
		name    string
		metrics ShellFrameMetrics
		footer  int
		excess  int
	}{
		{name: "resting controls", metrics: ShellFrameMetrics{ControlRows: 1}, footer: 2, excess: 0},
		{name: "feedback alone", metrics: ShellFrameMetrics{FeedbackRows: 1}, footer: 2, excess: 0},
		{name: "feedback beside controls", metrics: ShellFrameMetrics{FeedbackRows: 1, ControlRows: 1}, footer: 3, excess: 1},
		{name: "wrapped controls", metrics: ShellFrameMetrics{ControlRows: 2}, footer: 3, excess: 1},
		{name: "both at the floor", metrics: ShellFrameMetrics{FeedbackRows: 1, ControlRows: 2}, footer: 4, excess: 2},
		{name: "silent footer", metrics: ShellFrameMetrics{}, footer: 2, excess: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.metrics.FooterRows(); got != test.footer {
				t.Errorf("FooterRows = %d, want %d", got, test.footer)
			}
			if got := test.metrics.ExcessRows(); got != test.excess {
				t.Errorf("ExcessRows = %d, want %d", got, test.excess)
			}
			if got, want := test.metrics.FooterRows()-test.metrics.ExcessRows(), ShellFooterBaselineRows; got != want {
				t.Errorf("footer less its excess = %d, want the baseline %d every view is sized against", got, want)
			}
		})
	}
}

// TestShellFrameMetricsNeverReportANegativeReservation guards the one arithmetic
// a caller cannot check for itself. A view subtracts the excess from its budget,
// so a negative one would hand it rows the frame does not have.
func TestShellFrameMetricsNeverReportANegativeReservation(t *testing.T) {
	for _, metrics := range []ShellFrameMetrics{
		{FeedbackRows: -1},
		{ControlRows: -3},
		{FeedbackRows: -2, ControlRows: 1},
	} {
		if got := metrics.ExcessRows(); got < 0 {
			t.Fatalf("%+v reserves %d rows", metrics, got)
		}
		if got := metrics.FooterRows(); got < ShellFooterBaselineRows {
			t.Fatalf("%+v measures %d footer rows, below the baseline %d", metrics, got, ShellFooterBaselineRows)
		}
	}
}
