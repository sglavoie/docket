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

// PickerPurpose identifies what action the GlobalProjectPicker is serving.
type PickerPurpose int

const (
	PickerPurposeQuickCapture PickerPurpose = iota
	PickerPurposeBulkCapture
	PickerPurposeNoteCreate
	PickerPurposeStatusUpdate
	PickerPurposeTaskMoveSource
	PickerPurposeTaskMoveDest
	PickerPurposeLinkedTask
	PickerPurposeTemplatePick
	PickerPurposePriorityUpdate
	PickerPurposeDueDate
	PickerPurposeSectionChange
)

// renderStepBreadcrumb renders a compact "Step X > Step Y > Step Z" indicator
// for multi-step overlay flows. The current step (0-based index) is highlighted
// in the primary color; the others are muted. Returns "" for nil/empty slices.
func renderStepBreadcrumb(steps []string, current int) string {
	if len(steps) == 0 {
		return ""
	}
	sep := MutedStyle.Render(" › ")
	var parts []string
	for i, s := range steps {
		if i == current {
			parts = append(parts, lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(s))
		} else {
			parts = append(parts, MutedStyle.Render(s))
		}
	}
	result := strings.Join(parts, sep)
	return " " + result
}

// pickerPurposeBreadcrumb returns the flow step labels and the 0-based index
// of the step represented by the GlobalProjectPicker for the given purpose.
func pickerPurposeBreadcrumb(purpose PickerPurpose) (steps []string, stepIdx int) {
	switch purpose {
	case PickerPurposeQuickCapture:
		return []string{"Select Project", "Quick Capture"}, 0
	case PickerPurposeBulkCapture:
		return []string{"Select Project", "Bulk Capture"}, 0
	case PickerPurposeNoteCreate:
		return []string{"Select Project", "Create Note"}, 0
	case PickerPurposeStatusUpdate:
		return []string{"Select Project", "Select Task", "Choose Status"}, 0
	case PickerPurposeTaskMoveSource:
		return []string{"Source Project", "Select Task", "Destination", "Section"}, 0
	case PickerPurposeTaskMoveDest:
		return []string{"Source Project", "Select Task", "Destination", "Section"}, 2
	case PickerPurposeLinkedTask:
		return []string{"Select Project", "Select Task", "Link Type", "Link Mode"}, 0
	case PickerPurposeTemplatePick:
		return []string{"Select Project", "Select Template", "Create Task"}, 0
	case PickerPurposePriorityUpdate:
		return []string{"Select Project", "Select Task", "Set Priority"}, 0
	case PickerPurposeDueDate:
		return []string{"Select Project", "Select Task", "Set Due Date"}, 0
	case PickerPurposeSectionChange:
		return []string{"Select Project", "Select Task", "Choose Section"}, 0
	default:
		return nil, 0
	}
}

// ShowGlobalProjectPickerMsg opens the project picker overlay for the given purpose.
type ShowGlobalProjectPickerMsg struct {
	Purpose    PickerPurpose
	ExcludeIDs []string // project IDs to exclude (used for dest picker in task move)
	BackMsg    tea.Msg  // emitted on Esc to go back one step; nil = cancel entirely
}

// GlobalProjectPickerResultMsg is emitted when the user selects a project.
type GlobalProjectPickerResultMsg struct {
	Project     *models.Project
	AllProjects []*models.Project
	Purpose     PickerPurpose
}

// GlobalProjectPicker is a reusable overlay for picking a project.
// It is stored on AppModel and rendered on top of the current view.
type GlobalProjectPicker struct {
	projects      []*models.Project
	filtered      []*models.Project
	cursor        int
	input         textinput.Model
	allProjects   []*models.Project
	purpose       PickerPurpose
	backMsg       tea.Msg // emitted on Esc to go back; nil = cancel entirely
	breadcrumb    []string
	breadcrumbIdx int
}

const globalProjectPickerMaxVisible = 10

func newGlobalProjectPicker(activeProjects, allProjects []*models.Project, purpose PickerPurpose, excludeIDs []string, backMsg tea.Msg) *GlobalProjectPicker {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 100
	ti.Focus()
	applyThemedTextInput(&ti)

	// Filter out excluded projects
	var projects []*models.Project
	if len(excludeIDs) == 0 {
		projects = activeProjects
	} else {
		excluded := make(map[string]bool, len(excludeIDs))
		for _, id := range excludeIDs {
			excluded[id] = true
		}
		for _, p := range activeProjects {
			if !excluded[p.ID] {
				projects = append(projects, p)
			}
		}
	}

	steps, stepIdx := pickerPurposeBreadcrumb(purpose)
	return &GlobalProjectPicker{
		projects:      projects,
		filtered:      projects,
		cursor:        0,
		input:         ti,
		allProjects:   allProjects,
		purpose:       purpose,
		backMsg:       backMsg,
		breadcrumb:    steps,
		breadcrumbIdx: stepIdx,
	}
}

func (p *GlobalProjectPicker) initCmd() tea.Cmd {
	return textinput.Blink
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *GlobalProjectPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		if p.cursor < len(p.filtered) {
			selected := p.filtered[p.cursor]
			return nil, true, GlobalProjectPickerResultMsg{
				Project:     selected,
				AllProjects: p.allProjects,
				Purpose:     p.purpose,
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
		p.moveCursor(-globalProjectPickerMaxVisible)
		return nil, false, nil

	case "ctrl+d":
		p.moveCursor(globalProjectPickerMaxVisible)
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

func (p *GlobalProjectPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	maxCursor := max(len(p.filtered)-1, 0)
	if p.cursor > maxCursor {
		p.cursor = maxCursor
	}
}

func (p *GlobalProjectPicker) fuzzyFilter() {
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

func (p *GlobalProjectPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Select Project")
	footerText := HelpStyle.Render("Enter: select · ↑↓/Ctrl+j/k: navigate · Esc: cancel")

	// Box sizing: 60% of screen with a comfortable minimum when space allows.
	// Title/footer chrome is clipped by renderTitledBox; it must never expand a
	// picker beyond the terminal width after this clamp.
	innerWidth := max(min(max(width*3/5, 40), width-4), 1)

	// Size the text input
	p.input.SetWidth(max(innerWidth-2, 1))

	var content strings.Builder
	if bc := renderStepBreadcrumb(p.breadcrumb, p.breadcrumbIdx); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}
	content.WriteString(p.input.View())
	content.WriteString("\n")

	if len(p.filtered) == 0 {
		content.WriteString(MutedStyle.Render("No matching projects."))
	} else {
		// Compute visible window
		visibleCount := min(len(p.filtered), globalProjectPickerMaxVisible)
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
			lineBudget := max(innerWidth-2, 1)
			line = truncateIntentional(line, lineBudget, "")

			// Pad to fill width for highlight background
			lineWidth := lipgloss.Width(line)
			if lineWidth < lineBudget {
				line += strings.Repeat(" ", lineBudget-lineWidth)
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

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
