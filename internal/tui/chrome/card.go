package chrome

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TruncateAndPad truncates a line to exactly maxWidth visual characters,
// padding with spaces if shorter, truncating with "..." if longer.
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

	// Truncate - dense card content is allowed to clip, so use the intentional
	// ANSI-aware audited helper.
	truncated := textfit.TruncateIntentional(line, maxWidth-3, "")
	truncatedWidth := lipgloss.Width(truncated)
	// Add ellipsis and pad to exact width
	return truncated + "..." + strings.Repeat(" ", maxWidth-truncatedWidth-3)
}

// overlayTitleSubjectFloor is the fewest columns a subject is worth drawing in.
// Under it the subject is an ellipsis and a letter, which names nothing.
const overlayTitleSubjectFloor = 4

// FitOverlayTitle composes an overlay's one-line title, `title: subject suffix`,
// inside width text columns. The subject is the one part that gives way: the
// title names the act and the suffix carries a value no row repeats, so both
// are kept whole and the subject is fitted to what they leave. A width that
// leaves the subject nothing worth reading drops it, and only then is the
// remainder clipped, so the title never wraps onto a second row.
func FitOverlayTitle(title, subject, suffix string, width int) string {
	const separator = ": "
	if subject != "" {
		room := width - lipgloss.Width(title) - lipgloss.Width(separator) - lipgloss.Width(suffix)
		if room >= min(overlayTitleSubjectFloor, lipgloss.Width(subject)) {
			return title + separator + textfit.TruncateToVisualWidth(subject, room) + suffix
		}
	}
	return textfit.TruncateToVisualWidth(title+suffix, width)
}

// RenderOverlayBox renders a modal surface with an uninterrupted semantic
// border, title and help text inside the frame, and a subtle one-cell halo on
// all four sides. Width is the framed box width; the halo adds two columns and
// two rows. Title and footer must already be styled.
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
