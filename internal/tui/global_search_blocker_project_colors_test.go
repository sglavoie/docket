package tui

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func assertSession5RenderInvariant(t *testing.T, colored, fallback string) {
	t.Helper()

	if got, want := ansi.Strip(colored), ansi.Strip(fallback); got != want {
		t.Fatalf("project color changed rendered text:\ncolored=%q\nfallback=%q", got, want)
	}
	if got, want := lipgloss.Width(colored), lipgloss.Width(fallback); got != want {
		t.Fatalf("project color changed rendered width: colored=%d fallback=%d\ncolored=%q\nfallback=%q",
			got, want, ansi.Strip(colored), ansi.Strip(fallback))
	}
}

func TestGlobalSearchProjectColorsPreserveRowsPreviewsAndSelection(t *testing.T) {
	previousTheme := activeThemeName
	t.Cleanup(func() { activeThemeName = previousTheme })
	activeThemeName = "dark"

	alpha := &models.Project{ID: "alpha", Name: "Alpha Project", Slug: "alpha-project", ColorSlot: 4}
	beta := &models.Project{ID: "beta", Name: "Beta Project", Slug: "beta-project", ColorSlot: 19}
	alphaFallback := *alpha
	alphaFallback.ColorSlot = 0
	betaFallback := *beta
	betaFallback.ColorSlot = 0

	task := &models.Task{
		ID:            "task-search-color",
		ShortID:       17,
		Title:         "Ship highlighted search",
		Description:   "Keep the description marker",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	note := &models.Note{Title: "Search color notes", Body: "Preview body", Tags: []string{"rendering"}}
	taskResult := filteredSearchResult{
		result:         models.SearchResult{Task: task, Project: alpha},
		titlePositions: []int{0, 5},
	}
	noteResult := filteredSearchResult{
		result:         models.SearchResult{Note: note, Project: beta},
		titlePositions: []int{0, 7},
	}
	taskFallback := taskResult
	taskFallback.result.Project = &alphaFallback
	noteFallback := noteResult
	noteFallback.result.Project = &betaFallback

	m := &GlobalSearchModel{runtime: newTestConfig(), listWidth: 72, previewWidth: 34, cursor: 0}
	taskRow := m.renderTaskSearchRow(1, taskResult, nil)
	taskFallbackRow := m.renderTaskSearchRow(1, taskFallback, nil)
	noteRow := m.renderNoteSearchRow(1, noteResult)
	noteFallbackRow := m.renderNoteSearchRow(1, noteFallback)

	if want := projectColorStyle(alpha).Render("[alpha-p…]"); !strings.Contains(taskRow, want) {
		t.Fatalf("Global Search task row did not color the truncated project badge %q: %q", want, taskRow)
	}
	if want := projectColorStyle(beta).Render("[beta-pr…]"); !strings.Contains(noteRow, want) {
		t.Fatalf("Global Search note row did not color the truncated project badge %q: %q", want, noteRow)
	}
	if projectColor(alpha) == projectColor(beta) {
		t.Fatal("test fixture requires two visibly distinct project slots")
	}
	assertSession5RenderInvariant(t, taskRow, taskFallbackRow)
	assertSession5RenderInvariant(t, noteRow, noteFallbackRow)

	taskPlain := ansi.Strip(taskRow)
	taskIDAt := strings.Index(taskPlain, "#17")
	taskProjectAt := strings.Index(taskPlain, "[alpha-p…]")
	taskTitleAt := strings.Index(taskPlain, task.Title)
	if taskIDAt < 0 || taskProjectAt < taskIDAt || taskTitleAt < taskProjectAt {
		t.Fatalf("Global Search task icon/ID/project/title ordering changed: %q", taskPlain)
	}
	notePlain := ansi.Strip(noteRow)
	noteIconAt := strings.Index(notePlain, "[N]")
	noteProjectAt := strings.Index(notePlain, "[beta-pr…]")
	noteTitleAt := strings.Index(notePlain, note.Title)
	if noteIconAt < 0 || noteProjectAt < noteIconAt || noteTitleAt < noteProjectAt {
		t.Fatalf("Global Search note icon/project/title ordering changed: %q", notePlain)
	}
	if !strings.Contains(taskRow, HighlightStyle.Render("S")) || !strings.Contains(noteRow, HighlightStyle.Render("S")) {
		t.Fatal("Global Search project color change dropped title highlighting")
	}

	selected := m.renderTaskSearchRow(0, taskResult, nil)
	selectedFallback := m.renderTaskSearchRow(0, taskFallback, nil)
	assertSession5RenderInvariant(t, selected, selectedFallback)
	if !regexp.MustCompile(`\x1b\[[^m]*48;`).MatchString(selected) {
		t.Fatalf("selected Global Search row lost its background style: %q", selected)
	}

	taskPreview := m.renderTaskPreview(taskResult)
	taskFallbackPreview := m.renderTaskPreview(taskFallback)
	notePreview := m.renderNotePreview(noteResult.result)
	noteFallbackPreview := m.renderNotePreview(noteFallback.result)
	if want := projectColorStyle(alpha).Bold(true).Render(alpha.Name); !strings.Contains(taskPreview, want) {
		t.Fatalf("Global Search task preview did not color the project value %q: %q", want, taskPreview)
	}
	if want := projectColorStyle(beta).Bold(true).Render(beta.Name); !strings.Contains(notePreview, want) {
		t.Fatalf("Global Search note preview did not color the project value %q: %q", want, notePreview)
	}
	assertSession5RenderInvariant(t, taskPreview, taskFallbackPreview)
	assertSession5RenderInvariant(t, notePreview, noteFallbackPreview)

	m.listWidth = 22
	for _, rows := range [][2]string{
		{m.renderTaskSearchRow(1, taskResult, nil), m.renderTaskSearchRow(1, taskFallback, nil)},
		{m.renderNoteSearchRow(1, noteResult), m.renderNoteSearchRow(1, noteFallback)},
	} {
		assertSession5RenderInvariant(t, rows[0], rows[1])
	}
}

func TestBlockerProjectColorsAreLimitedToGlobalContent(t *testing.T) {
	previousTheme := activeThemeName
	t.Cleanup(func() { activeThemeName = previousTheme })
	activeThemeName = "dark"

	project := &models.Project{ID: "blocker-project", Name: "Delivery", ColorSlot: 11}
	fallbackProject := *project
	fallbackProject.ColorSlot = 0
	blocker := models.NewBlocker("Resolve deployment access")
	blocker.Description = "Coordinate credentials before release."
	item := blockerItem{Blocker: blocker, Project: project}
	fallbackItem := blockerItem{Blocker: blocker, Project: &fallbackProject}

	global := &GlobalBlockersModel{
		runtime: newTestConfig(),
		entries: []GlobalBlockerEntry{{Blocker: blocker, Project: project}},
		state:   newBlockerViewState(""),
	}
	globalFallback := &GlobalBlockersModel{
		runtime: newTestConfig(),
		entries: []GlobalBlockerEntry{{Blocker: blocker, Project: &fallbackProject}},
		state:   newBlockerViewState(""),
	}
	global.state.setSize(38, 16)
	globalFallback.state.setSize(38, 16)

	globalRow := blockerListItemView(&global.state, global, item, false)
	globalFallbackRow := blockerListItemView(&globalFallback.state, globalFallback, fallbackItem, false)
	if want := projectColorStyle(project).Render("[Delivery]"); !strings.Contains(globalRow, want) {
		t.Fatalf("global Blockers row did not color the project identity %q: %q", want, globalRow)
	}
	assertSession5RenderInvariant(t, globalRow, globalFallbackRow)
	for _, selected := range []bool{false, true} {
		row := blockerListItemView(&global.state, global, item, selected)
		fallbackRow := blockerListItemView(&globalFallback.state, globalFallback, fallbackItem, selected)
		assertSession5RenderInvariant(t, row, fallbackRow)
	}

	var globalExpanded strings.Builder
	var fallbackExpanded strings.Builder
	blockerExpandedView(&global.state, global, &globalExpanded)
	blockerExpandedView(&globalFallback.state, globalFallback, &fallbackExpanded)
	if want := projectColorStyle(project).Render(project.Name); !strings.Contains(globalExpanded.String(), want) {
		t.Fatalf("expanded global Blockers content did not color the project value %q: %q", want, globalExpanded.String())
	}
	assertSession5RenderInvariant(t, globalExpanded.String(), fallbackExpanded.String())

	local := &BlockersModel{project: project, blockers: []*models.Blocker{blocker}, state: newBlockerViewState("")}
	localFallback := &BlockersModel{project: &fallbackProject, blockers: []*models.Blocker{blocker}, state: newBlockerViewState("")}
	local.state.setSize(38, 16)
	localFallback.state.setSize(38, 16)
	localRow := blockerListItemView(&local.state, local, item, false)
	localFallbackRow := blockerListItemView(&localFallback.state, localFallback, fallbackItem, false)
	if localRow != localFallbackRow {
		t.Fatalf("project-local Blockers row changed with ColorSlot:\ncolored=%q\nfallback=%q", localRow, localFallbackRow)
	}
	if strings.Contains(ansi.Strip(localRow), project.Name) {
		t.Fatalf("project-local Blockers row unexpectedly rendered project identity: %q", ansi.Strip(localRow))
	}

	var localExpanded strings.Builder
	var localFallbackExpanded strings.Builder
	blockerExpandedView(&local.state, local, &localExpanded)
	blockerExpandedView(&localFallback.state, localFallback, &localFallbackExpanded)
	if localExpanded.String() != localFallbackExpanded.String() {
		t.Fatalf("project-local expanded Blockers changed with ColorSlot:\ncolored=%q\nfallback=%q",
			localExpanded.String(), localFallbackExpanded.String())
	}
	if strings.Contains(ansi.Strip(localExpanded.String()), "Project:") {
		t.Fatalf("project-local expanded Blockers unexpectedly rendered a project label: %q", ansi.Strip(localExpanded.String()))
	}

	var addForm strings.Builder
	var fallbackAddForm strings.Builder
	blockerAddView(&global.state, global, &addForm)
	blockerAddView(&globalFallback.state, globalFallback, &fallbackAddForm)
	if addForm.String() != fallbackAddForm.String() {
		t.Fatalf("global blocker add form changed with ColorSlot:\ncolored=%q\nfallback=%q",
			addForm.String(), fallbackAddForm.String())
	}
}
