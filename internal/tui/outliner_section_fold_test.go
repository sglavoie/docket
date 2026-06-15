package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func newSectionFoldTestOutliner(project *models.Project) *OutlinerModel {
	m := minimalOutliner()
	m.project = project
	m.activePerspective = PerspectiveAll
	m.Dispatcher = keymap.NewDispatcher(m.buildKeyMap())
	return m
}

func dispatchOutlinerKeys(t *testing.T, m *OutlinerModel, keys ...rune) *OutlinerModel {
	t.Helper()
	for _, key := range keys {
		model, _, consumed := m.Dispatcher.Dispatch(syntheticKey(key))
		if !consumed {
			t.Fatalf("key %q was not consumed", string(key))
		}
		if model == nil {
			continue
		}
		next, ok := model.(*OutlinerModel)
		if !ok {
			t.Fatalf("key %q returned model %T, want *OutlinerModel", string(key), model)
		}
		m = next
	}
	return m
}

func countOutlinerTaskRows(rows []outlinerRow) int {
	count := 0
	for _, row := range rows {
		if row.task != nil && !row.IsSectionHeader && !row.IsSeparator {
			count++
		}
	}
	return count
}

func TestOutlinerSectionFoldPrefixAllCollapsesVisibleSections(t *testing.T) {
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)
	nextTask := makeTask("next-1", "Next task", models.SectionNext)
	project := &models.Project{
		ID:    "project",
		Tasks: []*models.Task{inboxTask, nextTask},
	}
	project.BuildTaskTree()

	m := newSectionFoldTestOutliner(project)
	m.rebuildRows()
	m.viewScrollOffset = 4

	m = dispatchOutlinerKeys(t, m, 'f', 'S')

	for _, key := range []string{models.SectionInbox, models.SectionNext} {
		if !m.collapsedSections[key] {
			t.Fatalf("collapsedSections[%q] = false, want true", key)
		}
	}
	if got := countOutlinerTaskRows(m.rows); got != 0 {
		t.Fatalf("visible task rows after collapse-all = %d, want 0", got)
	}
	if m.viewScrollOffset != 0 {
		t.Fatalf("viewScrollOffset = %d, want reset to 0", m.viewScrollOffset)
	}
}

func TestOutlinerSectionFoldPrefixAllExpandsCollapsedSections(t *testing.T) {
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)
	nextTask := makeTask("next-1", "Next task", models.SectionNext)
	project := &models.Project{
		ID:    "project",
		Tasks: []*models.Task{inboxTask, nextTask},
	}
	project.BuildTaskTree()

	m := newSectionFoldTestOutliner(project)
	m.collapsedSections[models.SectionInbox] = true
	m.collapsedSections[models.SectionNext] = true
	m.rebuildRows()
	m.viewScrollOffset = 4

	m = dispatchOutlinerKeys(t, m, 'f', 'S')

	for _, key := range []string{models.SectionInbox, models.SectionNext} {
		if m.collapsedSections[key] {
			t.Fatalf("collapsedSections[%q] after expand-all = true, want false", key)
		}
	}
	if got := countOutlinerTaskRows(m.rows); got != 2 {
		t.Fatalf("visible task rows after expand-all = %d, want 2", got)
	}
	if m.viewScrollOffset != 0 {
		t.Fatalf("viewScrollOffset = %d, want reset to 0", m.viewScrollOffset)
	}
}

func TestOutlinerSectionFoldPrefixAllMixedStateCollapsesAll(t *testing.T) {
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)
	nextTask := makeTask("next-1", "Next task", models.SectionNext)
	project := &models.Project{
		ID:    "project",
		Tasks: []*models.Task{inboxTask, nextTask},
	}
	project.BuildTaskTree()

	m := newSectionFoldTestOutliner(project)
	m.collapsedSections[models.SectionInbox] = true
	m.rebuildRows()

	m = dispatchOutlinerKeys(t, m, 'f', 'S')

	for _, key := range []string{models.SectionInbox, models.SectionNext} {
		if !m.collapsedSections[key] {
			t.Fatalf("collapsedSections[%q] = false, want true", key)
		}
	}
	if got := countOutlinerTaskRows(m.rows); got != 0 {
		t.Fatalf("visible task rows after mixed-state collapse-all = %d, want 0", got)
	}
}

