package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetTaskRef_SingleProjectTaskRow(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", models.SectionInbox)

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{task: task, ProjectID: proj.ID},
	}
	m.cursor = 1

	gotTask, gotProj := m.getTaskRef()
	if gotTask != task {
		t.Fatalf("task = %v, want %v", gotTask, task)
	}
	if gotProj != proj {
		t.Fatalf("project = %v, want %v", gotProj, proj)
	}
}

func TestGetTaskRef_CrossProjectTaskRow(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	projB := &models.Project{ID: "proj-B"}
	a1 := makeTask("a1", "A1", "")
	b1 := makeTask("b1", "B1", "")

	m := minimalOutliner()
	m.loadedProjects = []*models.Project{projA, projB}
	m.rows = []outlinerRow{
		{task: a1, ProjectID: projA.ID},
		{IsSeparator: true},
		{task: b1, ProjectID: projB.ID},
	}

	m.cursor = 0
	gotTask, gotProj := m.getTaskRef()
	if gotTask != a1 || gotProj != projA {
		t.Fatalf("cursor=0 got (%v, %v), want (%v, %v)", gotTask, gotProj, a1, projA)
	}

	m.cursor = 2
	gotTask, gotProj = m.getTaskRef()
	if gotTask != b1 || gotProj != projB {
		t.Fatalf("cursor=2 got (%v, %v), want (%v, %v)", gotTask, gotProj, b1, projB)
	}
}

func TestGetTaskRef_HeaderSeparatorEmpty(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", "")

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{IsSeparator: true},
		{task: task, ProjectID: proj.ID},
	}

	cases := []struct {
		name   string
		cursor int
	}{
		{"header", 0},
		{"separator", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.cursor = tc.cursor
			gotTask, gotProj := m.getTaskRef()
			if gotTask != nil || gotProj != nil {
				t.Fatalf("got (%v, %v), want (nil, nil)", gotTask, gotProj)
			}
		})
	}

	m.rows = nil
	m.cursor = 0
	if gotTask, gotProj := m.getTaskRef(); gotTask != nil || gotProj != nil {
		t.Fatalf("empty rows got (%v, %v), want (nil, nil)", gotTask, gotProj)
	}

	m.rows = []outlinerRow{{task: task, ProjectID: proj.ID}}
	m.cursor = 5
	if gotTask, gotProj := m.getTaskRef(); gotTask != nil || gotProj != nil {
		t.Fatalf("out-of-range got (%v, %v), want (nil, nil)", gotTask, gotProj)
	}
}

func TestGetTaskRef_CrossProjectStaleProjectID(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	stale := makeTask("orphan", "Orphan", "")

	m := minimalOutliner()
	m.loadedProjects = []*models.Project{projA}
	m.rows = []outlinerRow{{task: stale, ProjectID: "proj-missing"}}
	m.cursor = 0

	gotTask, gotProj := m.getTaskRef()
	if gotTask != nil || gotProj != nil {
		t.Fatalf("stale project ID got (%v, %v), want (nil, nil)", gotTask, gotProj)
	}
}

func TestCursorProject_SingleProject(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", "")

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{{task: task, ProjectID: proj.ID}}
	m.cursor = 0

	if got := m.cursorProject(); got != proj {
		t.Fatalf("cursorProject = %v, want %v", got, proj)
	}
}

func TestCursorProject_CrossProject(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	projB := &models.Project{ID: "proj-B"}
	a := makeTask("a", "A", "")
	b := makeTask("b", "B", "")

	m := minimalOutliner()
	m.loadedProjects = []*models.Project{projA, projB}
	m.rows = []outlinerRow{
		{task: a, ProjectID: projA.ID},
		{IsSeparator: true},
		{task: b, ProjectID: projB.ID},
	}

	m.cursor = 0
	if got := m.cursorProject(); got != projA {
		t.Fatalf("cursor=0 got %v, want %v", got, projA)
	}

	m.cursor = 2
	if got := m.cursorProject(); got != projB {
		t.Fatalf("cursor=2 got %v, want %v", got, projB)
	}
}

func TestCursorProject_HeaderSeparatorEmptyOutOfRange(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", "")

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{IsSeparator: true},
		{task: task, ProjectID: proj.ID},
	}

	m.cursor = 0
	if got := m.cursorProject(); got != nil {
		t.Fatalf("header got %v, want nil", got)
	}

	m.cursor = 1
	if got := m.cursorProject(); got != nil {
		t.Fatalf("separator got %v, want nil", got)
	}

	m.rows = nil
	m.cursor = 0
	if got := m.cursorProject(); got != nil {
		t.Fatalf("empty rows got %v, want nil", got)
	}

	m.rows = []outlinerRow{{task: task, ProjectID: proj.ID}}
	m.cursor = 5
	if got := m.cursorProject(); got != nil {
		t.Fatalf("out-of-range got %v, want nil", got)
	}
}

func TestCursorProject_NilTaskRow(t *testing.T) {
	proj := &models.Project{ID: "proj"}

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{{ProjectID: proj.ID}}
	m.cursor = 0

	if got := m.cursorProject(); got != nil {
		t.Fatalf("nil task got %v, want nil", got)
	}
}

func TestCursorProject_CrossProjectStaleProjectID(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	stale := makeTask("orphan", "Orphan", "")

	m := minimalOutliner()
	m.loadedProjects = []*models.Project{projA}
	m.rows = []outlinerRow{{task: stale, ProjectID: "proj-missing"}}
	m.cursor = 0

	if got := m.cursorProject(); got != nil {
		t.Fatalf("stale project ID got %v, want nil", got)
	}
}

func TestSetNoTaskStatus(t *testing.T) {
	m := minimalOutliner()
	m.setNoTaskStatus()
	if m.statusMessage != statusNoTaskSelected {
		t.Fatalf("statusMessage = %q, want %q", m.statusMessage, statusNoTaskSelected)
	}
	if m.statusMessage != "No task selected" {
		t.Fatalf("statusMessage = %q, want %q", m.statusMessage, "No task selected")
	}
}

func TestSetNoProjectStatus(t *testing.T) {
	m := minimalOutliner()
	m.setNoProjectStatus()
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("statusMessage = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
	if m.statusMessage != "No project under cursor" {
		t.Fatalf("statusMessage = %q, want %q", m.statusMessage, "No project under cursor")
	}
}
