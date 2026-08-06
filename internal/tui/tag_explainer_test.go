package tui

import "testing"

func TestJoinTagExplainerContinuationsKeepsEveryPublisherAndLine(t *testing.T) {
	got := joinTagExplainerContinuations(
		tagExplainer{Continuations: []string{"behavioral first", "", "behavioral second"}},
		tagExplainer{},
		tagExplainer{Continuations: []string{"semantic"}},
	)
	want := "behavioral first" + footerHelpSeparator +
		"behavioral second" + footerHelpSeparator + "semantic"
	if got != want {
		t.Fatalf("joined continuation = %q, want %q", got, want)
	}
}
