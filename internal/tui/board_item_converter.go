package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// boardConverterStep represents the current step in the conversion process.
type boardConverterStep int

const (
	converterStepProject boardConverterStep = iota
	converterStepParent
)

// BoardItemConverterModel handles converting a board item to a project task.
type BoardItemConverterModel struct {
	BaseSearchableView
	boardItem *models.BoardItem
	config    *config.Config

	step boardConverterStep

	// Project selection
	allProjects      []*models.Project
	filteredProjects []*models.Project
	projectCursor    int

	// Parent selection (after project is selected)
	targetProject *models.Project
	allTasks      []*models.Task
	filteredTasks []*models.Task
	taskCursor    int

	err error
}

// NewBoardItemConverterModel creates a new board item converter.
func NewBoardItemConverterModel(boardItem *models.BoardItem, cfg *config.Config) *BoardItemConverterModel {
	projects := cfg.GetActiveProjects()

	// If the board item has a project association, move that project to the top
	if boardItem.ProjectID != "" {
		for i, p := range projects {
			if p.ID == boardItem.ProjectID {
				reordered := make([]*models.Project, 0, len(projects))
				reordered = append(reordered, p)
				reordered = append(reordered, projects[:i]...)
				reordered = append(reordered, projects[i+1:]...)
				projects = reordered
				break
			}
		}
	}

	m := &BoardItemConverterModel{
		boardItem:        boardItem,
		config:           cfg,
		step:             converterStepProject,
		allProjects:      projects,
		filteredProjects: projects,
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search...", 50, 30)
	return m
}

// SetSize sets the view dimensions.
func (m *BoardItemConverterModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *BoardItemConverterModel) Init() tea.Cmd {
	return nil
}

func (m *BoardItemConverterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeypress(msg)
	}
	return m, nil
}

