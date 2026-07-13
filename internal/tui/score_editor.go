package tui

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// ScoreEditorDoneMsg is sent when the score editor finishes (save or cancel).
type ScoreEditorDoneMsg struct {
	Task    *models.Task
	Saved   bool
	Project *models.Project
}

// ScoreEditorModel is a small overlay for editing scoring dimensions on a task.
type ScoreEditorModel struct {
	task        *models.Task
	project     *models.Project
	config      *config.Config
	profile     *scoring.ScoringProfile
	coordinator TaskItemCoordinator

	// Per-dimension editing state
	values  []int    // current value for each dimension
	sources []string // "manual", "derived", or "default" (for display)
	cursor  int      // which dimension is focused

	// Snapshot of task.ScoreDimensions at open time so that pressing 'd'
	// (reset) can recompute derived values without mutating the live task.
	// The live task is only modified on save.
	scratchDims map[string]int

	// Dimensions
	width  int
	height int
}

// NewScoreEditorModel creates a score editor for a task. The coordinator commits
// the edited dimensions through the application layer (atomic, ledger-undoable)
// on save.
func NewScoreEditorModel(task *models.Task, project *models.Project, cfg *config.Config, coordinator TaskItemCoordinator) *ScoreEditorModel {
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

	// Snapshot current ScoreDimensions so 'd' (reset) can recompute
	// without mutating the live task before save.
	scratchDims := make(map[string]int, len(task.ScoreDimensions))
	maps.Copy(scratchDims, task.ScoreDimensions)

	return &ScoreEditorModel{
		task:        task,
		project:     project,
		config:      cfg,
		profile:     profile,
		coordinator: coordinator,
		values:      values,
		sources:     sources,
		scratchDims: scratchDims,
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

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
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
			// Reset to derived/default using scratch state (don't mutate live task).
			dim := &m.profile.Dimensions[m.cursor]
			delete(m.scratchDims, dim.ID)
			// Temporarily apply scratch dims to task for recomputation, then restore.
			origDims := m.task.ScoreDimensions
			m.task.ScoreDimensions = m.scratchDims
			result := scoring.ScoreBreakdown(m.task, m.profile)
			m.task.ScoreDimensions = origDims
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

// save collects the edited dimensions and commits them through the application
// layer as one atomic, ledger-undoable ChangeSet. Only dimensions the user set
// to "manual" are persisted; a dimension reset back to derived/default is simply
// absent, so the application layer's replace semantics drops it. The commit runs
// against a fresh snapshot, so on success the shell reloads the touched project
// before rebuilding score-bearing views.
func (m *ScoreEditorModel) save() tea.Cmd {
	dims := make(map[string]int)
	for i, dim := range m.profile.Dimensions {
		if m.sources[i] == "manual" {
			dims[dim.ID] = m.values[i]
		}
	}

	project := m.project
	task := m.task
	return adaptSubviewCommit(m.coordinator.SetScore(task.ID, dims), func() tea.Msg {
		return ScoreEditorDoneMsg{Task: task, Saved: true, Project: project}
	})
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
func (m *ScoreEditorModel) View() tea.View {
	if m.task == nil {
		return tea.NewView("No task selected")
	}

	// Compute live score from in-progress editor values
	totalScore := m.computeLiveScore()

	// Content width for wrapping (box border=2, padding=4, margin=4)
	contentWidth := max(m.width-10, 30)

	titleStyle := OverlayTitleStyle
	headerStyle := BoldStyle
	dimStyle := NormalStyle
	selectedStyle := OverlaySelectedStyle
	mutedStyle := lipgloss.NewStyle().Foreground(ColorSubtleBorder)
	scoreStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)

	var lines []string
	titleText := fmt.Sprintf("Score Editor: #%d %s", m.task.ShortID, m.task.Title)
	lines = append(lines, titleStyle.Width(contentWidth).Render(titleText))
	lines = append(lines, headerStyle.Render(fmt.Sprintf("Profile: %s (%s)", m.profile.Name, m.profile.Formula)))
	lines = append(lines, "")

	for i, dim := range m.profile.Dimensions {
		cursor := "  "
		style := dimStyle
		if i == m.cursor {
			cursor = "  "
			style = selectedStyle
		}

		// Value bar
		barLen := dim.Max - dim.Min + 1
		var bar strings.Builder
		for v := dim.Min; v <= dim.Max; v++ {
			if v == m.values[i] {
				bar.WriteString("█")
			} else {
				bar.WriteString("░")
			}
		}
		_ = barLen

		roleStr := ""
		switch dim.Role {
		case scoring.RoleNumerator:
			roleStr = " [num]"
		case scoring.RoleDenominator:
			roleStr = " [den]"
		default:
		}

		sourceTag := mutedStyle.Render(fmt.Sprintf("(%s)", m.sources[i]))

		line := fmt.Sprintf("%s%-20s %s %d/%d  %s%s",
			cursor,
			dim.Name+roleStr,
			bar.String(),
			m.values[i], dim.Max,
			sourceTag,
			"",
		)
		lines = append(lines, style.Render(line))
	}

	lines = append(lines, "")
	pct := scoring.ScorePercent(totalScore, m.profile)
	lines = append(lines, scoreStyle.Render(fmt.Sprintf("Score: %d/%d (raw: %d)", int(pct), scoring.MaxNormalizedScore(m.profile), int(totalScore))))
	lines = append(lines, "")
	helpText := "j/k: navigate · h/l: adjust · 0-9: set · d: reset · Enter: save · Esc: cancel"
	lines = append(lines, mutedStyle.Render(renderWrappingHelpLine(helpText, contentWidth)))

	var content strings.Builder
	for i, l := range lines {
		if i > 0 {
			content.WriteString("\n")
		}
		content.WriteString(l)
	}

	// Render as a centered box
	boxStyle := OverlayBoxStyle

	box := boxStyle.Render(content.String())

	// Center the box in the terminal
	return tea.NewView(lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box))
}

// ShouldCaptureEsc returns true since the editor handles Esc internally.
func (m *ScoreEditorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *ScoreEditorModel) StatusBarLabel() string { return "Edit Score" }
