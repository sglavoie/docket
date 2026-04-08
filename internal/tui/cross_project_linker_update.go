package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *CrossProjectLinkerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeypress(msg)
	}
	return m, nil
}

func (m *CrossProjectLinkerModel) handleKeypress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.pendingPrefix != "" {
		prefix := m.pendingPrefix
		m.pendingPrefix = ""
		if key != "esc" && prefix == "g" && key == "g" {
			m.goToFirst()
		}
		return m, nil
	}

	if m.IsSearchMode() && (m.step == StepSelectProject || m.step == StepSelectTask) {
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
		if m.step == StepSelectProject || m.step == StepSelectTask {
			return m, m.ActivateSearch()
		}
		return m, nil

	case "esc":
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.applySearchFilter()
			return m, nil
		}

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
			m.pendingPrefix = "g"
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
		if len(m.filteredProjects) == 0 || m.projectCursor >= len(m.filteredProjects) {
			return m, nil
		}
		m.targetProject = m.filteredProjects[m.projectCursor]
		if err := storage.LoadProject(m.targetProject); err != nil {
			return m, func() tea.Msg { return StatusMsg(fmt.Sprintf("Failed to load project: %v", err)) }
		}
		m.allTasks = m.getSelectableTasks()
		m.filteredTasks = m.allTasks
		m.taskCursor = 0
		m.ClearSearch()
		m.step = StepSelectTask
		return m, nil

	case StepSelectTask:
		if len(m.filteredTasks) == 0 || m.taskCursor >= len(m.filteredTasks) {
			return m, nil
		}
		m.targetTask = m.filteredTasks[m.taskCursor]
		m.ClearSearch()
		m.step = StepSelectLinkType
		m.linkTypeCursor = 0
		return m, nil

	case StepSelectLinkType:
		return m, m.createLink()
	}
	return m, nil
}

func (m *CrossProjectLinkerModel) applySearchFilter() {
	switch m.step {
	case StepSelectProject:
		m.filterProjects()
	case StepSelectTask:
		m.filterTasks()
	}
}

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
		maxCursor := max(len(m.filteredProjects)-1, 0)
		m.projectCursor = clamp(m.projectCursor+delta, 0, maxCursor)
	case StepSelectTask:
		maxCursor := max(len(m.filteredTasks)-1, 0)
		m.taskCursor = clamp(m.taskCursor+delta, 0, maxCursor)
	case StepSelectLinkType:
		m.linkTypeCursor = clamp(m.linkTypeCursor+delta, 0, 2)
	}
}

func (m *CrossProjectLinkerModel) getSelectableTasks() []*models.Task {
	return m.targetProject.Tasks
}

func (m *CrossProjectLinkerModel) createLink() tea.Cmd {
	var linkType LinkType
	switch m.linkTypeCursor {
	case 0:
		linkType = LinkTypeBlocker
	case 1:
		linkType = LinkTypeDependent
	case 2:
		linkType = LinkTypeFollowUp
	}

	targetRef := m.targetProject.ID + ":" + m.targetTask.ID
	sourceRef := m.sourceProject.ID + ":" + m.sourceTask.ID

	switch linkType {
	case LinkTypeBlocker:
		if err := m.sourceTask.AddDependency(targetRef); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to add dependency: %w", err)} }
		}
	case LinkTypeDependent:
		if err := m.targetTask.AddDependency(sourceRef); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to add dependency: %w", err)} }
		}
	case LinkTypeFollowUp:
		if err := m.sourceTask.AddRelatedTo(targetRef); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to add related link: %w", err)} }
		}
		if err := m.targetTask.AddRelatedTo(sourceRef); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to add related link: %w", err)} }
		}
	}

	m.sourceProject.MutateTask(m.sourceTask.ID, func(t *models.Task) {})
	m.targetProject.MutateTask(m.targetTask.ID, func(t *models.Task) {})

	srcProj := m.sourceProject
	tgtProj := m.targetProject
	srcTask := m.sourceTask
	tgtTask := m.targetTask
	return func() tea.Msg {
		if err := storage.BackupProject(srcProj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup source project: %w", err)}
		}
		if err := storage.SaveProjectLocked(srcProj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save source project: %w", err)}
		}
		if err := storage.BackupProject(tgtProj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup target project: %w", err)}
		}
		if err := storage.SaveProjectLocked(tgtProj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save target project: %w", err)}
		}

		return CrossProjectLinkCreatedMsg{
			SourceTask:    srcTask,
			SourceProject: srcProj,
			TargetTask:    tgtTask,
			TargetProject: tgtProj,
			LinkType:      linkType,
		}
	}
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if max >= 0 && value > max {
		return max
	}
	return value
}
