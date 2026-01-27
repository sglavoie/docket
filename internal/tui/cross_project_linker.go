package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// CrossProjectLinkerStep represents the current step in the linking process.
type CrossProjectLinkerStep int

const (
	StepSelectProject CrossProjectLinkerStep = iota
	StepSelectTask
	StepSelectLinkType
)

// CrossProjectLinkerModel handles cross-project task linking.
type CrossProjectLinkerModel struct {
	BaseSearchableView
	sourceTask    *models.Task
	sourceProject *models.Project
	config        *config.Config

	step CrossProjectLinkerStep

	// Project selection
	allProjects      []*models.Project
	filteredProjects []*models.Project
	projectCursor    int

	// Task selection (after project is selected)
	targetProject *models.Project
	allTasks      []*models.Task
	filteredTasks []*models.Task
	taskCursor    int

	// Link type selection
	targetTask     *models.Task
	linkTypeCursor int

	err error
}

// NewCrossProjectLinkerModel creates a new cross-project linker.
func NewCrossProjectLinkerModel(sourceTask *models.Task, sourceProject *models.Project, cfg *config.Config) *CrossProjectLinkerModel {
	// Get all active projects except the source
	var projects []*models.Project
	for _, p := range cfg.GetActiveProjects() {
		if p.ID != sourceProject.ID {
			projects = append(projects, p)
		}
	}

	m := &CrossProjectLinkerModel{
		sourceTask:       sourceTask,
		sourceProject:    sourceProject,
		config:           cfg,
		step:             StepSelectProject,
		allProjects:      projects,
		filteredProjects: projects,
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search...", 50, 30)
	return m
}

// SetSize sets the view dimensions.
func (m *CrossProjectLinkerModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *CrossProjectLinkerModel) Init() tea.Cmd {
	return nil
}

func (m *CrossProjectLinkerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeypress(msg)
	}
	return m, nil
}

func (m *CrossProjectLinkerModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle search mode input first (only in project/task selection steps)
	if m.IsSearchMode() && (m.step == StepSelectProject || m.step == StepSelectTask) {
		handled, cmd, valueChanged := m.HandleSearchInput(msg)
		if valueChanged {
			m.applySearchFilter()
		}
		if handled {
			return m, cmd
		}
		// Pass through for navigation while in search mode
	}

	switch key {
	case "/":
		// Activate search mode (only in project/task selection)
		if m.step == StepSelectProject || m.step == StepSelectTask {
			return m, m.ActivateSearch()
		}
		return m, nil

	case "esc":
		// If there's a search query, clear it first
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.applySearchFilter()
			return m, nil
		}
		// Go back one step or close
		switch m.step {
		case StepSelectProject:
			return m, func() tea.Msg { return CloseCrossProjectLinkerMsg{} }
		case StepSelectTask:
			m.step = StepSelectProject
			m.targetProject = nil
			m.allTasks = nil
			m.filteredTasks = nil
			m.ClearSearch()
			return m, nil
		case StepSelectLinkType:
			m.step = StepSelectTask
			m.targetTask = nil
			return m, nil
		}
		return m, nil

	case "q":
		// Close the linker (but not while in search mode)
		if !m.IsSearchMode() {
			return m, func() tea.Msg { return CloseCrossProjectLinkerMsg{} }
		}
		return m, nil

	case "enter":
		return m.handleEnter()

	case "up", "k":
		if !m.IsSearchMode() {
			m.moveCursor(-1)
		}
		return m, nil

	case "down", "j":
		if !m.IsSearchMode() {
			m.moveCursor(1)
		}
		return m, nil

	case "g":
		if !m.IsSearchMode() {
			m.goToFirst()
		}
		return m, nil

	case "G":
		if !m.IsSearchMode() {
			m.goToLast()
		}
		return m, nil
	}

	return m, nil
}

