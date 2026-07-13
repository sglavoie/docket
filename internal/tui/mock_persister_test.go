package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// MockPersister implements the Persister interface for testing.
// It allows controlling return values and tracking calls to persistence methods.
type MockPersister struct {
	// Configurable errors for each operation
	LoadError             error
	SaveError             error
	BackupError           error
	RestoreFromBackupErr  error
	RecordUndoSnapshotErr error
	ArchiveTasksErr       error

	// Tracking for method calls
	LoadCalls                      []*models.Project
	SaveCalls                      []*models.Project
	BackupCalls                    []*models.Project
	RecordUndoSnapshotOptionsCalls []storage.UndoSnapshotOptions

	// Configurable return values
	BackupExistsReturn bool
	ArchiveTasksCount  int

	// Projects storage for Load simulation
	Projects map[string]*models.Project
}

// NewMockPersister creates a new MockPersister with initialized slices.
func NewMockPersister() *MockPersister {
	return &MockPersister{
		LoadCalls:                      make([]*models.Project, 0),
		SaveCalls:                      make([]*models.Project, 0),
		BackupCalls:                    make([]*models.Project, 0),
		RecordUndoSnapshotOptionsCalls: make([]storage.UndoSnapshotOptions, 0),
		Projects:                       make(map[string]*models.Project),
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

func (m *MockPersister) Save(project *models.Project) error {
	m.SaveCalls = append(m.SaveCalls, project)
	return m.SaveError
}

func (m *MockPersister) Backup(project *models.Project) error {
	m.BackupCalls = append(m.BackupCalls, project)
	return m.BackupError
}

func (m *MockPersister) BackupExists(project *models.Project) bool {
	return m.BackupExistsReturn
}

func (m *MockPersister) RestoreFromBackup(project *models.Project) error {
	return m.RestoreFromBackupErr
}

func (m *MockPersister) RecordUndoSnapshot(project *models.Project, action, context string, depth int, _ []string) error {
	return m.RecordUndoSnapshotErr
}

func (m *MockPersister) RecordUndoSnapshotWithOptions(project *models.Project, opts storage.UndoSnapshotOptions) error {
	m.RecordUndoSnapshotOptionsCalls = append(m.RecordUndoSnapshotOptionsCalls, opts)
	return m.RecordUndoSnapshotErr
}

func (m *MockPersister) ArchiveTasks(project *models.Project) (int, error) {
	return m.ArchiveTasksCount, m.ArchiveTasksErr
}

// Verify MockPersister implements Persister interface
var _ Persister = (*MockPersister)(nil)