func TestOutlinerSectionFoldPrefixAllCrossProjectUsesSharedSectionKeys(t *testing.T) {
	alphaTask := makeTask("alpha-1", "Alpha inbox", models.SectionInbox)
	betaTask := makeTask("beta-1", "Beta next", models.SectionNext)
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Tasks: []*models.Task{alphaTask}}
	beta := &models.Project{ID: "beta", Name: "Beta", Tasks: []*models.Task{betaTask}}
	alpha.BuildTaskTree()
	beta.BuildTaskTree()

	m := newSectionFoldTestOutliner(nil)
	m.loadedProjects = []*models.Project{alpha, beta}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	m = dispatchOutlinerKeys(t, m, 'f', 'S')

	for _, key := range []string{"_xp:" + models.SectionInbox, "_xp:" + models.SectionNext} {
		if !m.collapsedSections[key] {
			t.Fatalf("collapsedSections[%q] = false, want true", key)
		}
	}
	for _, key := range []string{models.SectionInbox, models.SectionNext} {
		if m.collapsedSections[key] {
			t.Fatalf("collapsedSections[%q] = true, want false for cross-project All", key)
		}
	}
	if got := countOutlinerTaskRows(m.rows); got != 0 {
		t.Fatalf("visible task rows after cross-project collapse-all = %d, want 0", got)
	}
}

func TestOutlinerSectionFoldPrefixAllNoOpsWithoutHeaders(t *testing.T) {
	task := makeTask("task-1", "Task", models.SectionNext)
	project := &models.Project{
		ID:    "project",
		Tasks: []*models.Task{task},
	}
	project.BuildTaskTree()

	m := newSectionFoldTestOutliner(project)
	m.rows = []outlinerRow{{task: task}}
	m.cursor = 0
	m.viewScrollOffset = 4

	m = dispatchOutlinerKeys(t, m, 'f', 'S')

	if len(m.collapsedSections) != 0 {
		t.Fatalf("collapsedSections = %#v, want empty", m.collapsedSections)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want unchanged 0", m.cursor)
	}
	if m.viewScrollOffset != 4 {
		t.Fatalf("viewScrollOffset = %d, want unchanged 4", m.viewScrollOffset)
	}
	if got := countOutlinerTaskRows(m.rows); got != 1 {
		t.Fatalf("visible task rows after no-op = %d, want 1", got)
	}
}

func TestOutlinerSectionFoldPrefixTaskCollapsesContainingSection(t *testing.T) {
	task := makeTask("someday-1", "Someday task", models.SectionSomeday)
	project := &models.Project{
		ID:    "project",
		Tasks: []*models.Task{task},
	}
	project.BuildTaskTree()

	m := newSectionFoldTestOutliner(project)
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionSomeday, SectionCount: 1},
		{task: task},
	}
	m.cursor = 1

	m = dispatchOutlinerKeys(t, m, 'f', 's')

	if !m.collapsedSections[models.SectionSomeday] {
		t.Fatalf("collapsedSections[%q] = false, want true", models.SectionSomeday)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want section header index 0", m.cursor)
	}
	if got := countOutlinerTaskRows(m.rows); got != 0 {
		t.Fatalf("visible task rows after collapse = %d, want 0", got)
	}
}