func (m *CrossProjectLinkerModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepSelectProject:
		if len(m.filteredProjects) == 0 {
			return m, nil
		}
		m.targetProject = m.filteredProjects[m.projectCursor]
		// Load the target project's tasks
		if err := storage.LoadProject(m.targetProject); err != nil {
			m.err = fmt.Errorf("failed to load project: %w", err)
			return m, nil
		}
		m.allTasks = m.getSelectableTasks()
		m.filteredTasks = m.allTasks
		m.taskCursor = 0
		m.ClearSearch() // Clear search for next step
		m.step = StepSelectTask
		return m, nil

	case StepSelectTask:
		if len(m.filteredTasks) == 0 {
			return m, nil
		}
		m.targetTask = m.filteredTasks[m.taskCursor]
		m.ClearSearch() // Clear search for next step
		m.step = StepSelectLinkType
		m.linkTypeCursor = 0
		return m, nil

	case StepSelectLinkType:
		return m, m.createLink()
	}
	return m, nil
}

// applySearchFilter applies the current search query to the appropriate list.
func (m *CrossProjectLinkerModel) applySearchFilter() {
	switch m.step {
	case StepSelectProject:
		m.filterProjects()
	case StepSelectTask:
		m.filterTasks()
	}
}

// goToFirst moves cursor to the first item.
func (m *CrossProjectLinkerModel) goToFirst() {
	switch m.step {
	case StepSelectProject:
		m.projectCursor = 0
	case StepSelectTask:
		m.taskCursor = 0
	case StepSelectLinkType:
		m.linkTypeCursor = 0
	}
}

// goToLast moves cursor to the last item.
func (m *CrossProjectLinkerModel) goToLast() {
	switch m.step {
	case StepSelectProject:
		if len(m.filteredProjects) > 0 {
			m.projectCursor = len(m.filteredProjects) - 1
		}
	case StepSelectTask:
		if len(m.filteredTasks) > 0 {
			m.taskCursor = len(m.filteredTasks) - 1
		}
	case StepSelectLinkType:
		m.linkTypeCursor = 2
	}
}

func (m *CrossProjectLinkerModel) moveCursor(delta int) {
	switch m.step {
	case StepSelectProject:
		maxCursor := len(m.filteredProjects) - 1
		if maxCursor < 0 {
			maxCursor = 0
		}
		m.projectCursor = clamp(m.projectCursor+delta, 0, maxCursor)

	case StepSelectTask:
		maxCursor := len(m.filteredTasks) - 1
		if maxCursor < 0 {
			maxCursor = 0
		}
		m.taskCursor = clamp(m.taskCursor+delta, 0, maxCursor)

	case StepSelectLinkType:
		// 3 link types: Blocks, Blocked By, Related
		m.linkTypeCursor = clamp(m.linkTypeCursor+delta, 0, 2)
	}
}

