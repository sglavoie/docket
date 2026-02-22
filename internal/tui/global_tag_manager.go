package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TagProjectUsage tracks how many tasks in a project use a specific tag.
type TagProjectUsage struct {
	Project   *models.Project
	TaskCount int
}

// GlobalTagInfo holds a tag name, its total usage count, and per-project breakdown.
type GlobalTagInfo struct {
	Name      string
	TaskCount int // Total across all projects
	Projects  []TagProjectUsage
}

// GlobalTagManagerModel is the model for the global tag management view.
type GlobalTagManagerModel struct {
	config *config.Config
	tags   []GlobalTagInfo
	cursor int
	width  int
	height int

	// Rename state
	renameMode   bool
	renameInput  textinput.Model
	renameTarget string
	errorMsg     string

	// Project picker state
	showProjectPicker bool
	projectCursor     int
	selectedTag       string
}

// ShowGlobalTagManagerMsg is sent to open the global tag manager view.
type ShowGlobalTagManagerMsg struct{}

// CloseGlobalTagManagerMsg is sent when the global tag manager should close.
type CloseGlobalTagManagerMsg struct{}

// GlobalTagRenamedMsg is sent after a tag has been renamed across projects.
type GlobalTagRenamedMsg struct {
	OldName      string
	NewName      string
	ProjectCount int
	TaskCount    int
}

// GlobalTagNavigateMsg is sent when navigating to a project with a tag filter.
type GlobalTagNavigateMsg struct {
	Project *models.Project
	Tag     string
}

// NewGlobalTagManagerModel creates a new global tag manager model.
func NewGlobalTagManagerModel(cfg *config.Config) *GlobalTagManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new tag name"
	renameInput.CharLimit = 50
	renameInput.Width = 30

	m := &GlobalTagManagerModel{
		config:      cfg,
		cursor:      0,
		renameInput: renameInput,
	}
	m.buildGlobalTagList()
	return m
}

// SetSize sets the view dimensions.
func (m *GlobalTagManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildGlobalTagList aggregates tags from all active projects.
func (m *GlobalTagManagerModel) buildGlobalTagList() {
	tagMap := make(map[string]*GlobalTagInfo)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		// Count tags in this project
		projectTagCounts := make(map[string]int)
		for _, task := range project.Tasks {
			for _, tag := range task.Tags {
				projectTagCounts[tag]++
			}
		}

		// Merge into global map
		for tag, count := range projectTagCounts {
			if info, exists := tagMap[tag]; exists {
				info.TaskCount += count
				info.Projects = append(info.Projects, TagProjectUsage{
					Project:   project,
					TaskCount: count,
				})
			} else {
				tagMap[tag] = &GlobalTagInfo{
					Name:      tag,
					TaskCount: count,
					Projects: []TagProjectUsage{{
						Project:   project,
						TaskCount: count,
					}},
				}
			}
		}
	}

	// Convert map to sorted slice
	m.tags = make([]GlobalTagInfo, 0, len(tagMap))
	for _, info := range tagMap {
		m.tags = append(m.tags, *info)
	}

	sort.Slice(m.tags, func(i, j int) bool {
		return m.tags[i].Name < m.tags[j].Name
	})

	// Clamp cursor
	if m.cursor >= len(m.tags) {
		m.cursor = len(m.tags) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// renameGlobalTag renames a tag across all projects that use it.
func (m *GlobalTagManagerModel) renameGlobalTag(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		newName = strings.TrimSpace(newName)
		if newName == "" || newName == oldName {
			return nil
		}

		// Find the tag info
		var tagInfo *GlobalTagInfo
		for i := range m.tags {
			if m.tags[i].Name == oldName {
				tagInfo = &m.tags[i]
				break
			}
		}
		if tagInfo == nil {
			return nil
		}

		projectCount := 0
		taskCount := 0

		// Rename in each project that uses this tag
		for _, usage := range tagInfo.Projects {
			project := usage.Project

			// Reload project to ensure fresh data
			if err := storage.LoadProject(project); err != nil {
				continue
			}

			// Track if we made any changes
			modified := false
			for _, task := range project.Tasks {
				for i, tag := range task.Tags {
					if tag == oldName {
						task.Tags[i] = newName
						modified = true
						taskCount++
					}
				}
			}

			if modified {
				if err := storage.BackupProject(project); err != nil {
					return ErrorMsg{Err: fmt.Errorf("failed to backup %s: %w", project.Name, err)}
				}
				if err := storage.SaveProject(project); err != nil {
					return ErrorMsg{Err: fmt.Errorf("failed to save %s: %w", project.Name, err)}
				}
				projectCount++
			}
		}

		return GlobalTagRenamedMsg{
			OldName:      oldName,
			NewName:      newName,
			ProjectCount: projectCount,
			TaskCount:    taskCount,
		}
	}
}

// Init initializes the model.
func (m *GlobalTagManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *GlobalTagManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case GlobalTagRenamedMsg:
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, func() tea.Msg {
			return CloseGlobalTagManagerMsg{}
		}

	case tea.KeyMsg:
		m.errorMsg = ""

		if m.renameMode {
			return m.handleRenameMode(msg)
		}

		if m.showProjectPicker {
			return m.handleProjectPickerMode(msg)
		}

		return m.handleNormalMode(msg)
	}

	return m, nil
}

