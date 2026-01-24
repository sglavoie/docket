package testutil

import (
	"fmt"

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
	Available_    bool
	OpenedFiles   []string
	EditedContent string
	ReturnContent string
	EditError     error
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
	if m.ReturnContent != "" {
		return m.ReturnContent, nil
	}
	return content, nil
}

// MockRepository implements storage.ProjectRepository for testing.
type MockRepository struct {
	Projects    map[string]*models.Project
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
		// Deep copy to avoid shared references
		project.NextShortID = stored.NextShortID
		project.Sections = append([]string{}, stored.Sections...)
		project.SectionColors = make(map[string]string)
		for k, v := range stored.SectionColors {
			project.SectionColors[k] = v
		}

		// Deep copy tasks
		project.Tasks = make([]*models.Task, len(stored.Tasks))
		for i, task := range stored.Tasks {
			taskCopy := *task
			// Deep copy slices
			if task.Tags != nil {
				taskCopy.Tags = append([]string{}, task.Tags...)
			}
			if task.DependsOn != nil {
				taskCopy.DependsOn = append([]string{}, task.DependsOn...)
			}
			if task.Comments != nil {
				taskCopy.Comments = make([]*models.Comment, len(task.Comments))
				for j, comment := range task.Comments {
					commentCopy := *comment
					taskCopy.Comments[j] = &commentCopy
				}
			}
			project.Tasks[i] = &taskCopy
		}
	}
	return nil
}

func (m *MockRepository) Save(project *models.Project) error {
	if m.SaveError != nil {
		return m.SaveError
	}
	// Deep copy the project to avoid test interference
	savedProject := &models.Project{
		ID:            project.ID,
		Name:          project.Name,
		Path:          project.Path,
		NextShortID:   project.NextShortID,
		Sections:      append([]string{}, project.Sections...),
		SectionColors: make(map[string]string),
		Tasks:         make([]*models.Task, len(project.Tasks)),
	}

	// Deep copy tasks
	for i, task := range project.Tasks {
		taskCopy := *task // Copy the struct
		// Deep copy slices and maps
		if task.Tags != nil {
			taskCopy.Tags = append([]string{}, task.Tags...)
		}
		if task.DependsOn != nil {
			taskCopy.DependsOn = append([]string{}, task.DependsOn...)
		}
		if task.Comments != nil {
			taskCopy.Comments = make([]*models.Comment, len(task.Comments))
			for j, comment := range task.Comments {
				commentCopy := *comment
				taskCopy.Comments[j] = &commentCopy
			}
		}
		savedProject.Tasks[i] = &taskCopy
	}

	for k, v := range project.SectionColors {
		savedProject.SectionColors[k] = v
	}
	m.Projects[project.ID] = savedProject
	return nil
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