func TestOutlinerSectionFoldPrefixHeaderTogglesSection(t *testing.T) {
	task := makeTask("next-1", "Next task", models.SectionNext)
	project := &models.Project{
		ID:    "project",
		Tasks: []*models.Task{task},
	}
	project.BuildTaskTree()

	m := newSectionFoldTestOutliner(project)
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionNext, SectionCount: 1},
		{task: task},
	}
	m.cursor = 0

	m = dispatchOutlinerKeys(t, m, 'f', 's')
	if !m.collapsedSections[models.SectionNext] {
		t.Fatalf("collapsedSections[%q] after collapse = false, want true", models.SectionNext)
	}
	if got := countOutlinerTaskRows(m.rows); got != 0 {
		t.Fatalf("visible task rows after collapse = %d, want 0", got)
	}

	m = dispatchOutlinerKeys(t, m, 'f', 's')
	if m.collapsedSections[models.SectionNext] {
		t.Fatalf("collapsedSections[%q] after expand = true, want false", models.SectionNext)
	}
	if got := countOutlinerTaskRows(m.rows); got != 1 {
		t.Fatalf("visible task rows after expand = %d, want 1", got)
	}
}

func TestOutlinerSectionFoldPrefixCrossProjectAllUsesSharedSectionKey(t *testing.T) {
	alphaTask := makeTask("alpha-1", "Alpha inbox", models.SectionInbox)
	betaTask := makeTask("beta-1", "Beta inbox", models.SectionInbox)
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Tasks: []*models.Task{alphaTask}}
	beta := &models.Project{ID: "beta", Name: "Beta", Tasks: []*models.Task{betaTask}}
	alpha.BuildTaskTree()
	beta.BuildTaskTree()

	m := newSectionFoldTestOutliner(nil)
	m.loadedProjects = []*models.Project{alpha, beta}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	headerIndex := -1
	for i, row := range m.rows {
		if row.IsSectionHeader && row.SectionName == models.SectionInbox {
			headerIndex = i
			break
		}
	}
	if headerIndex < 0 {
		t.Fatal("Inbox section header not found")
	}
	m.cursor = headerIndex

	m = dispatchOutlinerKeys(t, m, 'f', 's')

	sharedKey := "_xp:" + models.SectionInbox
	if !m.collapsedSections[sharedKey] {
		t.Fatalf("collapsedSections[%q] = false, want true", sharedKey)
	}
	if m.collapsedSections[models.SectionInbox] {
		t.Fatalf("collapsedSections[%q] = true, want false for cross-project All", models.SectionInbox)
	}
	if got := countOutlinerTaskRows(m.rows); got != 0 {
		t.Fatalf("visible task rows after cross-project collapse = %d, want 0", got)
	}
}

func TestOutlinerSectionFoldPrefixNoOpsOnNonSectionRows(t *testing.T) {
	task := makeTask("task-1", "Task", models.SectionNext)

	tests := []struct {
		name              string
		rows              []outlinerRow
		cursor            int
		project           *models.Project
		activePerspective int
	}{
		{
			name: "separator",
			rows: []outlinerRow{
				{IsSeparator: true},
			},
			activePerspective: PerspectiveAll,
		},
		{
			name: "task without header",
			rows: []outlinerRow{
				{task: task},
			},
			project:           &models.Project{ID: "project", Tasks: []*models.Task{task}},
			activePerspective: PerspectiveAll,
		},
		{
			name: "non-All cross-project task row",
			rows: []outlinerRow{
				{task: task, ProjectID: "project"},
				{IsSeparator: true},
			},
			activePerspective: PerspectiveAll + 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.project != nil {
				tt.project.BuildTaskTree()
			}
			m := newSectionFoldTestOutliner(tt.project)
			m.activePerspective = tt.activePerspective
			m.rows = tt.rows
			m.cursor = tt.cursor

			m = dispatchOutlinerKeys(t, m, 'f', 's')

			if len(m.collapsedSections) != 0 {
				t.Fatalf("collapsedSections = %#v, want empty", m.collapsedSections)
			}
			if m.cursor != tt.cursor {
				t.Fatalf("cursor = %d, want unchanged %d", m.cursor, tt.cursor)
			}
		})
	}
}
