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

func TestGetTaskRef_SectionHeaderSeparatorEmpty(t *testing.T) {
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

func TestOutlinerProjectContext(t *testing.T) {
	assertProject := func(t *testing.T, m *OutlinerModel, want *models.Project) {
		t.Helper()
		if got := m.contextProject(); got != want {
			t.Fatalf("contextProject() = %v, want %v", got, want)
		}
	}

	t.Run("single project always wins", func(t *testing.T) {
		project := &models.Project{ID: "single"}
		m := minimalOutliner()
		m.project = project
		m.rows = nil
		m.cursor = 99
		assertProject(t, m, project)
	})

	t.Run("cross project task resolves explicit owner", func(t *testing.T) {
		alpha := &models.Project{ID: "alpha"}
		beta := &models.Project{ID: "beta"}
		m := minimalOutliner()
		m.loadedProjects = []*models.Project{alpha, beta}
		m.rows = []outlinerRow{{task: makeTask("task", "Task", ""), ProjectID: beta.ID}}
		assertProject(t, m, beta)
	})

	t.Run("real project header resolves explicit owner", func(t *testing.T) {
		m, _, betaHeader, _, _ := crossProjectHeaderFixture(t)
		m.cursor = betaHeader
		want := m.crossResolveProject(m.rows[betaHeader].ProjectID)
		assertProject(t, m, want)
		if task, project := m.getTaskRef(); task != nil || project != nil {
			t.Fatalf("project header became task target: got (%v, %v)", task, project)
		}
	})

	t.Run("scoped non-project rows use active scope", func(t *testing.T) {
		scoped := &models.Project{ID: "scoped"}
		other := &models.Project{ID: "other"}
		for _, tc := range []struct {
			name string
			rows []outlinerRow
		}{
			{
				name: "section header ignores grouping project ID",
				rows: []outlinerRow{{
					IsSectionHeader: true,
					SectionName:     models.SectionInbox,
					ProjectID:       other.ID,
				}},
			},
			{name: "separator", rows: []outlinerRow{{IsSeparator: true}}},
			{name: "empty row", rows: []outlinerRow{{}}},
			{name: "empty result", rows: nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m := minimalOutliner()
				m.loadedProjects = []*models.Project{scoped, other}
				m.scopedProjectID = scoped.ID
				m.rows = tc.rows
				m.cursor = 0
				assertProject(t, m, scoped)
			})
		}
	})

	t.Run("unscoped non-project rows have no context", func(t *testing.T) {
		project := &models.Project{ID: "project"}
		for _, tc := range []struct {
			name string
			rows []outlinerRow
		}{
			{
				name: "section header with grouping project ID",
				rows: []outlinerRow{{
					IsSectionHeader: true,
					SectionName:     models.SectionInbox,
					ProjectID:       project.ID,
				}},
			},
			{name: "separator", rows: []outlinerRow{{IsSeparator: true}}},
			{name: "empty row", rows: []outlinerRow{{}}},
			{name: "empty result", rows: nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m := minimalOutliner()
				m.loadedProjects = []*models.Project{project}
				m.rows = tc.rows
				m.cursor = 0
				assertProject(t, m, nil)
			})
		}
	})

	t.Run("stale explicit IDs do not fall back to scope", func(t *testing.T) {
		scoped := &models.Project{ID: "scoped"}
		for _, row := range []outlinerRow{
			{task: makeTask("stale-task", "Stale task", ""), ProjectID: "missing"},
			{IsProjectHeader: true, ProjectID: "missing"},
		} {
			m := minimalOutliner()
			m.loadedProjects = []*models.Project{scoped}
			m.scopedProjectID = scoped.ID
			m.rows = []outlinerRow{row}
			assertProject(t, m, nil)
		}
	})

	t.Run("out of range cursor has no context even when scoped", func(t *testing.T) {
		scoped := &models.Project{ID: "scoped"}
		m := minimalOutliner()
		m.loadedProjects = []*models.Project{scoped}
		m.scopedProjectID = scoped.ID
		m.rows = []outlinerRow{{IsSectionHeader: true}}
		m.cursor = len(m.rows)
		assertProject(t, m, nil)
	})
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
