package testutil

import (
	"fmt"
	"maps"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// MockSelector implements external.FuzzySelector for testing.
type MockSelector struct {
	Available_     bool
	SelectedTask   *models.Task
	SelectedTasks  []*models.Task
	SelectProject_ *models.Project
	SelectError    error
}

func (m *MockSelector) Available() bool {
	return m.Available_
}

func (m *MockSelector) SelectProject(projects []*models.Project, prompt string) (*models.Project, error) {
	if m.SelectError != nil {
		return nil, m.SelectError
	}
	if m.SelectProject_ != nil {
		return m.SelectProject_, nil
	}
	if len(projects) > 0 {
		return projects[0], nil
	}
	return nil, fmt.Errorf("no projects")
}

func (m *MockSelector) SelectTask(tasks []*models.Task, prompt string, previewCmd string) (*models.Task, error) {
	if m.SelectError != nil {
		return nil, m.SelectError
	}
	if m.SelectedTask != nil {
		return m.SelectedTask, nil
	}
	if len(tasks) > 0 {
		return tasks[0], nil
	}
	return nil, fmt.Errorf("no tasks")
}

func (m *MockSelector) MultiSelectTasks(tasks []*models.Task, prompt string) ([]*models.Task, error) {
	if m.SelectError != nil {
		return nil, m.SelectError
	}
	if m.SelectedTasks != nil {
		return m.SelectedTasks, nil
	}
	return tasks, nil
}

// MockRenderer implements external.MarkdownRenderer for testing.
type MockRenderer struct {
	Available_     bool
	RenderedOutput string
	RenderError    error
}

func (m *MockRenderer) Available() bool {
	return m.Available_
}

func (m *MockRenderer) RenderMarkdown(content string) error {
	if m.RenderError != nil {
		return m.RenderError
	}
	m.RenderedOutput = content
	return nil
}

func (m *MockRenderer) RenderFile(path string) error {
	if m.RenderError != nil {
		return m.RenderError
	}
	m.RenderedOutput = "file:" + path
	return nil
}

func (m *MockRenderer) RenderTaskMarkdown(task *models.Task) error {
	if m.RenderError != nil {
		return m.RenderError
	}
	m.RenderedOutput = "task:" + task.Title
	return nil
}

func (m *MockRenderer) PageMarkdown(content string) error {
	if m.RenderError != nil {
		return m.RenderError
	}
	m.RenderedOutput = content
	return nil
}

// MockEditor implements external.Editor for testing.
type MockEditor struct {
	Available_       bool
	OpenedFiles      []string
	EditedContent    string
	ReturnContent    string
	ReturnContentSet bool
	EditError        error
}

func (m *MockEditor) Available() bool {
	return m.Available_
}

func (m *MockEditor) OpenFile(path string) error {
	m.OpenedFiles = append(m.OpenedFiles, path)
	return m.EditError
}

func (m *MockEditor) OpenFileAtLine(path string, line int) error {
	m.OpenedFiles = append(m.OpenedFiles, fmt.Sprintf("%s:%d", path, line))
	return m.EditError
}

func (m *MockEditor) OpenFolder(path string) error {
	m.OpenedFiles = append(m.OpenedFiles, path)
	return m.EditError
}

func (m *MockEditor) OpenNewWindow(path string) error {
	m.OpenedFiles = append(m.OpenedFiles, path)
	return m.EditError
}

func (m *MockEditor) EditContent(content string, extension string) (string, error) {
	if m.EditError != nil {
		return "", m.EditError
	}
	m.EditedContent = content
	if m.ReturnContentSet || m.ReturnContent != "" {
		return m.ReturnContent, nil
	}
	return content, nil
}

// MockRepository implements storage.ProjectRepository for testing.
type MockRepository struct {
	Projects    map[string]*models.Project
	LastSaved   *models.Project // Most recently saved project copy for assertions
	LoadError   error
	SaveError   error
	BackupError error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		Projects: make(map[string]*models.Project),
	}
}

func (m *MockRepository) Load(project *models.Project) error {
	if m.LoadError != nil {
		return m.LoadError
	}
	if stored, ok := m.Projects[project.ID]; ok {
		loaded := cloneProject(stored)
		project.NextShortID = loaded.NextShortID
		project.Sections = loaded.Sections
		project.SectionColors = loaded.SectionColors
		project.Tasks = loaded.Tasks
		project.Warnings = loaded.Warnings
		assignProjectPointers(project)
	}
	return nil
}

func (m *MockRepository) Save(project *models.Project) error {
	if m.SaveError != nil {
		return m.SaveError
	}
	savedProject := cloneProject(project)
	m.Projects[project.ID] = savedProject
	m.LastSaved = savedProject
	return nil
}

func cloneProject(project *models.Project) *models.Project {
	if project == nil {
		return nil
	}

	projectCopy := *project
	projectCopy.ArchivedAt = clonePointer(project.ArchivedAt)
	projectCopy.PinnedAt = clonePointer(project.PinnedAt)
	projectCopy.Sections = append([]string{}, project.Sections...)
	projectCopy.SectionColors = make(map[string]string)
	maps.Copy(projectCopy.SectionColors, project.SectionColors)
	projectCopy.Warnings = append([]string{}, project.Warnings...)

	projectCopy.Tasks = make([]*models.Task, len(project.Tasks))
	for i, task := range project.Tasks {
		projectCopy.Tasks[i] = cloneTask(task)
	}
	rebuildTaskTreePointers(project.Tasks, projectCopy.Tasks)
	assignProjectPointers(&projectCopy)
	return &projectCopy
}

