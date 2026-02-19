package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var activityTabNames = []string{"All", "Status", "Edits", "Comments"}

const (
	activityTabAll      = 0
	activityTabStatus   = 1
	activityTabEdits    = 2
	activityTabComments = 3
)

// TaskActivityModel displays the activity log for a task.
type TaskActivityModel struct {
	task    *models.Task
	project *models.Project

	activeTab int
	cursor    int
	sortAsc   bool
	width     int
	height    int
}

// NewTaskActivityModel creates a new task activity viewer.
func NewTaskActivityModel(task *models.Task, project *models.Project) *TaskActivityModel {
	return &TaskActivityModel{
		task:    task,
		project: project,
	}
}

func (m *TaskActivityModel) Init() tea.Cmd {
	return nil
}

func (m *TaskActivityModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *TaskActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseTaskActivityMsg{} }

		case "j", "down":
			entries := m.getEntries()
			if m.cursor < len(entries)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

		case "G":
			entries := m.getEntries()
			if len(entries) > 0 {
				m.cursor = len(entries) - 1
			}

		case "[", "h":
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(activityTabNames) - 1
			}
			m.cursor = 0

		case "]", "l":
			if m.activeTab < len(activityTabNames)-1 {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			m.cursor = 0

		case "s":
			m.sortAsc = !m.sortAsc
			m.cursor = 0

		case "y":
			// Copy current entry as text
			entries := m.getEntries()
			if m.cursor < len(entries) {
				entry := entries[m.cursor]
				text := formatEntryText(entry)
				return m, func() tea.Msg {
					return CopyTaskTitleMsg{Task: &models.Task{Title: text}}
				}
			}
		}
	}

	return m, nil
}

func (m *TaskActivityModel) View() string {
	var b strings.Builder

	entries := m.getEntries()
	sortDir := "newest first"
	if m.sortAsc {
		sortDir = "oldest first"
	}
	subtitle := fmt.Sprintf("%d entries (%s)", len(entries), sortDir)
	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Activity Log for #%d: %s", m.task.ShortID, m.task.Title), subtitle))
	b.WriteString("\n")

	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	if len(entries) == 0 {
		b.WriteString(MutedStyle.Render("No activity recorded yet."))
		b.WriteString("\n")
	} else {
		headerLines := 5
		availableHeight := m.height - headerLines
		maxVisible := availableHeight / 2
		if maxVisible < 3 {
			maxVisible = 3
		}

		scrollOffset := calcScrollOffset(m.cursor, maxVisible)

		for i := scrollOffset; i < len(entries) && i < scrollOffset+maxVisible; i++ {
			entry := entries[i]

			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			relTime := storage.FormatRelativeTime(entry.Timestamp)
			ts := fmt.Sprintf("[%s] %s", entry.Timestamp.Format("2006-01-02 15:04"), relTime)

			var line strings.Builder
			line.WriteString(cursor)

			fieldStyle := fieldColorStyle(entry.Field)

			if i == m.cursor {
				line.WriteString(fieldStyle.Render(formatEntryText(entry)))
				line.WriteString("  ")
				line.WriteString(MutedStyle.Render(ts))
			} else {
				line.WriteString(MutedStyle.Render(formatEntryText(entry)))
				line.WriteString("  ")
				line.WriteString(MutedStyle.Render(ts))
			}

			b.WriteString(line.String())
			b.WriteString("\n")
		}

		if len(entries) > maxVisible {
			remaining := len(entries) - scrollOffset - maxVisible
			if remaining > 0 {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
				b.WriteString("\n")
			}
		}
	}

	helpText := "[/]: tabs · j/k: navigate · s: sort · y: copy entry · q/Esc: close"
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, m.width)))

	return b.String()
}

// getEntries returns entries in the current sort order, filtered by active tab.
// Default (sortAsc=false) is reverse chronological (newest first).
func (m *TaskActivityModel) getEntries() []*models.ActivityEntry {
	var source []*models.ActivityEntry
	if m.sortAsc {
		source = make([]*models.ActivityEntry, len(m.task.Activity))
		for i, e := range m.task.Activity {
			source[len(m.task.Activity)-1-i] = e
		}
	} else {
		source = make([]*models.ActivityEntry, len(m.task.Activity))
		copy(source, m.task.Activity)
	}

	// Apply tab filter
	if m.activeTab == activityTabAll {
		return source
	}

	var filtered []*models.ActivityEntry
	for _, e := range source {
		switch m.activeTab {
		case activityTabStatus:
			if e.Field == "status" || e.Field == "created" {
				filtered = append(filtered, e)
			}
		case activityTabEdits:
			if e.Field != "status" && e.Field != "created" && e.Field != "comment" {
				filtered = append(filtered, e)
			}
		case activityTabComments:
			if e.Field == "comment" {
				filtered = append(filtered, e)
			}
		}
	}
	return filtered
}

// renderTabBar renders the tab bar for filtering.
func (m *TaskActivityModel) renderTabBar() string {
	return RenderTabBar(activityTabNames, m.activeTab, min(m.width-6, 54)-2, 1)
}

// formatEntryText formats an activity entry as a human-readable string.
func formatEntryText(entry *models.ActivityEntry) string {
	if entry.Field == "created" {
		return "created"
	}
	if entry.OldValue == "" {
		return fmt.Sprintf("%s: %s", entry.Field, entry.NewValue)
	}
	return fmt.Sprintf("%s: %s > %s", entry.Field, entry.OldValue, entry.NewValue)
}

// fieldColorStyle returns a lipgloss style for the given field type.
func fieldColorStyle(field string) lipgloss.Style {
	switch field {
	case "status":
		return SuccessStyle
	case "priority":
		return makeColorStyle(ColorWarning)
	case "section":
		return makeColorStyle(ColorSecondary)
	case "tags":
		return PurpleStyle
	case "due":
		return makeColorStyle(ColorError)
	case "title":
		return makeColorStyle(ColorPrimary)
	case "created":
		return MutedStyle
	default:
		return lipgloss.NewStyle()
	}
}

// HasTextInput returns false - no text input in activity view.
func (m *TaskActivityModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns false - esc should navigate back.
func (m *TaskActivityModel) ShouldCaptureEsc() bool {
	return false
}
