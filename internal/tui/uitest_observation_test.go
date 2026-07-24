package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestUITestObservationBasicFields(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	obs := driver.Observe(ObserveOptions{})

	if obs.CurrentView != "CrossProjectOutliner" {
		t.Errorf("CurrentView = %q, want CrossProjectOutliner", obs.CurrentView)
	}
	if len(obs.ViewStack) != 1 || obs.ViewStack[0] != "CrossProjectOutliner" {
		t.Errorf("ViewStack = %v, want [CrossProjectOutliner]", obs.ViewStack)
	}
	if obs.Width != 100 || obs.Height != 30 {
		t.Errorf("size = %dx%d, want 100x30", obs.Width, obs.Height)
	}
	if obs.PlainScreen == "" {
		t.Error("PlainScreen is empty")
	}
	if strings.Contains(obs.PlainScreen, "\x1b") {
		t.Error("PlainScreen still contains ANSI escapes")
	}
	if obs.ANSIScreen != "" {
		t.Error("ANSIScreen should be empty when IncludeANSI is false")
	}
}

func TestUITestObservationIncludeANSI(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	obs := driver.Observe(ObserveOptions{IncludeANSI: true})
	if obs.ANSIScreen == "" {
		t.Error("ANSIScreen should be populated when IncludeANSI is true")
	}
}

func TestUITestObservationStatusAndError(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)
	driver.App().statusMessage = "saved"
	driver.App().statusIsInfo = true

	obs := driver.Observe(ObserveOptions{})
	if obs.Status != "saved" {
		t.Errorf("Status = %q, want saved", obs.Status)
	}
	if !obs.StatusIsInfo {
		t.Error("StatusIsInfo = false, want true")
	}

	driver.App().err = errors.New("boom")
	obs = driver.Observe(ObserveOptions{})
	if obs.Error != "boom" {
		t.Errorf("Error = %q, want boom", obs.Error)
	}
}

func TestUITestObservationNormalizationIssues(t *testing.T) {
	// Width overflow: a 10-column line in an 8-column terminal.
	_, issues := normalizeScreen("wide line!!", 8, 24)
	if !hasIssueKind(issues, "width_overflow") {
		t.Errorf("expected width_overflow, got %+v", issues)
	}

	// Height overflow: 3 lines in a 2-row terminal.
	_, issues = normalizeScreen("a\nb\nc", 80, 2)
	if !hasIssueKind(issues, "height_overflow") {
		t.Errorf("expected height_overflow, got %+v", issues)
	}

	// Invalid UTF-8.
	_, issues = normalizeScreen("bad\xffbyte", 80, 24)
	if !hasIssueKind(issues, "invalid_utf8") {
		t.Errorf("expected invalid_utf8, got %+v", issues)
	}

	// ANSI is stripped and clean content yields no issues.
	plain, issues := normalizeScreen("\x1b[31mred\x1b[0m", 80, 24)
	if plain != "red" {
		t.Errorf("plain = %q, want red", plain)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func hasIssueKind(issues []UITestIssue, kind string) bool {
	for _, i := range issues {
		if i.Kind == kind {
			return true
		}
	}
	return false
}
