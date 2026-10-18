package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/outliner"
)

// assertUXScreenFitsItsTerminal measures display cells, so a double-width rune
// cannot make a frame pass merely because its rune count fits.
func assertUXScreenFitsItsTerminal(t *testing.T, obs UITestObservation, size [2]int) {
	t.Helper()
	if len(obs.LayoutIssues) > 0 {
		t.Fatalf("launch view: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	lines := strings.Split(obs.PlainScreen, "\n")
	if len(lines) > size[1] {
		t.Fatalf("the screen drew %d lines into a %d-row terminal", len(lines), size[1])
	}
	for index, line := range lines {
		if width := ansi.StringWidth(line); width > size[0] {
			t.Fatalf("line %d spends %d display cells in a %d-column terminal: %q", index+1, width, size[0], line)
		}
	}
}

// assertUXMonochromeKeepsContent projects the styled screen through the ASCII
// color profile, which is what a monochrome terminal does to it, and requires
// the result to be the same text. Color may carry emphasis; it may not be the
// only place a fact is stated.
func assertUXMonochromeKeepsContent(t *testing.T, obs UITestObservation, required ...string) {
	t.Helper()
	var mono bytes.Buffer
	writer := colorprofile.Writer{Forward: &mono, Profile: colorprofile.ASCII}
	if _, err := writer.WriteString(obs.ANSIScreen); err != nil {
		t.Fatalf("project the screen through the ASCII profile: %v", err)
	}
	plain := ansi.Strip(mono.String())
	if plain != obs.PlainScreen {
		t.Fatalf("monochrome output changed content\n%s", plain)
	}
	for _, text := range required {
		if !strings.Contains(plain, text) {
			t.Fatalf("monochrome output lacks %q\n%s", text, plain)
		}
	}
}

// uxDailyUseDriver launches one named fixture at one size. It is the one place
// the fixture name on a frame is turned into a workspace, so no staging function
// has to know how a driver is built or remember to register the cleanup that
// removes the temporary workspace.
func uxDailyUseDriver(t *testing.T, fixture string, width, height int) (*UITestDriver, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(fixture, width, height)
	if err != nil {
		t.Fatalf("fixture %s: %v", fixture, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatalf("the %s fixture app has no Outliner", fixture)
	}
	return driver, view
}
