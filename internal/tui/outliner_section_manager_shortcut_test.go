package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerCtrlS_CrossProjectTaskRowTargetsOwningProject(t *testing.T) {
	project := &models.Project{ID: "proj-cross", Name: "Cross"}
	task := &models.Task{ID: "task-cross", Title: "Cross Task", WorkflowState: models.StateInbox}

	m := minimalOutliner()
	m.project = nil
	m.loadedProjects = []*models.Project{project}
	m.rows = []outlinerRow{{task: task, ProjectID: project.ID}}
	m.cursor = 0

	_, cmd, handled := m.handleUtilityKey("ctrl+s")
	if !handled {
		t.Fatal("expected ctrl+s to be handled")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+s to emit a command")
	}

	gotMsg := cmd()
	msg, ok := gotMsg.(ShowSectionManagerMsg)
	if !ok {
		t.Fatalf("expected ShowSectionManagerMsg, got %T", gotMsg)
	}
	if msg.Project != project {
		t.Fatalf("section manager project = %v, want %v", msg.Project, project)
	}
}

func TestOutlinerCtrlS_CrossProjectUnscopedNonProjectRowsDoNotOpenManager(t *testing.T) {
	project := &models.Project{ID: "proj-cross", Name: "Cross"}

	tests := []struct {
		name string
		rows []outlinerRow
	}{
		{
			name: "section header",
			rows: []outlinerRow{{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: project.ID}},
		},
		{
			name: "separator",
			rows: []outlinerRow{{IsSeparator: true}},
		},
		{
			name: "no rows",
			rows: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := minimalOutliner()
			m.project = nil
			m.loadedProjects = []*models.Project{project}
			m.rows = tt.rows
			m.cursor = 0

			_, cmd, handled := m.handleUtilityKey("ctrl+s")
			if !handled {
				t.Fatal("expected ctrl+s to be handled")
			}
			if cmd != nil {
				t.Fatal("expected ctrl+s on a non-task target to avoid opening the manager")
			}
			if m.statusMessage != statusNoProjectUnderCursor {
				t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
			}
		})
	}
}

func TestOutlinerCtrlS_SingleProjectTargetsCurrentProject(t *testing.T) {
	project := &models.Project{ID: "proj-single", Name: "Single"}

	m := minimalOutliner()
	m.project = project

	_, cmd, handled := m.handleUtilityKey("ctrl+s")
	if !handled {
		t.Fatal("expected ctrl+s to be handled")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+s to emit a command")
	}

	gotMsg := cmd()
	msg, ok := gotMsg.(ShowSectionManagerMsg)
	if !ok {
		t.Fatalf("expected ShowSectionManagerMsg, got %T", gotMsg)
	}
	if msg.Project != project {
		t.Fatalf("section manager project = %v, want %v", msg.Project, project)
	}
}
