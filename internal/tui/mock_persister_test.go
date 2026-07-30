package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// MockPersister implements the Persister interface for testing.
// It allows controlling return values and tracking calls to persistence methods.
type MockPersister struct {
	// Configurable errors for each operation
	LoadError error

	// Tracking for method calls
	LoadCalls []*models.Project

	// Projects storage for Load simulation
	Projects map[string]*models.Project
}

// NewMockPersister creates a new MockPersister with initialized slices.
func NewMockPersister() *MockPersister {
	return &MockPersister{
		LoadCalls: make([]*models.Project, 0),
		Projects:  make(map[string]*models.Project),
	}
}

func (m *MockPersister) Load(project *models.Project) error {
	m.LoadCalls = append(m.LoadCalls, project)
	if m.LoadError != nil {
		return m.LoadError
	}
	if stored, ok := m.Projects[project.ID]; ok {
		project.Tasks = stored.Tasks
		project.NextShortID = stored.NextShortID
	}
	return nil
}

// Verify MockPersister implements Persister interface
var _ Persister = (*MockPersister)(nil)
