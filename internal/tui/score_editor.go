package tui

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
)

// ScoreEditorDoneMsg is sent when the score editor finishes (save or cancel).
type ScoreEditorDoneMsg struct {
	Task    *models.Task
	Saved   bool
	Project *models.Project
}

// ScoreEditorModel is a small overlay for editing scoring dimensions on a task.
type ScoreEditorModel struct {
	task    *models.Task
	project *models.Project
	config  *config.Config
	profile *scoring.ScoringProfile

	// Per-dimension editing state
	values  []int    // current value for each dimension
	sources []string // "manual", "derived", or "default" (for display)
	cursor  int      // which dimension is focused

	// Dimensions
	width  int
	height int
}

// NewScoreEditorModel creates a score editor for a task.
func NewScoreEditorModel(task *models.Task, project *models.Project, cfg *config.Config) *ScoreEditorModel {
	profile := cfg.GetProjectScoringProfile(project.ID)
	if profile == nil {
		return nil
	}

	breakdown := scoring.ScoreBreakdown(task, profile)
	values := make([]int, len(breakdown))
	sources := make([]string, len(breakdown))
	for i, r := range breakdown {
		values[i] = r.Value
		sources[i] = r.Source
	}

	return &ScoreEditorModel{
		task:    task,
		project: project,
		config:  cfg,
		profile: profile,
		values:  values,
		sources: sources,
	}
}

// SetSize implements View.
func (m *ScoreEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init implements tea.Model.
func (m *ScoreEditorModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *ScoreEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg {
				return ScoreEditorDoneMsg{Task: m.task, Saved: false, Project: m.project}
			}

		case "enter":
			return m, m.save()

		case "j", "down":
			if m.cursor < len(m.profile.Dimensions)-1 {
				m.cursor++
			}
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "l", "right", "+":
			dim := &m.profile.Dimensions[m.cursor]
			if m.values[m.cursor] < dim.Max {
				m.values[m.cursor]++
				m.sources[m.cursor] = "manual"
			}
			return m, nil

		case "h", "left", "-":
			dim := &m.profile.Dimensions[m.cursor]
			if m.values[m.cursor] > dim.Min {
				m.values[m.cursor]--
				m.sources[m.cursor] = "manual"
			}
			return m, nil

		case "d":
			// Reset to derived/default
			dim := &m.profile.Dimensions[m.cursor]
			if m.task.ScoreDimensions != nil {
				delete(m.task.ScoreDimensions, dim.ID)
			}
			// Recompute this dimension
			result := scoring.ScoreBreakdown(m.task, m.profile)
			if m.cursor < len(result) {
				m.values[m.cursor] = result[m.cursor].Value
				m.sources[m.cursor] = result[m.cursor].Source
			}
			return m, nil

		default:
			// Direct number input: accept digits for quick value setting
			if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
				digit, _ := strconv.Atoi(msg.String())
				dim := &m.profile.Dimensions[m.cursor]
				if digit >= dim.Min && digit <= dim.Max {
					m.values[m.cursor] = digit
					m.sources[m.cursor] = "manual"
				}
			}
			return m, nil
		}
	}
	return m, nil
}

// save applies the edited values to the task and saves.
func (m *ScoreEditorModel) save() tea.Cmd {
	if m.task.ScoreDimensions == nil {
		m.task.ScoreDimensions = make(map[string]int)
	}

	for i, dim := range m.profile.Dimensions {
		if m.sources[i] == "manual" {
			m.task.ScoreDimensions[dim.ID] = m.values[i]
		}
	}

	if err := storage.SaveProject(m.project); err != nil {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
	}

	return func() tea.Msg {
		return ScoreEditorDoneMsg{Task: m.task, Saved: true, Project: m.project}
	}
}

// computeLiveScore calculates the score from the editor's in-progress values.
func (m *ScoreEditorModel) computeLiveScore() float64 {
	results := make([]scoring.DimensionResult, len(m.profile.Dimensions))
	for i, dim := range m.profile.Dimensions {
		results[i] = scoring.DimensionResult{
			DimensionID:   dim.ID,
			DimensionName: dim.Name,
			Value:         m.values[i],
			Source:        m.sources[i],
			Weight:        dim.Weight,
			Role:          dim.Role,
		}
	}
	return scoring.ApplyFormula(results, m.profile.Formula)
}

// View implements tea.Model.
func (m *ScoreEditorModel) View() string {
	// Compute live score from in-progress editor values
	totalScore := m.computeLiveScore()

	// Content width for wrapping (box border=2, padding=4, margin=4)
	contentWidth := m.width - 10
	if contentWidth < 30 {
		contentWidth = 30
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	headerStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle()
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	scoreStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))

	var lines []string
	titleText := fmt.Sprintf("Score Editor: #%d %s", m.task.ShortID, m.task.Title)
	lines = append(lines, titleStyle.Width(contentWidth).Render(titleText))
	lines = append(lines, headerStyle.Render(fmt.Sprintf("Profile: %s (%s)", m.profile.Name, m.profile.Formula)))
	lines = append(lines, "")

	for i, dim := range m.profile.Dimensions {
		cursor := "  "
		style := dimStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		// Value bar
		barLen := dim.Max - dim.Min + 1
		bar := ""
		for v := dim.Min; v <= dim.Max; v++ {
			if v == m.values[i] {
				bar += "█"
			} else {
				bar += "░"
			}
		}
		_ = barLen

		roleStr := ""
		if dim.Role == scoring.RoleNumerator {
			roleStr = " [num]"
		} else if dim.Role == scoring.RoleDenominator {
			roleStr = " [den]"
		}

		sourceTag := mutedStyle.Render(fmt.Sprintf("(%s)", m.sources[i]))

		line := fmt.Sprintf("%s%-20s %s %d/%d  %s%s",
			cursor,
			dim.Name+roleStr,
			bar,
			m.values[i], dim.Max,
			sourceTag,
			"",
		)
		lines = append(lines, style.Render(line))
	}

	lines = append(lines, "")
	lines = append(lines, scoreStyle.Render(fmt.Sprintf("Total Score: %d", int(totalScore))))
	lines = append(lines, "")
	helpText := "j/k: navigate · h/l: adjust · 0-9: set · d: reset · Enter: save · Esc: cancel"
	lines = append(lines, mutedStyle.Render(renderWrappingHelpLine(helpText, contentWidth)))

	content := ""
	for i, l := range lines {
		if i > 0 {
			content += "\n"
		}
		content += l
	}

	// Render as a centered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(1, 2)

	box := boxStyle.Render(content)

	// Center the box in the terminal
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box)
}

// ShouldCaptureEsc returns true since the editor handles Esc internally.
func (m *ScoreEditorModel) ShouldCaptureEsc() bool {
	return true
}

// HasTextInput returns false since there's no free-form text input.
func (m *ScoreEditorModel) HasTextInput() bool {
	return false
}