func (m *BoardItemConverterModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle search mode input first
	if m.IsSearchMode() {
		handled, cmd, valueChanged := m.HandleSearchInput(msg)
		if valueChanged {
			m.applySearchFilter()
		}
		if handled {
			return m, cmd
		}
	}

	switch key {
	case "/":
		return m, m.ActivateSearch()

	case "esc":
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.applySearchFilter()
			return m, nil
		}
		switch m.step {
		case converterStepProject:
			return m, func() tea.Msg { return CloseBoardItemConverterMsg{} }
		case converterStepParent:
			m.step = converterStepProject
			m.targetProject = nil
			m.allTasks = nil
			m.filteredTasks = nil
			m.ClearSearch()
			return m, nil
		}
		return m, nil

	case "q":
		if !m.IsSearchMode() {
			return m, func() tea.Msg { return CloseBoardItemConverterMsg{} }
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

func (m *BoardItemConverterModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case converterStepProject:
		if len(m.filteredProjects) == 0 {
			return m, nil
		}
		m.targetProject = m.filteredProjects[m.projectCursor]
		if err := storage.LoadProject(m.targetProject); err != nil {
			m.err = fmt.Errorf("failed to load project: %w", err)
			return m, nil
		}
		m.allTasks = m.getParentCandidates()
		m.filteredTasks = m.allTasks
		m.taskCursor = 0
		m.ClearSearch()
		m.step = converterStepParent
		return m, nil

	case converterStepParent:
		return m, m.convertItem()
	}
	return m, nil
}

// getParentCandidates returns tasks that can serve as parents (non-leaf containers + all tasks).
func (m *BoardItemConverterModel) getParentCandidates() []*models.Task {
	var tasks []*models.Task
	for _, t := range m.targetProject.Tasks {
		if !t.Status.IsComplete() {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// convertItem creates the task from the board item and saves everything.
func (m *BoardItemConverterModel) convertItem() tea.Cmd {
	return func() tea.Msg {
		project := m.targetProject
		item := m.boardItem

		task := models.NewTask(uuid.New().String(), project.GetNextShortID(), models.TaskTypeTask, item.Title)
		task.Priority = item.Priority
		task.Tags = make([]string, len(item.Tags))
		copy(task.Tags, item.Tags)
		task.Source = item.Source
		task.URL = item.URL
		task.Description = item.Body

		// Set parent if one was selected (cursor 0 is "No parent")
		if m.taskCursor > 0 && m.taskCursor-1 < len(m.filteredTasks) {
			parentTask := m.filteredTasks[m.taskCursor-1]
			parentID := parentTask.ID
			task.ParentID = &parentID
		}

		project.Tasks = append(project.Tasks, task)
		project.BuildTaskTree()

		if err := storage.BackupProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup project: %w", err)}
		}
		if err := storage.SaveProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Update board item with link
		item.LinkedTaskID = task.ID
		item.ProjectID = project.ID
		item.ProjectName = project.Name
		if err := storage.SaveBoardItem(m.config.DataDir, item); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to update board item: %w", err)}
		}

		return BoardItemConvertedMsg{
			Task:      task,
			Project:   project,
			BoardItem: item,
		}
	}
}

// applySearchFilter applies the current search query to the appropriate list.
func (m *BoardItemConverterModel) applySearchFilter() {
	switch m.step {
	case converterStepProject:
		m.filterProjects()
	case converterStepParent:
		m.filterTasks()
	}
}

func (m *BoardItemConverterModel) filterProjects() {
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

func (m *BoardItemConverterModel) filterTasks() {
	query := strings.TrimSpace(strings.ToLower(m.SearchQuery()))
	if query == "" {
		m.filteredTasks = m.allTasks
		m.taskCursor = clamp(m.taskCursor, 0, len(m.filteredTasks))
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

func (m *BoardItemConverterModel) moveCursor(delta int) {
	switch m.step {
	case converterStepProject:
		maxCursor := len(m.filteredProjects) - 1
		if maxCursor < 0 {
			maxCursor = 0
		}
		m.projectCursor = clamp(m.projectCursor+delta, 0, maxCursor)

	case converterStepParent:
		// +1 because first entry is "No parent (root task)"
		maxCursor := len(m.filteredTasks)
		m.taskCursor = clamp(m.taskCursor+delta, 0, maxCursor)
	}
}

func (m *BoardItemConverterModel) goToFirst() {
	switch m.step {
	case converterStepProject:
		m.projectCursor = 0
	case converterStepParent:
		m.taskCursor = 0
	}
}

func (m *BoardItemConverterModel) goToLast() {
	switch m.step {
	case converterStepProject:
		if len(m.filteredProjects) > 0 {
			m.projectCursor = len(m.filteredProjects) - 1
		}
	case converterStepParent:
		m.taskCursor = len(m.filteredTasks)
	}
}

func (m *BoardItemConverterModel) View() string {
	subtitle := fmt.Sprintf("#%d %s", m.boardItem.ShortID, truncateTitle(m.boardItem.Title, 30))

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Convert to Task", subtitle))
	out.WriteString("\n\n")

	switch m.step {
	case converterStepProject:
		out.WriteString(m.viewProjectSelection())
	case converterStepParent:
		out.WriteString(m.viewParentSelection())
	}

	return out.String()
}

func (m *BoardItemConverterModel) viewProjectSelection() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Select Project (1/2)"))
	b.WriteString("\n\n")

	if m.IsSearchMode() {
		b.WriteString("Search: " + m.SearchInputView())
		b.WriteString("\n\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filteredProjects))))
		b.WriteString("\n\n")
	}

	if len(m.filteredProjects) == 0 {
		if len(m.allProjects) == 0 {
			b.WriteString(MutedStyle.Render("No projects available."))
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
	scrollOffset := calcScrollOffset(m.projectCursor, maxVisible)

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

func (m *BoardItemConverterModel) viewParentSelection() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Select Parent in %s (2/2)", m.targetProject.Name)))
	b.WriteString("\n\n")

	if m.IsSearchMode() {
		b.WriteString("Search: " + m.SearchInputView())
		b.WriteString("\n\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filteredTasks))))
		b.WriteString("\n\n")
	}

	maxVisible := 8
	// Total items = 1 (no parent) + len(filteredTasks)
	totalItems := 1 + len(m.filteredTasks)
	scrollOffset := calcScrollOffset(m.taskCursor, maxVisible)

	for i := 0; i < totalItems; i++ {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}
		cursor := "  "
		if i == m.taskCursor {
			cursor = "> "
		}

		var line string
		if i == 0 {
			line = fmt.Sprintf("%sNo parent (root task)", cursor)
		} else {
			t := m.filteredTasks[i-1]
			statusIcon := GetStatusBadge(string(t.Status), t.Status.Icon())
			typeIcon := GetTypeBadge(string(t.Type), t.Type.Icon())
			title := truncateTitle(t.Title, m.Width()-35)
			line = fmt.Sprintf("%s%s %s #%d %s", cursor, statusIcon, typeIcon, t.ShortID, title)
		}

		if i == m.taskCursor {
			line = BoldStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if totalItems > maxVisible {
		remaining := totalItems - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.IsSearchMode() {
		b.WriteString(HelpStyle.Render("Type to filter • Enter confirm • Esc back"))
	} else {
		b.WriteString(HelpStyle.Render("/ search • j/k navigate • Enter create task • Esc back"))
	}

	return b.String()
}

// HasTextInput returns true when search mode is active.
func (m *BoardItemConverterModel) HasTextInput() bool {
	return m.IsSearchMode()
}

// ShouldCaptureEsc returns true when the view needs esc for internal navigation.
func (m *BoardItemConverterModel) ShouldCaptureEsc() bool {
	return m.IsSearchMode() || m.SearchQuery() != "" || m.step > converterStepProject
}
