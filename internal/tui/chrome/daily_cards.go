package chrome

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The two cards of the daily loop: the Now card the Outliner header draws over
// the lanes, and the triage card a sorting pass composites over the rows. Both
// are pure presentation over values the Outliner already read, so every age,
// meter, and project name arrives rendered or as plain text.

// NowCardRail leads every Now card row: the accent bar and its gap, which is
// what sets the card apart from the lanes below it.
const NowCardRail = "▎ "

// nowCardTitleFloor is the narrowest the lead title is shortened to before the
// right-hand note gives up its room instead.
const nowCardTitleFloor = 16

// NowCardSpec is one Now card. Progress is the lead's own meter and count,
// already rendered, or empty; More counts the other Doing work it stands for.
type NowCardSpec struct {
	Title      string
	Meta       string
	ResumeNote string
	Progress   string
	More       int
}

// NowCardRowCount is the rows a spec draws: the lead, a resume row when there
// is a note, and a progress row when there is a meter or company in Doing.
func NowCardRowCount(spec NowCardSpec) int {
	rows := 1
	if spec.ResumeNote != "" {
		rows++
	}
	if spec.Progress != "" || spec.More > 0 {
		rows++
	}
	return rows
}

// RenderNowCard draws the card's rows, each exactly width wide, behind one
// accent rail so the block reads as a single object above the lanes.
func RenderNowCard(spec NowCardSpec, width int) []string {
	rail := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render(strings.TrimRight(NowCardRail, " ")) + " "
	inner := max(width-lipgloss.Width(rail), 1)

	lines := []string{rail + NowCardJustify(strings.TrimSpace(spec.Title), theme.BoldPrimaryStyle, spec.Meta, inner)}
	if spec.ResumeNote != "" {
		marker := theme.MutedStyle.Render("  ↳ ")
		room := max(inner-lipgloss.Width(marker), 1)
		text := lipgloss.NewStyle().Foreground(theme.ColorMuted).Italic(true).
			Render(textfit.TruncateIntentional(spec.ResumeNote, room, "…"))
		lines = append(lines, rail+padToWidth(marker+text, inner))
	}
	if spec.Progress != "" || spec.More > 0 {
		lines = append(lines, rail+nowCardProgressRow(spec.Progress, spec.More, inner))
	}
	return lines
}

// NowCardJustify lays a title on the left and a muted note on the right. The
// title gives way first, down to nowCardTitleFloor; past that the note is
// dropped whole rather than shortened, since half an age reads as a different
// one.
func NowCardJustify(title string, titleStyle lipgloss.Style, right string, width int) string {
	rightRendered := ""
	if right != "" {
		rightRendered = theme.MutedStyle.Render(right)
	}
	rightWidth := lipgloss.Width(rightRendered)
	room := width - rightWidth - 2
	if right != "" && room < min(nowCardTitleFloor, lipgloss.Width(title)) {
		rightRendered, rightWidth = "", 0
		room = width
	}
	left := titleStyle.Render(textfit.TruncateIntentional(title, max(room, 1), "…"))
	gap := width - lipgloss.Width(left) - rightWidth
	if rightWidth == 0 || gap < 1 {
		return padToWidth(left, width)
	}
	return left + strings.Repeat(" ", gap) + rightRendered
}

// nowCardProgressRow pairs the lead's progress with the count of the other
// Doing work. The meter is the lead's own reading and the tail only points
// elsewhere, so the tail is what a narrow row gives up.
func nowCardProgressRow(progress string, more, width int) string {
	left := ""
	if progress != "" {
		left = "  " + progress
	}
	right := ""
	if more > 0 {
		right = theme.MutedStyle.Render(fmt.Sprintf("+%d more in Doing", more))
	}
	leftWidth, rightWidth := lipgloss.Width(left), lipgloss.Width(right)
	if leftWidth+rightWidth+1 > width {
		return padToWidth(textfit.FitStyledLineToWidth(left, width), width)
	}
	return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
}