// handleNormalMode handles keyboard input in normal mode.
func (m *GlobalTagManagerModel) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+t":
		return m, func() tea.Msg {
			return CloseGlobalTagManagerMsg{}
		}

	case "j", "down":
		if len(m.tags) > 0 {
			m.cursor++
			if m.cursor >= len(m.tags) {
				m.cursor = 0
			}
		}
		return m, nil

	case "k", "up":
		if len(m.tags) > 0 {
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.tags) - 1
			}
		}
		return m, nil

	case "enter":
		if len(m.tags) > 0 && m.cursor < len(m.tags) {
			tag := m.tags[m.cursor]
			if len(tag.Projects) == 1 {
				// Single project, navigate directly
				return m, func() tea.Msg {
					return GlobalTagNavigateMsg{
						Project: tag.Projects[0].Project,
						Tag:     tag.Name,
					}
				}
			}
			// Multiple projects, show picker
			m.showProjectPicker = true
			m.projectCursor = 0
			m.selectedTag = tag.Name
		}
		return m, nil

	case "r":
		if len(m.tags) > 0 && m.cursor < len(m.tags) {
			m.renameMode = true
			m.renameTarget = m.tags[m.cursor].Name
			m.renameInput.SetValue(m.renameTarget)
			m.renameInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	}

	return m, nil
}

// handleRenameMode handles keyboard input while in rename mode.
func (m *GlobalTagManagerModel) handleRenameMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	case "enter":
		newName := strings.TrimSpace(m.renameInput.Value())
		if newName != "" && newName != m.renameTarget {
			return m, m.renameGlobalTag(m.renameTarget, newName)
		}
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// handleProjectPickerMode handles keyboard input in project picker mode.
func (m *GlobalTagManagerModel) handleProjectPickerMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Find the current tag's projects
	var projects []TagProjectUsage
	for _, tag := range m.tags {
		if tag.Name == m.selectedTag {
			projects = tag.Projects
			break
		}
	}

	switch msg.String() {
	case "esc":
		m.showProjectPicker = false
		m.projectCursor = 0
		m.selectedTag = ""
		return m, nil

	case "j", "down":
		if len(projects) > 0 {
			m.projectCursor++
			if m.projectCursor >= len(projects) {
				m.projectCursor = 0
			}
		}
		return m, nil

	case "k", "up":
		if len(projects) > 0 {
			m.projectCursor--
			if m.projectCursor < 0 {
				m.projectCursor = len(projects) - 1
			}
		}
		return m, nil

	case "enter":
		if len(projects) > 0 && m.projectCursor < len(projects) {
			project := projects[m.projectCursor].Project
			tag := m.selectedTag
			return m, func() tea.Msg {
				return GlobalTagNavigateMsg{
					Project: project,
					Tag:     tag,
				}
			}
		}
		return m, nil
	}

	return m, nil
}

// View renders the global tag manager view.
func (m *GlobalTagManagerModel) View() string {
	var b strings.Builder

	if m.showProjectPicker {
		return m.renderProjectPicker()
	}

	b.WriteString(renderFullscreenTitle("Global Tag Manager", "across all projects"))
	b.WriteString("\n\n")

	if len(m.tags) == 0 {
		b.WriteString(MutedStyle.Render("No tags found in any project"))
		b.WriteString("\n")
	} else {
		for i, tag := range m.tags {
			cursor := "  "
			style := TagManagerItemStyle
			if i == m.cursor {
				cursor = "> "
				style = TagManagerItemSelectedStyle
			}

			if m.renameMode && tag.Name == m.renameTarget {
				b.WriteString(cursor)
				b.WriteString("Rename to: ")
				b.WriteString(m.renameInput.View())
				b.WriteString("\n")
			} else {
				line := fmt.Sprintf("%s%s ", cursor, tag.Name)
				b.WriteString(style.Render(line))
				countStr := fmt.Sprintf("(%d)", tag.TaskCount)
				b.WriteString(TagManagerCountStyle.Render(countStr))
				projectStr := fmt.Sprintf(" [%d project", len(tag.Projects))
				if len(tag.Projects) != 1 {
					projectStr += "s"
				}
				projectStr += "]"
				b.WriteString(MutedStyle.Render(projectStr))
				b.WriteString("\n")
			}
		}
	}

	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(TagManagerErrorStyle.Render(m.errorMsg))
		b.WriteString("\n")
	}

	var footerHelp string
	if m.renameMode {
		footerHelp = "enter confirm · esc cancel"
	} else if len(m.tags) > 0 {
		footerHelp = "j/k nav · enter filter · r rename · q close"
	} else {
		footerHelp = "q close"
	}
	b.WriteString(renderStatusBarFooter(HelpStyle.Render(footerHelp), "", m.width))

	return b.String()
}

// renderProjectPicker renders the project picker overlay.
func (m *GlobalTagManagerModel) renderProjectPicker() string {
	var b strings.Builder

	var projects []TagProjectUsage
	for _, tag := range m.tags {
		if tag.Name == m.selectedTag {
			projects = tag.Projects
			break
		}
	}

	b.WriteString(renderFullscreenTitle("Select Project", fmt.Sprintf("for tag \"%s\"", m.selectedTag)))
	b.WriteString("\n\n")

	for i, usage := range projects {
		cursor := "  "
		style := TagManagerItemStyle
		if i == m.projectCursor {
			cursor = "> "
			style = TagManagerItemSelectedStyle
		}

		line := fmt.Sprintf("%s%s ", cursor, usage.Project.Name)
		b.WriteString(style.Render(line))
		countStr := fmt.Sprintf("(%d tasks)", usage.TaskCount)
		b.WriteString(TagManagerCountStyle.Render(countStr))
		b.WriteString("\n")
	}

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("j/k nav · enter select · esc back"), "", m.width))

	return b.String()
}

func (m *GlobalTagManagerModel) StatusBarLabel() string { return "Global Tags" }
