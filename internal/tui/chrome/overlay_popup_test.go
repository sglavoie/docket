package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestOverlayPopupDimmedKeepsCompleteHaloInsideViewport(t *testing.T) {
	const (
		baseWidth  = 30
		baseHeight = 12
		frameWidth = 18
	)
	baseLine := strings.Repeat(".", baseWidth)
	base := strings.Repeat(baseLine+"\n", baseHeight-1) + baseLine
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	popup := RenderOverlayBox("body", frameWidth, "Title", "Enter · Esc", borderStyle)

	rendered := OverlayPopupDimmed(base, popup, baseWidth, baseHeight)
	lines := strings.Split(rendered, "\n")
	if got := len(lines); got != baseHeight {
		t.Fatalf("composed height = %d, want %d", got, baseHeight)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != baseWidth {
			t.Fatalf("composed line %d width = %d, want %d", i, got, baseWidth)
		}
	}

	popupHeight := lipgloss.Height(popup)
	startRow := baseHeight - popupHeight
	startCol := (baseWidth - lipgloss.Width(popup)) / 2
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = ansi.Strip(line)
	}

	topHalo := ansi.Cut(plain[startRow], startCol, startCol+lipgloss.Width(popup))
	bottomHalo := ansi.Cut(plain[baseHeight-1], startCol, startCol+lipgloss.Width(popup))
	if strings.TrimSpace(topHalo) != "" || strings.TrimSpace(bottomHalo) != "" {
		t.Fatalf("top or bottom halo was clipped:\n%s", strings.Join(plain, "\n"))
	}

	topBorder := ansi.Cut(plain[startRow+1], startCol, startCol+lipgloss.Width(popup))
	bottomBorder := ansi.Cut(plain[baseHeight-2], startCol, startCol+lipgloss.Width(popup))
	if !strings.HasPrefix(topBorder, " ╭") || !strings.HasSuffix(topBorder, "╮ ") {
		t.Fatalf("top border lacks both side halo cells: %q", topBorder)
	}
	if !strings.HasPrefix(bottomBorder, " ╰") || !strings.HasSuffix(bottomBorder, "╯ ") {
		t.Fatalf("bottom border lacks both side halo cells: %q", bottomBorder)
	}
}

func TestOverlayPopupDimmedFitsTallPopupWithoutClippingFrame(t *testing.T) {
	const (
		baseWidth  = 30
		baseHeight = 10
		frameWidth = 18
	)
	baseLine := strings.Repeat(".", baseWidth)
	base := strings.Repeat(baseLine+"\n", baseHeight-1) + baseLine
	content := strings.Repeat("row\n", 16) + "last row"
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorWarning)
	popup := RenderOverlayBox("body\n"+content, frameWidth, "Confirm", "y · n", borderStyle)
	if lipgloss.Height(popup) <= baseHeight {
		t.Fatal("test popup must exceed the viewport height")
	}

	rendered := OverlayPopupDimmed(base, popup, baseWidth, baseHeight)
	lines := strings.Split(ansi.Strip(rendered), "\n")
	if got := len(lines); got != baseHeight {
		t.Fatalf("composed height = %d, want %d", got, baseHeight)
	}

	startCol := (baseWidth - lipgloss.Width(popup)) / 2
	topHalo := ansi.Cut(lines[0], startCol, startCol+lipgloss.Width(popup))
	topBorder := ansi.Cut(lines[1], startCol, startCol+lipgloss.Width(popup))
	bottomBorder := ansi.Cut(lines[baseHeight-2], startCol, startCol+lipgloss.Width(popup))
	bottomHalo := ansi.Cut(lines[baseHeight-1], startCol, startCol+lipgloss.Width(popup))
	if strings.TrimSpace(topHalo) != "" || strings.TrimSpace(bottomHalo) != "" {
		t.Fatalf("height fitting clipped an outer halo row:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(topBorder, " ╭") || !strings.HasSuffix(topBorder, "╮ ") {
		t.Fatalf("height fitting clipped the top border: %q", topBorder)
	}
	if !strings.HasPrefix(bottomBorder, " ╰") || !strings.HasSuffix(bottomBorder, "╯ ") {
		t.Fatalf("height fitting clipped the bottom border: %q", bottomBorder)
	}
	if !strings.Contains(rendered, "Confirm") || !strings.Contains(rendered, "y · n") {
		t.Fatalf("height fitting dropped title or help:\n%s", strings.Join(lines, "\n"))
	}
}

func TestOverlayPopupDimmedPreservesNestedModalFrames(t *testing.T) {
	const (
		baseWidth  = 44
		baseHeight = 18
	)
	baseLine := strings.Repeat(".", baseWidth)
	base := strings.Repeat(baseLine+"\n", baseHeight-1) + baseLine
	parent := RenderOverlayBox(
		"parent content\nline two\nline three\nline four\nline five\nline six",
		30,
		"Parent actions",
		"Enter",
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
	child := RenderOverlayBox(
		"Proceed?",
		18,
		"Confirm",
		"y · n",
		lipgloss.NewStyle().Foreground(theme.ColorWarning),
	)

	withParent := OverlayPopupDimmed(base, parent, baseWidth, baseHeight)
	rendered := OverlayPopupDimmed(withParent, child, baseWidth, baseHeight)
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "Parent actions") || !strings.Contains(plain, "Confirm") {
		t.Fatalf("nested composition dropped a modal identity:\n%s", plain)
	}
	for i, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got != baseWidth {
			t.Fatalf("nested line %d width = %d, want %d", i, got, baseWidth)
		}
	}

	childWidth := lipgloss.Width(child)
	startCol := (baseWidth - childWidth) / 2
	lines := strings.Split(plain, "\n")
	childTop := baseHeight - lipgloss.Height(child)
	topBorder := ansi.Cut(lines[childTop+1], startCol, startCol+childWidth)
	bottomBorder := ansi.Cut(lines[baseHeight-2], startCol, startCol+childWidth)
	if !strings.HasPrefix(topBorder, " ╭") || !strings.HasSuffix(topBorder, "╮ ") ||
		!strings.HasPrefix(bottomBorder, " ╰") || !strings.HasSuffix(bottomBorder, "╯ ") {
		t.Fatalf("top nested modal lost its complete frame:\n%s", plain)
	}
}

