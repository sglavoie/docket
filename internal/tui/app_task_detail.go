package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// activateTaskDetail opens or retargets the one live Task Detail child route.
// Legacy child messages call this seam with their corresponding tab, making
// those messages aliases instead of independent navigation participants.
func (m *AppModel) activateTaskDetail(task *models.Task, project *models.Project, tab TaskDetailTab) tea.Cmd {
	if task == nil || project == nil || tab > TaskActivity {
		return nil
	}
	if !m.typedNavigation {
		return m.activateLegacyTaskDetail(task, project, tab)
	}

	m.lastNavigatedTaskID = task.ID
	m.recordTaskVisit(task, project)

	if m.taskDetail == nil || m.taskDetail.task == nil || m.taskDetail.project == nil ||
		m.taskDetail.task.ID != task.ID || m.taskDetail.project.ID != project.ID {
		editor := ""
		if m.runtime != nil {
			editor = m.runtime.Machine().GetEditor()
		}
		m.taskDetail = NewTaskDetailModelForTab(
			task,
			project,
			m.runtime,
			tab,
			editor,
			m.secondaryFeatures.Subview,
			m.secondaryFeatures.Attachment,
		)
		m.taskDetail.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		m.taskDetail.SetSize(m.width, m.height-2)
		m.registerTaskDetailPresentations()
	} else {
		m.taskDetail.task = task
		m.taskDetail.project = project
		m.taskDetail.SetTab(tab)
		m.taskDetail.syncEmbeddedTasks()
	}
	identity := taskIdentityFor(task, project)
	m.taskDetail.SetPlanningMembership(m.dailyPlanIDs[identity], m.focusQueueIDs[identity])

	route, err := NewRoute(TaskDetail, TaskDetailParams{
		ProjectID: project.ID,
		TaskID:    task.ID,
		Tab:       tab,
	})
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}

	m.navigation.Overlays.CancelAll()
	if m.navigation.Primary.Current().ID() == TaskDetail {
		if err := m.navigation.Primary.ReplaceCurrentMode(route); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: err} }
		}
	} else if err := m.navigation.Primary.PushTaskDetail(route, m.currentFocusTarget()); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	m.syncPresentationStack()
	m.setAccentOnView(m.taskDetail)
	return m.taskDetail.Init()
}

func (m *AppModel) activateLegacyTaskDetail(task *models.Task, project *models.Project, tab TaskDetailTab) tea.Cmd {
	editor := ""
	if m.runtime != nil {
		editor = m.runtime.Machine().GetEditor()
	}
	switch tab {
	case TaskChecklist:
		view := NewTaskChecklistModelWithEditor(task, project, editor, m.secondaryFeatures.Subview)
		m.showView(ViewTaskChecklist, view)
		return view.Init()
	case TaskComments:
		view := NewTaskCommentsModelWithEditor(task, project, editor, m.secondaryFeatures.Subview)
		m.showView(ViewTaskComments, view)
		return view.Init()
	case TaskDecisions:
		view := NewTaskDecisionsModelWithEditor(task, project, editor, m.secondaryFeatures.Subview)
		m.showView(ViewTaskDecisions, view)
		return view.Init()
	case TaskAttachments:
		view := NewTaskAttachmentsModel(task, project, m.runtime, m.secondaryFeatures.Attachment)
		m.showView(ViewTaskAttachments, view)
		return view.Init()
	case TaskActivity:
		view := NewTaskActivityModel(task, project)
		m.showView(ViewTaskActivity, view)
		return view.Init()
	default:
		view := NewTaskDetailModel(task, project, m.runtime)
		m.showView(ViewTaskDetail, view)
		return view.Init()
	}
}

func (m *AppModel) registerTaskDetailPresentations() {
	if m.views == nil || m.taskDetail == nil {
		return
	}
	m.views.Register(ViewTaskDetail, m.taskDetail)
	m.views.Register(ViewTaskChecklist, m.taskDetail.checklist)
	m.views.Register(ViewTaskComments, m.taskDetail.comments)
	m.views.Register(ViewTaskDecisions, m.taskDetail.decisions)
	m.views.Register(ViewTaskAttachments, m.taskDetail.attachments)
	m.views.Register(ViewTaskActivity, m.taskDetail.activity)
}

func (m *AppModel) syncTaskDetailTab(tab TaskDetailTab) tea.Cmd {
	if !m.typedNavigation || m.taskDetail == nil || tab > TaskActivity {
		return nil
	}
	current := m.navigation.Primary.Current()
	params, ok := current.Params().(TaskDetailParams)
	if current.ID() != TaskDetail || !ok {
		return nil
	}
	if params.Tab == tab && m.taskDetail.Tab() == tab {
		return nil
	}
	params.Tab = tab
	route, err := NewRoute(TaskDetail, params)
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	m.taskDetail.SetTab(tab)
	if err := m.navigation.Primary.ReplaceCurrentMode(route); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	m.syncPresentationStack()
	return nil
}
