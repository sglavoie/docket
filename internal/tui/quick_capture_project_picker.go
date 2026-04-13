package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// ShowQuickCaptureProjectPickerMsg signals that quick capture needs a project
// selection because no project is currently active.
type ShowQuickCaptureProjectPickerMsg struct{}

// QuickCaptureProjectPicker is a lightweight overlay that lets the user pick a
// destination project before opening quick capture. It is stored on AppModel
// and rendered on top of the current view, following the QuickNoteOverlay pattern.
type QuickCaptureProjectPicker struct {
	projects    []*models.Project
	filtered    []*models.Project
	cursor      int
	input       textinput.Model
	allProjects []*models.Project
}

const quickCapturePickerMaxVisible = 10

func newQuickCaptureProjectPicker(activeProjects, allProjects []*models.Project) *QuickCaptureProjectPicker {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 100
	ti.Focus()

	return &QuickCaptureProjectPicker{
		projects:    activeProjects,
		filtered:    activeProjects,
		cursor:      0,
		input:       ti,
		allProjects: allProjects,
	}
}

func (p *QuickCaptureProjectPicker) initCmd() tea.Cmd {
	return textinput.Blink
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *QuickCaptureProjectPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil

	case "enter":
		if p.cursor < len(p.filtered) {
			selected := p.filtered[p.cursor]
			return nil, true, ShowQuickCaptureMsg{
				Project:           selected,
				AllProjects:       p.allProjects,
				FromGlobalCapture: true,
			}
		}
		return nil, false, nil

	case "up", "ctrl+k", "ctrl+p":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n":
		p.moveCursor(1)
		return nil, false, nil

	case "ctrl+u":
		p.moveCursor(-quickCapturePickerMaxVisible)
		return nil, false, nil

	case "ctrl+d":
		p.moveCursor(quickCapturePickerMaxVisible)
		return nil, false, nil

	default:
		prev := p.input.Value()
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		if p.input.Value() != prev {
			p.fuzzyFilter()
		}
		return cmd, false, nil
	}
}

func (p *QuickCaptureProjectPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	maxCursor := max(len(p.filtered)-1, 0)
	if p.cursor > maxCursor {
		p.cursor = maxCursor
	}
}

func (p *QuickCaptureProjectPicker) fuzzyFilter() {
	query := strings.TrimSpace(p.input.Value())

	if query == "" {
		p.filtered = p.projects
		p.cursor = 0
		return
	}

	type scoredProject struct {
		project *models.Project
		score   int
	}
	var scored []scoredProject

	for _, proj := range p.projects {
		text := fmt.Sprintf("%s %s", proj.Name, proj.Context)
		score := fuzzy.ScoreTerms(text, query)
		if score > 0 {
			scored = append(scored, scoredProject{project: proj, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	p.filtered = make([]*models.Project, len(scored))
	for i, s := range scored {
		p.filtered[i] = s.project
	}

	maxCursor := max(len(p.filtered)-1, 0)
	if p.cursor > maxCursor {
		p.cursor = maxCursor
	}
}

func (p *QuickCaptureProjectPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Select Project")
	footerText := HelpStyle.Render("Enter: select · ↑↓/Ctrl+j/k: navigate · Esc: cancel")

	// Box sizing: 60% of screen, min 40
	maxWidth := min(max(width*3/5, 40), width-4)

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	// Size the text input
	p.input.SetWidth(max(innerWidth-2, 10))

	var content strings.Builder
	content.WriteString(p.input.View())
	content.WriteString("\n")

	if len(p.filtered) == 0 {
		content.WriteString(MutedStyle.Render("No matching projects."))
	} else {
		// Compute visible window
		visibleCount := min(len(p.filtered), quickCapturePickerMaxVisible)
		start := 0
		if p.cursor >= visibleCount {
			start = p.cursor - visibleCount + 1
		}
		end := min(start+visibleCount, len(p.filtered))

		for i := start; i < end; i++ {
			proj := p.filtered[i]
			icon := proj.StatusIconEmoji()
			if icon == "" {
				icon = " "
			}
			name := proj.Name
			ctx := ""
			if proj.Context != "" {
				ctx = MutedStyle.Render(" (" + proj.Context + ")")
			}

			line := fmt.Sprintf(" %s %s%s", icon, name, ctx)

			// Pad to fill width for highlight background
			lineWidth := lipgloss.Width(line)
			if lineWidth < innerWidth-2 {
				line += strings.Repeat(" ", innerWidth-2-lineWidth)
			}

			if i == p.cursor {
				line = SelectedStyle.Render(line)
			}

			content.WriteString(line)
			if i < end-1 {
				content.WriteString("\n")
			}
		}

		// Show scroll indicator
		if len(p.filtered) > visibleCount {
			indicator := MutedStyle.Render(fmt.Sprintf(" %d/%d", p.cursor+1, len(p.filtered)))
			content.WriteString("\n")
			content.WriteString(indicator)
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content.String())

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}

	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)

	return strings.Join(lines, "\n")
}
