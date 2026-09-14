package tui

import (
	"strings"
	"testing"
)

// These tests hold the clipping-visibility half of the harness to its contract:
// a truncated line is reported as an observation, never as a layout issue, and a
// `not_clipped` failure says which of the two defects it found.

func TestScreenClippedLinesNamesTruncatedLinesOnly(t *testing.T) {
	plain := strings.Join([]string{
		"Active Projects · All",
		"  └── □ Restore the overdue billing reco" + truncationEllipsis + "   ",
		"  └── □ Confirm the release sign-off",
	}, "\n")

	clipped := screenClippedLines(plain)
	if len(clipped) != 1 {
		t.Fatalf("expected exactly one clipped line, got %d: %+v", len(clipped), clipped)
	}
	if clipped[0].Line != 2 {
		t.Fatalf("clipped line number = %d, want 2", clipped[0].Line)
	}
	if strings.HasSuffix(clipped[0].Text, " ") {
		t.Fatalf("clipped text keeps trailing padding: %q", clipped[0].Text)
	}
	if !strings.HasSuffix(clipped[0].Text, truncationEllipsis) {
		t.Fatalf("clipped text does not end at the truncation point: %q", clipped[0].Text)
	}
}

func TestScreenClippedLinesIsNotALayoutIssue(t *testing.T) {
	// A clipped line fits its terminal by construction, so the normalizer has
	// nothing to say about it. That is the whole reason the observation carries
	// the list separately; if this ever fails, `no_layout_issues` changed meaning.
	plain, issues := normalizeScreen("short"+truncationEllipsis, 40, 24)
	if len(issues) != 0 {
		t.Fatalf("normalizeScreen flagged a clipped line: %+v", issues)
	}
	if len(screenClippedLines(plain)) != 1 {
		t.Fatalf("clipped line went unreported")
	}
}

func TestClippedPrefixOfDistinguishesClippedFromAbsent(t *testing.T) {
	const want = "Restore the overdue billing reconciliation export"
	plain := "  └── □ Restore the overdue billing reco" + truncationEllipsis

	prefix, clipped := clippedPrefixOf(plain, want)
	if !clipped {
		t.Fatalf("clipped title reported as absent")
	}
	if prefix != "Restore the overdue billing reco" {
		t.Fatalf("clipped prefix = %q", prefix)
	}

	if _, clipped := clippedPrefixOf("nothing like it here"+truncationEllipsis, want); clipped {
		t.Fatalf("absent title reported as clipped")
	}
}

func TestClippedPrefixOfRefusesShortCoincidences(t *testing.T) {
	// "Re" ending a clipped line says nothing about the title the scenario named,
	// and calling that a clipped rendering would put a misleading prefix in the
	// failure message.
	if _, clipped := clippedPrefixOf("Re"+truncationEllipsis, "Release"); clipped {
		t.Fatalf("two-rune coincidence reported as clipped")
	}
	if _, clipped := clippedPrefixOf("head"+truncationEllipsis, "head"); clipped {
		t.Fatalf("a want shorter than the minimum prefix reported as clipped")
	}
}

func TestClippedPrefixOfPicksTheLongestMatch(t *testing.T) {
	// Several rows may clip the same title at different widths (a narrow pane and
	// a wide one, say). The longest is the informative one.
	plain := strings.Join([]string{
		"Release" + truncationEllipsis,
		"Release the platform migra" + truncationEllipsis,
	}, "\n")
	prefix, clipped := clippedPrefixOf(plain, "Release the platform migration window")
	if !clipped {
		t.Fatalf("clipped title reported as absent")
	}
	if prefix != "Release the platform migra" {
		t.Fatalf("clipped prefix = %q, want the longest match", prefix)
	}
}

func TestObservationReportsClippedLines(t *testing.T) {
	// The overdue title is clipped at this width today; the point of the check is
	// that the harness says so rather than that the row is narrow.
	driver, ws, err := NewFixtureDriver(FixtureUXAcceptance, 100, 35)
	if err != nil {
		t.Fatalf("failed to build fixture driver: %v", err)
	}
	defer ws.Cleanup()

	obs := driver.Observe(ObserveOptions{})
	if len(obs.ClippedLines) == 0 {
		t.Fatalf("no clipped lines reported for a screen that clips:\n%s", obs.PlainScreen)
	}
	for _, line := range obs.ClippedLines {
		if line.Line <= 0 {
			t.Fatalf("clipped line carries no line number: %+v", line)
		}
	}
	for _, issue := range obs.LayoutIssues {
		if issue.Kind == "width_overflow" {
			t.Fatalf("clipping was recorded as a width overflow: %+v", issue)
		}
	}
}

func TestNotClippedAssertionNamesWhichDefectItFound(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureUXAcceptance, 100, 35)
	if err != nil {
		t.Fatalf("failed to build fixture driver: %v", err)
	}
	defer ws.Cleanup()

	assert := func(value string) error {
		return driver.runAssert(UITestAction{Type: UITestActionAssert, Assert: "not_clipped", Value: value})
	}

	if err := assert(uxAcceptanceDueTodayTitle); err != nil {
		t.Fatalf("whole title failed not_clipped: %v", err)
	}

	err = assert(uxAcceptanceOverdueTitle)
	if err == nil {
		t.Fatalf("clipped title passed not_clipped")
	}
	if !strings.Contains(err.Error(), "clips") {
		t.Fatalf("clipped failure does not name clipping: %v", err)
	}

	err = assert("A title this workspace has never held")
	if err == nil {
		t.Fatalf("absent title passed not_clipped")
	}
	if strings.Contains(err.Error(), "clips") {
		t.Fatalf("absent failure blamed clipping: %v", err)
	}
}

func TestNotClippedAssertionRejectsEmptyValue(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionAssert, Assert: "not_clipped"}}}
	if err := sc.Validate(); err == nil {
		t.Fatalf("a not_clipped assertion with no value validated")
	}
}