func assignProjectPointers(project *models.Project) {
	if project == nil {
		return
	}
	for _, task := range project.Tasks {
		if task != nil {
			task.Project = project
		}
	}
}

func rebuildTaskTreePointers(sourceTasks, tasks []*models.Task) {
	taskMap := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		task.Children = []*models.Task{}
		task.Parent = nil
		task.Level = 0
		taskMap[task.ID] = task
	}

	linked := make(map[string]map[string]bool, len(tasks))
	linkParentChild := func(parent, child *models.Task) {
		if parent == nil || child == nil {
			return
		}
		if linked[parent.ID] == nil {
			linked[parent.ID] = make(map[string]bool)
		}
		if linked[parent.ID][child.ID] {
			return
		}
		linked[parent.ID][child.ID] = true
		child.Parent = parent
		parent.Children = append(parent.Children, child)
	}

	for _, task := range tasks {
		if task == nil || task.ParentID == nil {
			continue
		}
		parent, ok := taskMap[*task.ParentID]
		if !ok {
			continue
		}
		linkParentChild(parent, task)
	}

	sourceToClone := make(map[*models.Task]*models.Task, len(tasks))
	for i, sourceTask := range sourceTasks {
		if sourceTask == nil || i >= len(tasks) {
			continue
		}
		sourceToClone[sourceTask] = tasks[i]
	}
	for _, sourceTask := range sourceTasks {
		parent := sourceToClone[sourceTask]
		if parent == nil {
			continue
		}
		for _, sourceChild := range sourceTask.Children {
			child := sourceToClone[sourceChild]
			linkParentChild(parent, child)
		}
	}

	visited := make(map[string]bool, len(tasks))
	setLevels := func(task *models.Task, level int) {}
	setLevels = func(task *models.Task, level int) {
		if task == nil || visited[task.ID] {
			return
		}
		visited[task.ID] = true
		task.Level = level
		for _, child := range task.Children {
			setLevels(child, level+1)
		}
	}

	for _, task := range tasks {
		if task != nil && task.Parent == nil {
			setLevels(task, 0)
		}
	}
	for _, task := range tasks {
		setLevels(task, 0)
	}
}

func cloneTask(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}

	taskCopy := *task
	taskCopy.ParentID = clonePointer(task.ParentID)
	taskCopy.Tags = append([]string{}, task.Tags...)
	taskCopy.CompletedAt = clonePointer(task.CompletedAt)
	taskCopy.DueDate = clonePointer(task.DueDate)
	taskCopy.DependsOn = append([]string{}, task.DependsOn...)
	taskCopy.RelatedTo = append([]string{}, task.RelatedTo...)
	taskCopy.PinnedAt = clonePointer(task.PinnedAt)
	taskCopy.Comments = clonePointerSlice(task.Comments)
	taskCopy.Decisions = clonePointerSlice(task.Decisions)
	taskCopy.Attachments = append([]models.Attachment{}, task.Attachments...)
	taskCopy.SnoozedUntil = clonePointer(task.SnoozedUntil)
	taskCopy.Activity = clonePointerSlice(task.Activity)
	taskCopy.Checklist = append([]models.ChecklistItem{}, task.Checklist...)
	taskCopy.StartDate = clonePointer(task.StartDate)
	taskCopy.ScoreDimensions = maps.Clone(task.ScoreDimensions)
	taskCopy.BugAttrs = clonePointer(task.BugAttrs)
	taskCopy.IdeaAttrs = clonePointer(task.IdeaAttrs)
	taskCopy.TaskAttrs = clonePointer(task.TaskAttrs)
	taskCopy.StoryAttrs = clonePointer(task.StoryAttrs)
	taskCopy.EpicAttrs = cloneEpicAttrs(task.EpicAttrs)

	taskCopy.Children = nil
	taskCopy.Parent = nil
	taskCopy.Project = nil
	taskCopy.Level = 0
	return &taskCopy
}

func cloneEpicAttrs(attrs *models.EpicAttrs) *models.EpicAttrs {
	if attrs == nil {
		return nil
	}
	attrsCopy := *attrs
	attrsCopy.ScopeBaselinedAt = clonePointer(attrs.ScopeBaselinedAt)
	return &attrsCopy
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	valueCopy := *value
	return &valueCopy
}

func clonePointerSlice[T any](values []*T) []*T {
	if values == nil {
		return nil
	}
	valuesCopy := make([]*T, len(values))
	for i, value := range values {
		valuesCopy[i] = clonePointer(value)
	}
	return valuesCopy
}

func (m *MockRepository) Backup(project *models.Project) error {
	return m.BackupError
}

func (m *MockRepository) RestoreFromBackup(project *models.Project) error {
	return nil
}

func (m *MockRepository) Exists(projectPath string) bool {
	for _, p := range m.Projects {
		if p.Path == projectPath {
			return true
		}
	}
	return false
}

func (m *MockRepository) CreateDir(project *models.Project) error {
	return nil
}

func (m *MockRepository) DeleteDir(projectPath string) error {
	for id, p := range m.Projects {
		if p.Path == projectPath {
			delete(m.Projects, id)
			break
		}
	}
	return nil
}

func (m *MockRepository) BackupExists(project *models.Project) bool {
	return false
}

// Ensure mocks implement their interfaces.
// Note: MockRepository implements storage.ProjectRepository but we can't
// assert it here due to import cycles (storage tests import testutil).
var (
	_ external.FuzzySelector    = (*MockSelector)(nil)
	_ external.MarkdownRenderer = (*MockRenderer)(nil)
	_ external.Editor           = (*MockEditor)(nil)
)