func padToWidth(line string, width int) string {
	if gap := width - lipgloss.Width(line); gap > 0 {
		return line + strings.Repeat(" ", gap)
	}
	return textfit.FitStyledLineToWidth(line, width)
}

// TriageCardSpec is one card of a sorting pass.
type TriageCardSpec struct {
	Heading string
	// Lead is an optional styled line above the progress rule: what the pass
	// already knows about the whole of what it is sorting.
	Lead        string
	Position    int
	Total       int
	Title       string
	Meta        string
	ResumeNote  string
	Description string
}

// triageCardBodyLines bounds what the task says about itself so the card stays
// a card; the whole text is one Peek away.
const triageCardBodyLines = 4

// TriageCardInnerWidth is the card's width: wide enough to read a title whole,
// narrow enough to sit as a card over the rows rather than replace them.
func TriageCardInnerWidth(width int) int {
	return min(max(width*3/5, 44), 72, max(width-6, 24))
}

// RenderTriageCard draws the card. Its keys are the footer's alone: the card
// stands directly above it, and saying them twice would spend its rows on a
// copy.
func RenderTriageCard(spec TriageCardSpec, width int) string {
	innerWidth := TriageCardInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	counter := theme.MutedStyle.Render(fmt.Sprintf("%d of %d", spec.Position, spec.Total))
	label := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(spec.Heading)
	header := label + strings.Repeat(" ", max(contentWidth-lipgloss.Width(label)-lipgloss.Width(counter), 1)) + counter

	lines := []string{triageProgressBar(spec.Position-1, spec.Total, contentWidth), ""}
	if spec.Lead != "" {
		lines = append([]string{textfit.TruncateIntentional(spec.Lead, contentWidth, "…")}, lines...)
	}
	for _, line := range textfit.WrapText(strings.TrimSpace(spec.Title), contentWidth) {
		lines = append(lines, theme.BoldPrimaryStyle.Render(line))
	}
	if spec.Meta != "" {
		lines = append(lines, theme.MutedStyle.Render(textfit.TruncateIntentional(spec.Meta, contentWidth, "…")))
	}
	if body := triageBodyLines(spec, contentWidth); len(body) > 0 {
		lines = append(lines, "")
		lines = append(lines, body...)
	}
	lines = append(lines, "")
	return RenderOverlayBox(strings.Join(lines, "\n"), innerWidth, header, "", lipgloss.NewStyle().Foreground(theme.ColorPrimary))
}

// triageProgressBar is the pass's distance as one thin rule: sorted cards in
// the success ink, the rest muted, so the end is visible from the first card.
func triageProgressBar(done, total, width int) string {
	if total <= 0 || width <= 0 {
		return ""
	}
	filled := min(max(done, 0)*width/total, width)
	return lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(strings.Repeat("━", filled)) +
		theme.MutedStyle.Render(strings.Repeat("─", width-filled))
}

// triageBodyLines is the resume note, then the head of the description.
func triageBodyLines(spec TriageCardSpec, width int) []string {
	var lines []string
	if note := strings.TrimSpace(spec.ResumeNote); note != "" {
		for _, line := range textfit.WrapText("↳ "+note, width) {
			lines = append(lines, lipgloss.NewStyle().Foreground(theme.ColorMuted).Italic(true).Render(line))
		}
	}
	for _, paragraph := range strings.Split(strings.TrimSpace(spec.Description), "\n") {
		if len(lines) > triageCardBodyLines {
			break
		}
		if strings.TrimSpace(paragraph) == "" {
			continue
		}
		for _, line := range textfit.WrapText(paragraph, width) {
			lines = append(lines, theme.MutedStyle.Render(line))
		}
	}
	if len(lines) > triageCardBodyLines {
		lines = append(lines[:triageCardBodyLines-1], theme.MutedStyle.Render("…"))
	}
	return lines
}
