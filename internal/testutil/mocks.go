package testutil

import "github.com/sglavoie/tt/internal/models"

// MockRepository implements storage.ProjectRepository for testing.
type MockRepository struct {
	Projects  map[string]*models.Project
	LastSaved *models.Project // Most recently saved project copy for assertions
	LoadError error
	SaveError error
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
		project.Tasks = loaded.Tasks
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
	taskCopy.PinnedAt = clonePointer(task.PinnedAt)
	taskCopy.Comments = clonePointerSlice(task.Comments)
	taskCopy.Attachments = append([]models.Attachment{}, task.Attachments...)
	taskCopy.DeferredUntil = clonePointer(task.DeferredUntil)
	taskCopy.Checklist = append([]models.ChecklistItem{}, task.Checklist...)

	taskCopy.Children = nil
	taskCopy.Parent = nil
	taskCopy.Project = nil
	taskCopy.Level = 0
	return &taskCopy
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
