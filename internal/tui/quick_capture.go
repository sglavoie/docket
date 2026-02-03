package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// QuickCaptureModel handles rapid task creation with inline metadata parsing.
type QuickCaptureModel struct {
	quickCaptureInput
	width  int
	height int
}

// NewQuickCaptureModel creates a new quick capture model.
func NewQuickCaptureModel(project *models.Project, allProjects []*models.Project) *QuickCaptureModel {
	m := &QuickCaptureModel{
		quickCaptureInput: newQuickCaptureInput(
			project,
			allProjects,
			"^type Task title !priority #tag @due ~recur >parent /section $status %project",
		),
		width: 80,
	}
	return m
}

// SetSize sets the view dimensions.
func (m *QuickCaptureModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.quickCaptureInput.width = width
	m.resizeTextarea(width)
}

func (m *QuickCaptureModel) Init() tea.Cmd {
	return nil
}

func (m *QuickCaptureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle autocomplete navigation when active
		if m.handleAutocompleteKey(key) {
			return m, nil
		}

		// Regular key handling
		switch key {
		case "esc":
			return m, func() tea.Msg { return CloseQuickCaptureMsg{} }

		case "enter":
			// Only create task if not in autocomplete mode
			if m.acMode == acModeNone {
				return m, m.createTask()
			}
		}
	}

	// Update textarea and refresh autocomplete/preview
	cmd := m.updateInput(msg)
	return m, cmd
}

func (m *QuickCaptureModel) View() string {
	var b strings.Builder
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Show autocomplete suggestions if active
	if ac := m.renderAutocomplete(); ac != "" {
		b.WriteString(ac)
	}

	b.WriteString("\n")

	// Show live preview or error
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	} else if preview := m.renderPreview(); preview != "" {
		b.WriteString(preview)
		b.WriteString("\n\n")
	}

	// Help text
	b.WriteString(MutedStyle.Render("^type · !N · #tag · @date · ~recur · >parent · /section · $status · %project"))
	b.WriteString("\n")
	var helpText string
	if m.isAutocompleteActive() {
		helpText = "Tab: accept · ↑↓: navigate · Esc: dismiss"
	} else {
		helpText = "Enter: create · Esc: cancel"
	}

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Quick Capture", ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.width))

	return out.String()
}

// HasTextInput returns true because quick capture uses a textarea input.
func (m *QuickCaptureModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns false so the app can close the view on esc.
func (m *QuickCaptureModel) ShouldCaptureEsc() bool {
	return false
}

func (m *QuickCaptureModel) createTask() tea.Cmd {
	return func() tea.Msg {
		input := m.inputValue()
		if input == "" {
			m.err = fmt.Errorf("task title cannot be empty")
			return nil
		}

		result, err := parseQuickCapture(input, m.project, m.allProjects)
		if err != nil {
			m.err = err
			return nil
		}

		// Determine target project (may be different for cross-project capture)
		targetProject := m.project
		if result.targetProjectID != nil {
			// Find the target project
			for _, p := range m.allProjects {
				if p.ID == *result.targetProjectID {
					targetProject = p
					// Load the target project to get its tasks
					if err := storage.LoadProject(targetProject); err != nil {
						return ErrorMsg{Err: fmt.Errorf("failed to load target project: %w", err)}
					}
					break
				}
			}
		}

		// Determine target section
		targetSection := result.section
		if targetSection == "" {
			targetSection = models.SectionInbox
		}

		// Ensure target section exists (Inbox gets red color)
		if !targetProject.HasSection(targetSection) {
			targetProject.EnsureSection(targetSection)
			if targetSection == models.SectionInbox {
				targetProject.SetSectionColor(models.SectionInbox, "red")
			}
		}

		// Create task
		task := &models.Task{
			ID:         uuid.New().String(),
			ShortID:    targetProject.GetNextShortID(),
			Type:       result.taskType,
			Title:      result.title,
			Status:     models.StatusTodo,
			Priority:   result.priority,
			Section:    targetSection,
			ParentID:   result.parentID,
			DueDate:    result.dueDate,
			Recurrence: result.recurrence,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		for _, tag := range result.tags {
			task.AddTag(tag)
		}

		// Apply status transition using Task methods (handles auto-triage)
		switch result.status {
		case models.StatusInProgress:
			task.Start()
			if result.activate {
				task.Activate()
			}
		case models.StatusBlocked:
			task.Block()
		case models.StatusDone:
			task.Complete()
		case models.StatusCanceled:
			task.Cancel()
		}

		// Add to project at the beginning so it's prominently visible
		targetProject.Tasks = append([]*models.Task{task}, targetProject.Tasks...)
		targetProject.BuildTaskTree()
		targetProject.ReorderRootTasksByCompletion()

		// Backup and save
		if err := storage.BackupProject(targetProject); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		if err := storage.SaveProject(targetProject); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reload to rebuild tree properly
		_ = storage.LoadProject(targetProject)

		// Also reload the current project if it's different from the target
		// (so that the view refreshes correctly)
		if result.targetProjectID != nil && m.project.ID != *result.targetProjectID {
			_ = storage.LoadProject(m.project)
		}

		return TaskCreatedMsg{Task: task}
	}
}