func (m *CrossProjectLinkerModel) filterProjects() {
	query := strings.TrimSpace(strings.ToLower(m.SearchQuery()))
	if query == "" {
		m.filteredProjects = m.allProjects
		m.projectCursor = clamp(m.projectCursor, 0, len(m.filteredProjects)-1)
		return
	}

	type scoredProject struct {
		project *models.Project
		score   int
	}
	var scored []scoredProject

	for _, p := range m.allProjects {
		searchText := strings.ToLower(p.Name + " " + p.Context)
		score := fuzzy.ScoreTerms(searchText, query)
		if score > 0 {
			scored = append(scored, scoredProject{p, score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filteredProjects = make([]*models.Project, len(scored))
	for i, s := range scored {
		m.filteredProjects[i] = s.project
	}
	m.projectCursor = 0
}

func (m *CrossProjectLinkerModel) filterTasks() {
	query := strings.TrimSpace(strings.ToLower(m.SearchQuery()))
	if query == "" {
		m.filteredTasks = m.allTasks
		m.taskCursor = clamp(m.taskCursor, 0, len(m.filteredTasks)-1)
		return
	}

	type scoredTask struct {
		task  *models.Task
		score int
	}
	var scored []scoredTask

	for _, t := range m.allTasks {
		searchText := strings.ToLower(t.Title + " " + fmt.Sprintf("#%d", t.ShortID))
		score := fuzzy.ScoreTerms(searchText, query)
		if score > 0 {
			scored = append(scored, scoredTask{t, score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filteredTasks = make([]*models.Task, len(scored))
	for i, s := range scored {
		m.filteredTasks[i] = s.task
	}
	m.taskCursor = 0
}

// getSelectableTasks returns tasks from target project that can be linked.
func (m *CrossProjectLinkerModel) getSelectableTasks() []*models.Task {
	var tasks []*models.Task
	for _, t := range m.targetProject.Tasks {
		// Skip notes
		if t.IsNote() {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks
}

func (m *CrossProjectLinkerModel) createLink() tea.Cmd {
	return func() tea.Msg {
		// Determine link type
		var linkType LinkType
		switch m.linkTypeCursor {
		case 0:
			linkType = LinkTypeBlocker // Target blocks source
		case 1:
			linkType = LinkTypeDependent // Source blocks target (source depends on target)
		case 2:
			linkType = LinkTypeFollowUp // Related (bidirectional)
		}

		// Create qualified reference
		targetRef := m.targetProject.ID + ":" + m.targetTask.ID
		sourceRef := m.sourceProject.ID + ":" + m.sourceTask.ID

		// Establish the relationship
		switch linkType {
		case LinkTypeBlocker:
			// Target blocks source: source depends on target
			m.sourceTask.AddDependency(targetRef)
		case LinkTypeDependent:
			// Source blocks target: target depends on source
			m.targetTask.AddDependency(sourceRef)
		case LinkTypeFollowUp:
			// Bidirectional relationship
			m.sourceTask.AddRelatedTo(targetRef)
			m.targetTask.AddRelatedTo(sourceRef)
		}

		// Update timestamps
		m.sourceTask.UpdatedAt = time.Now()
		m.targetTask.UpdatedAt = time.Now()

		// Backup and save source project
		if err := storage.BackupProject(m.sourceProject); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup source project: %w", err)}
		}
		if err := storage.SaveProject(m.sourceProject); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save source project: %w", err)}
		}

		// Backup and save target project
		if err := storage.BackupProject(m.targetProject); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup target project: %w", err)}
		}
		if err := storage.SaveProject(m.targetProject); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save target project: %w", err)}
		}

		return CrossProjectLinkCreatedMsg{
			SourceTask:    m.sourceTask,
			SourceProject: m.sourceProject,
			TargetTask:    m.targetTask,
			TargetProject: m.targetProject,
			LinkType:      linkType,
		}
	}
}

func (m *CrossProjectLinkerModel) View() string {
	var b strings.Builder

	switch m.step {
	case StepSelectProject:
		b.WriteString(m.viewProjectSelection())
	case StepSelectTask:
		b.WriteString(m.viewTaskSelection())
	case StepSelectLinkType:
		b.WriteString(m.viewLinkTypeSelection())
	}

	cardWidth := m.Width() - 4
	if cardWidth > 70 {
		cardWidth = 70
	}

	return renderOverlayCard(b.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    m.getStepTitle(),
		Subtitle: fmt.Sprintf("#%d %s", m.sourceTask.ShortID, truncateTitle(m.sourceTask.Title, 30)),
		Width:    cardWidth,
		Centered: true,
	})
}

func (m *CrossProjectLinkerModel) getStepTitle() string {
	switch m.step {
	case StepSelectProject:
		return "Select Project (1/3)"
	case StepSelectTask:
		return "Select Task (2/3)"
	case StepSelectLinkType:
		return "Select Link Type (3/3)"
	}
	return "Cross-Project Link"
}

func (m *CrossProjectLinkerModel) viewProjectSelection() string {
	var b strings.Builder

	// Show search input when active
	if m.IsSearchMode() {
		b.WriteString("Search: " + m.SearchInputView())
		b.WriteString("\n\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filteredProjects))))
		b.WriteString("\n\n")
	}

	if len(m.filteredProjects) == 0 {
		if len(m.allProjects) == 0 {
			b.WriteString(MutedStyle.Render("No other projects available."))
		} else {
			b.WriteString(MutedStyle.Render("No matching projects."))
		}
		b.WriteString("\n\n")
		if m.SearchQuery() != "" {
			b.WriteString(HelpStyle.Render("Esc clear filter • q close"))
		} else {
			b.WriteString(HelpStyle.Render("Esc cancel"))
		}
		return b.String()
	}

	maxVisible := 8
	scrollOffset := 0
	if m.projectCursor >= maxVisible {
		scrollOffset = m.projectCursor - maxVisible + 1
	}

	for i, p := range m.filteredProjects {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}
		cursor := "  "
		if i == m.projectCursor {
			cursor = "> "
		}
		contextBadge := MutedStyle.Render(fmt.Sprintf("[%s]", p.Context))
		line := fmt.Sprintf("%s%s %s", cursor, p.Name, contextBadge)
		if i == m.projectCursor {
			line = BoldStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.filteredProjects) > maxVisible {
		remaining := len(m.filteredProjects) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.IsSearchMode() {
		b.WriteString(HelpStyle.Render("Type to filter • Enter confirm • Esc cancel"))
	} else {
		b.WriteString(HelpStyle.Render("/ search • j/k navigate • Enter select • Esc cancel"))
	}

	return b.String()
}

func (m *CrossProjectLinkerModel) viewTaskSelection() string {
	var b strings.Builder

	// Show selected project
	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Project: %s", m.targetProject.Name)))
	b.WriteString("\n\n")

	// Show search input when active
	if m.IsSearchMode() {
		b.WriteString("Search: " + m.SearchInputView())
		b.WriteString("\n\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filteredTasks))))
		b.WriteString("\n\n")
	}

	if len(m.filteredTasks) == 0 {
		if len(m.allTasks) == 0 {
			b.WriteString(MutedStyle.Render("No tasks in this project."))
		} else {
			b.WriteString(MutedStyle.Render("No matching tasks."))
		}
		b.WriteString("\n\n")
		if m.SearchQuery() != "" {
			b.WriteString(HelpStyle.Render("Esc clear filter • q close"))
		} else {
			b.WriteString(HelpStyle.Render("Esc go back"))
		}
		return b.String()
	}

	maxVisible := 8
	scrollOffset := 0
	if m.taskCursor >= maxVisible {
		scrollOffset = m.taskCursor - maxVisible + 1
	}

	for i, t := range m.filteredTasks {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}
		cursor := "  "
		if i == m.taskCursor {
			cursor = "> "
		}
		statusIcon := GetStatusBadge(string(t.Status), t.Status.Icon())
		typeIcon := GetTypeBadge(string(t.Type), t.Type.Icon())
		title := truncateTitle(t.Title, m.Width()-35)
		line := fmt.Sprintf("%s%s %s #%d %s", cursor, statusIcon, typeIcon, t.ShortID, title)
		if i == m.taskCursor {
			line = BoldStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.filteredTasks) > maxVisible {
		remaining := len(m.filteredTasks) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.IsSearchMode() {
		b.WriteString(HelpStyle.Render("Type to filter • Enter confirm • Esc cancel"))
	} else {
		b.WriteString(HelpStyle.Render("/ search • j/k navigate • Enter select • Esc back"))
	}

	return b.String()
}

func (m *CrossProjectLinkerModel) viewLinkTypeSelection() string {
	var b strings.Builder

	// Show selected project and task
	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Project: %s", m.targetProject.Name)))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Task: #%d %s", m.targetTask.ShortID, truncateTitle(m.targetTask.Title, 40))))
	b.WriteString("\n\n")

	linkTypes := []struct {
		name string
		desc string
	}{
		{"Blocks", fmt.Sprintf("#%d blocks #%d (source depends on target)", m.targetTask.ShortID, m.sourceTask.ShortID)},
		{"Blocked By", fmt.Sprintf("#%d depends on #%d (target depends on source)", m.targetTask.ShortID, m.sourceTask.ShortID)},
		{"Related", fmt.Sprintf("#%d ↔ #%d (bidirectional)", m.sourceTask.ShortID, m.targetTask.ShortID)},
	}

	for i, lt := range linkTypes {
		cursor := "  "
		if i == m.linkTypeCursor {
			cursor = "> "
		}
		line := fmt.Sprintf("%s%s", cursor, lt.name)
		if i == m.linkTypeCursor {
			line = BoldStyle.Render(line)
			b.WriteString(line)
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render("    " + lt.desc))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("↑/↓ navigate • Enter create link • Esc back"))

	return b.String()
}

// HasTextInput returns true when search mode is active.
func (m *CrossProjectLinkerModel) HasTextInput() bool {
	return m.IsSearchMode()
}

// ShouldCaptureEsc returns true when search is active so esc clears search first.
func (m *CrossProjectLinkerModel) ShouldCaptureEsc() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// clamp restricts a value to the given range.
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if max >= 0 && value > max {
		return max
	}
	return value
}