// A popup that hangs off something the host is still drawing anchors where it
// was told to, keeps every row above that anchor as the host wrote it, and dims
// the rest — the whole difference between a modal that replaced the view and one
// that is standing on it.
func TestOverlayPopupDimmedAtAnchorsWithoutCoveringTheRowsAbove(t *testing.T) {
	const (
		baseWidth  = 40
		baseHeight = 14
		frameWidth = 16
		startRow   = 2
		startCol   = 5
	)
	baseLine := strings.Repeat(".", baseWidth)
	base := strings.Repeat(baseLine+"\n", baseHeight-1) + baseLine
	popup := RenderOverlayBox("one\ntwo", frameWidth, "", "",
		lipgloss.NewStyle().Foreground(theme.ColorPrimary))

	rendered := OverlayPopupDimmedAt(base, popup, baseWidth, baseHeight, startRow, startCol)
	lines := strings.Split(rendered, "\n")
	if got := len(lines); got != baseHeight {
		t.Fatalf("composed height = %d, want %d", got, baseHeight)
	}
	plain := strings.Split(ansi.Strip(rendered), "\n")
	for i, line := range plain {
		if got := lipgloss.Width(line); got != baseWidth {
			t.Fatalf("composed line %d width = %d, want %d", i, got, baseWidth)
		}
	}

	for row := range startRow {
		if plain[row] != baseLine {
			t.Fatalf("row %d above the anchor was written over: %q", row, plain[row])
		}
		if lines[row] == plain[row] {
			t.Fatalf("row %d above the anchor was left undimmed: %q", row, lines[row])
		}
	}

	popupWidth := lipgloss.Width(popup)
	border := ansi.Cut(plain[startRow+1], startCol, startCol+popupWidth)
	if !strings.HasPrefix(border, " ╭") || !strings.HasSuffix(border, "╮ ") {
		t.Fatalf("the box did not land at column %d: %q", startCol, border)
	}
	if left := ansi.Cut(plain[startRow+1], 0, startCol); left != strings.Repeat(".", startCol) {
		t.Fatalf("the columns left of the anchor lost the host: %q", left)
	}
	if right := ansi.Cut(plain[startRow+1], startCol+popupWidth, baseWidth); strings.TrimSpace(right) == "" {
		t.Fatalf("the columns right of the box lost the host: %q", right)
	}
	if tail := plain[baseHeight-1]; tail != baseLine {
		t.Fatalf("a short popup wrote below itself: %q", tail)
	}
}

// The anchor is a budget as well as a position: a popup too tall for what the
// anchor leaves loses interior rows rather than the host's header or its own
// frame, and one anchored past the last row is pulled back onto it.
func TestOverlayPopupDimmedAtFitsWhatTheAnchorLeaves(t *testing.T) {
	const (
		baseWidth  = 40
		baseHeight = 12
		frameWidth = 16
		startRow   = 3
	)
	baseLine := strings.Repeat(".", baseWidth)
	base := strings.Repeat(baseLine+"\n", baseHeight-1) + baseLine
	popup := RenderOverlayBox(strings.Repeat("row\n", 14)+"last", frameWidth, "Tall", "Esc",
		lipgloss.NewStyle().Foreground(theme.ColorPrimary))
	if lipgloss.Height(popup) <= baseHeight-startRow {
		t.Fatal("test popup must exceed what the anchor leaves")
	}

	plain := strings.Split(ansi.Strip(OverlayPopupDimmedAt(base, popup, baseWidth, baseHeight, startRow, 0)), "\n")
	if got := len(plain); got != baseHeight {
		t.Fatalf("composed height = %d, want %d", got, baseHeight)
	}
	for row := range startRow {
		if plain[row] != baseLine {
			t.Fatalf("a fitted popup covered host row %d: %q", row, plain[row])
		}
	}
	popupWidth := lipgloss.Width(popup)
	if top := ansi.Cut(plain[startRow+1], 0, popupWidth); !strings.HasPrefix(top, " ╭") {
		t.Fatalf("fitting dropped the top border: %q", top)
	}
	if bottom := ansi.Cut(plain[baseHeight-2], 0, popupWidth); !strings.HasPrefix(bottom, " ╰") {
		t.Fatalf("fitting dropped the bottom border: %q", bottom)
	}
	if !strings.Contains(strings.Join(plain, "\n"), "Tall") {
		t.Fatalf("fitting dropped the title:\n%s", strings.Join(plain, "\n"))
	}

	clamped := strings.Split(ansi.Strip(OverlayPopupDimmedAt(base, popup, baseWidth, baseHeight, baseHeight+4, 0)), "\n")
	if got := len(clamped); got != baseHeight {
		t.Fatalf("an anchor past the frame composed %d rows, want %d", got, baseHeight)
	}
	if clamped[baseHeight-1] == baseLine {
		t.Fatalf("an anchor past the frame drew nothing at all:\n%s", strings.Join(clamped, "\n"))
	}
	if clamped[baseHeight-2] != baseLine {
		t.Fatalf("an anchor past the frame wrote above the row it was pulled back to:\n%s", strings.Join(clamped, "\n"))
	}
}
