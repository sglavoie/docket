package chrome

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// TruncateAndPad truncates a line to exactly maxWidth visual characters,
// padding with spaces if shorter, truncating with "…" if longer.
func TruncateAndPad(line string, maxWidth int) string {
	if maxWidth < 4 {
		maxWidth = 4
	}

	visualWidth := lipgloss.Width(line)

	if visualWidth == maxWidth {
		return line
	}

	if visualWidth < maxWidth {
		// Pad with spaces
		return line + strings.Repeat(" ", maxWidth-visualWidth)
	}

	// Truncate with the shared ellipsis, then pad: a wide rune the cut could not
	// split leaves the result a column short of maxWidth.
	truncated := textfit.TruncateToVisualWidth(line, maxWidth)
	return truncated + strings.Repeat(" ", maxWidth-lipgloss.Width(truncated))
}

// overlayTitlePartFloor is the fewest columns a subject or a shortened title is
// worth drawing in. Under it the part is an ellipsis and a letter, which names
// nothing.
const overlayTitlePartFloor = 4

// FitOverlayTitle composes an overlay's one-line title, `title: subject suffix`,
// inside width text columns. The suffix carries a value no row repeats, so it
// is the last part to lose a column: the subject is fitted to what the title
// and the suffix leave, a width that leaves it nothing worth reading drops it,
// and a width too narrow for the two that remain shortens the title, which the
// overlay's own rows still explain. Only a width the suffix alone overruns
// clips it, so the title never wraps onto a second row.
func FitOverlayTitle(title, subject, suffix string, width int) string {
	const separator = ": "
	suffixWidth := lipgloss.Width(suffix)
	if subject != "" {
		room := width - lipgloss.Width(title) - lipgloss.Width(separator) - suffixWidth
		if room >= min(overlayTitlePartFloor, lipgloss.Width(subject)) {
			return title + separator + textfit.TruncateToVisualWidth(subject, room) + suffix
		}
	}
	if room := width - suffixWidth; suffix != "" && room >= min(overlayTitlePartFloor, lipgloss.Width(title)) {
		return textfit.TruncateToVisualWidth(title, room) + suffix
	}
	return textfit.TruncateToVisualWidth(title+suffix, width)
}

// RenderOverlayBox renders a modal surface with an uninterrupted semantic
// border, title and help text inside the frame, and a subtle one-cell halo on
// all four sides. Width is the framed box width; the halo adds two columns and
// two rows. Title and footer must already be styled.
// OverlayBoxContentTop and OverlayBoxContentLeft are the popup cell
// RenderOverlayBox draws its content's first character on when it has a title:
// the halo, the border, and the title row above it, and the halo, the border,
// and the padding cell beside it. A popup reading a click against its own
// content subtracts them rather than re-deriving the frame.
const (
	OverlayBoxContentTop  = 3
	OverlayBoxContentLeft = 3
)

func RenderOverlayBox(content string, width int, title, footer string, borderStyle lipgloss.Style) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderStyle.GetForeground()).
		Padding(0, 1).
		Width(width)

	var body strings.Builder
	if title != "" {
		body.WriteString(title)
	}
	if title != "" && content != "" {
		body.WriteString("\n")
	}
	body.WriteString(content)
	if footer != "" {
		if body.Len() > 0 {
			body.WriteString("\n")
		}
		body.WriteString(footer)
	}

	return renderOverlayHalo(boxStyle.Render(body.String()))
}

// renderOverlayHalo wraps a fully rendered overlay frame in a theme-aware ring.
// Padding short frame rows before adding the side cells keeps the halo rectangular
// even when styled content has uneven visual widths.
func renderOverlayHalo(frame string) string {
	if frame == "" {
		return ""
	}

	lines := strings.Split(frame, "\n")
	frameWidth := 0
	for _, line := range lines {
		frameWidth = max(frameWidth, lipgloss.Width(line))
	}

	haloCell := theme.OverlayHaloStyle.Render(" ")
	haloRow := theme.OverlayHaloStyle.Width(frameWidth + 2).Render("")
	rendered := make([]string, 0, len(lines)+2)
	rendered = append(rendered, haloRow)
	for _, line := range lines {
		if lineWidth := lipgloss.Width(line); lineWidth < frameWidth {
			line += strings.Repeat(" ", frameWidth-lineWidth)
		}
		rendered = append(rendered, haloCell+line+haloCell)
	}
	rendered = append(rendered, haloRow)

	return strings.Join(rendered, "\n")
}
