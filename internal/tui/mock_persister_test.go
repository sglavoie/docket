package tui

import (
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

// mockPersister implements the Persister interface for testing.
// It allows controlling return values and tracking calls to persistence methods.
type mockPersister struct {
	// Configurable errors for each operation
	LoadError error

	// Tracking for method calls
	LoadCalls []*models.Project

	// Projects storage for Load simulation
	Projects map[string]*models.Project
}

// newMockPersister creates a new mockPersister with initialized slices.
func newMockPersister() *mockPersister {
	return &mockPersister{
		LoadCalls: make([]*models.Project, 0),
		Projects:  make(map[string]*models.Project),
	}
}

func (m *mockPersister) Load(project *models.Project) error {
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

// Verify mockPersister implements Persister interface
var _ coordinator.Persister = (*mockPersister)(nil)
