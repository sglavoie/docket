package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestHostedPanelLayoutZeroValueOwnsTheArea(t *testing.T) {
	var layout HostedPanelLayout
	if layout.Active() {
		t.Fatalf("zero layout claims a host: %+v", layout)
	}
	if got, want := layout.PanelRows(20), 20; got != want {
		t.Fatalf("panel rows = %d, want the whole area %d", got, want)
	}
}

func TestHostedPanelLayoutPanelRows(t *testing.T) {
	drawn := HostedPanelLayout{HostRows: 14, HostDrawn: true}
	if got, want := drawn.PanelRows(20), 6; got != want {
		t.Fatalf("panel rows beside a drawn host = %d, want %d", got, want)
	}
	// An undrawn host keeps its budget so it is not re-anchored, but the rows it
	// is not using this frame are the panel's.
	undrawn := HostedPanelLayout{HostRows: 14}
	if got, want := undrawn.PanelRows(20), 20; got != want {
		t.Fatalf("panel rows over an undrawn host = %d, want %d", got, want)
	}
}

func TestComposeHostedPanelPinsPanelToTheBottom(t *testing.T) {
	host := "one\ntwo"
	panel := "capture\ninput"

	composed := ComposeHostedPanel(host, panel, 5)
	lines := strings.Split(composed, "\n")
	if got, want := len(lines), 7; got != want {
		t.Fatalf("composed height = %d, want %d", got, want)
	}
	if lines[0] != "one" || lines[1] != "two" {
		t.Fatalf("host was not drawn first:\n%s", composed)
	}
	if lines[5] != "capture" || lines[6] != "input" {
		t.Fatalf("panel is not pinned under the host's padded block:\n%s", composed)
	}

	// A host with nothing left to give is no host at all.
	if got := ComposeHostedPanel(host, panel, 0); got != panel {
		t.Fatalf("panel without host rows = %q, want %q", got, panel)
	}
}

func TestRenderLabeledDividerIsOneRow(t *testing.T) {
	divider := RenderLabeledDivider("Capture → Unfiled", 60)
	if strings.Contains(divider, "\n") {
		t.Fatalf("labeled divider spans more than one row: %q", divider)
	}
	plain := ansi.Strip(divider)
	if got, want := lipgloss.Width(plain), 60; got != want {
		t.Fatalf("labeled divider width = %d, want %d", got, want)
	}
	if !strings.HasPrefix(plain, "── Capture → Unfiled ─") {
		t.Fatalf("labeled divider does not name its block: %q", plain)
	}
}

func TestRenderLabeledDividerDegradesWithoutRoom(t *testing.T) {
	for _, tc := range []struct {
		name  string
		label string
		width int
	}{
		{"no label", "", 40},
		{"label wider than the rule", "Capture → a very long project name", 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ansi.Strip(RenderLabeledDivider(tc.label, tc.width))
			if want := ansi.Strip(RenderHorizontalDivider(tc.width)); got != want {
				t.Fatalf("divider = %q, want the plain rule %q", got, want)
			}
		})
	}
}
