package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The outline editor collects every rejection in one pass, so a buffer that is
// wrong in six places reports six problems. The status bar is one truncated
// line and cannot carry that, which is why the refusal gets a scrollable
// surface of its own.

// outlineErrorsChrome is the number of rows the surface spends on everything
// that is not a problem line: the title, the blank beneath it, the blank above
// the buffer path, the path itself, and the two-row guidance block.
const outlineErrorsChrome = 6

// outlineErrorsTitle is the surface's own name. It is a constant because the
// subtitle budget is measured against it.
const outlineErrorsTitle = "Outline Not Applied"

// OutlineErrorsViewModel lists the line-numbered problems one outline buffer
// collected, alongside the temp file those line numbers refer to. The buffer is
// deliberately still on disk: the user reopens it, fixes the named lines, and
// runs the round trip again.
type OutlineErrorsViewModel struct {
	projectName string
	bufferPath  string
	problems    []lineError
	width       int
	height      int
	offset      int
}

// NewOutlineErrorsViewModel builds the refusal surface for one rejected buffer.
func NewOutlineErrorsViewModel(projectName, bufferPath string, problems []lineError) *OutlineErrorsViewModel {
	return &OutlineErrorsViewModel{
		projectName: projectName,
		bufferPath:  bufferPath,
		problems:    problems,
	}
}

func (m *OutlineErrorsViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.clampOffset()
}

func (m *OutlineErrorsViewModel) Init() tea.Cmd { return nil }

func (m *OutlineErrorsViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.offset--
	case "down", "j":
		m.offset++
	case "pgup", "ctrl+u":
		m.offset -= max(m.capacity()/2, 1)
	case "pgdown", "ctrl+d":
		m.offset += max(m.capacity()/2, 1)
	case "g", "home":
		m.offset = 0
	case "G", "end":
		m.offset = len(m.lines())
	default:
		return m, nil
	}
	m.clampOffset()
	return m, nil
}

func (m *OutlineErrorsViewModel) View() tea.View {
	width := max(m.width, 1)
	lines := m.lines()
	capacity := m.capacity()

	var builder strings.Builder
	builder.WriteString(renderFullscreenTitle(outlineErrorsTitle, m.subtitle(width-lipgloss.Width(outlineErrorsTitle)-2)))
	builder.WriteString("\n\n")

	visible := lines[min(m.offset, len(lines)):]
	if len(visible) > capacity {
		visible = visible[:capacity]
	}
	// The validation style's top margin separates one error from the form field
	// above it. Here the problems are a list, so that margin would double every
	// row and make the scroll budget a function of the style instead of the
	// content.
	problemStyle := ValidationErrorStyle.UnsetMarginTop()
	for _, line := range visible {
		builder.WriteString(problemStyle.Render(truncateToVisualWidth(line, width)))
		builder.WriteString("\n")
	}
	for range capacity - len(visible) {
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString(MutedStyle.Render(truncateToVisualWidth("Buffer kept at "+m.bufferPath, width)))
	builder.WriteString("\n")
	builder.WriteString(renderGuidance(m.guidance(len(lines), capacity), width))

	return tea.NewView(builder.String())
}

func (m *OutlineErrorsViewModel) StatusBarLabel() string { return "Outline Errors" }

func (m *OutlineErrorsViewModel) HasTextInput() bool { return false }

func (m *OutlineErrorsViewModel) IsFullHeight() bool { return false }

// ShouldCaptureEsc is false so Esc pops the surface through the ordinary
// overlay unwind. There is no draft here to protect.
func (m *OutlineErrorsViewModel) ShouldCaptureEsc() bool { return false }

// subtitle degrades rather than clips, because a clipped count reads as a
// different number. The reassurance goes first: it is the one clause the
// surface's own title already implies.
func (m *OutlineErrorsViewModel) subtitle(budget int) string {
	scope := m.projectName
	if scope == "" {
		scope = "project"
	}
	count := fmt.Sprintf("%d problem%s", len(m.problems), pluralize(len(m.problems), "", "s"))
	for _, candidate := range []string{
		scope + " · " + count + " · nothing was changed",
		scope + " · " + count,
		count,
	} {
		if lipgloss.Width(candidate) <= budget {
			return candidate
		}
	}
	return truncateToVisualWidth(count, budget)
}

// lines renders each problem as the same "Line N: message" shape multiline
// capture uses, wrapped rather than clipped so a long message stays readable at
// narrow widths.
func (m *OutlineErrorsViewModel) lines() []string {
	width := max(m.width, 1)
	out := make([]string, 0, len(m.problems))
	for _, problem := range m.problems {
		out = append(out, wrapText(fmt.Sprintf("Line %d: %s", problem.line, problem.message), width)...)
	}
	return out
}

func (m *OutlineErrorsViewModel) capacity() int {
	return max(m.height-outlineErrorsChrome, 1)
}

func (m *OutlineErrorsViewModel) clampOffset() {
	m.offset = min(m.offset, max(len(m.lines())-m.capacity(), 0))
	m.offset = max(m.offset, 0)
}

func (m *OutlineErrorsViewModel) guidance(total, capacity int) GuidanceSpec {
	spec := GuidanceSpec{Hints: requiredGuidanceHints("Esc close"), Rows: 2}
	if total > capacity {
		spec.Hints = append(spec.Hints, optionalGuidanceHints(10, "j/k scroll")...)
		spec.Supplemental = fmt.Sprintf("%d–%d of %d", m.offset+1, min(m.offset+capacity, total), total)
	}
	return spec
}
