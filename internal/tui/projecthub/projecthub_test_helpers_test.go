package projecthub

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/textfit"
)

// A test helper cannot cross a package boundary, so the shell's doubles are
// declared here too rather than one side reaching for the other.

func syntheticKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func syntheticCtrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

func guidanceEntryText(spec chrome.GuidanceSpec) string {
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, " · ")
}

func assertGuidanceContains(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	if !ok {
		t.Fatalf("%s guidance is unavailable", state)
	}
	got := guidanceEntryText(spec)
	for _, entry := range want {
		if !strings.Contains(got, entry) {
			t.Errorf("%s guidance = %q, want %q", state, got, entry)
		}
	}
}

func assertRequiredGuidance(t *testing.T, state string, spec chrome.GuidanceSpec, want ...string) {
	t.Helper()
	assertGuidanceContains(t, state, spec, true, want...)
	for _, expected := range want {
		found := false
		for _, hint := range spec.Hints {
			if chrome.GuidanceHintText(hint) == expected && hint.Required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s guidance does not require %q: %+v", state, expected, spec.Hints)
		}
	}
}

func assertGuidanceFits(t *testing.T, spec chrome.GuidanceSpec, width int) {
	t.Helper()
	lines := chrome.GuidanceLines(spec, width)
	if len(lines) > max(spec.Rows, 1) {
		t.Fatalf("guidance uses %d lines with row budget %d: %v", len(lines), spec.Rows, lines)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("guidance line width = %d, want <= %d: %q", got, width, line)
		}
	}
}

func newTestConfig() *config.RuntimeState {
	return config.NewRuntimeState("", config.NewDomains())
}

// scopeTestProject mints a catalog-shaped project holding backlog tasks, the
// same shape the shell's own fixture builds.
func scopeTestProject(id, name string, taskIDs ...string) *models.Project {
	tasks := make([]*models.Task, 0, len(taskIDs))
	for i, taskID := range taskIDs {
		tasks = append(tasks, &models.Task{
			ID:            taskID,
			ShortID:       i + 1,
			Title:         taskID,
			WorkflowState: models.StateBacklog,
		})
	}
	project := &models.Project{
		ID:          id,
		Name:        name,
		Slug:        id,
		Tasks:       tasks,
		NextShortID: len(tasks) + 1,
	}
	project.BuildTaskTree()
	return project
}

func markDone(task *models.Task) {
	now := time.Now()
	task.ApplyWorkflowState(models.StateDone)
	task.CompletedAt = &now
}

// projectHubFixture stands in for the shell that mounts the Hub. The Hub reads
// its population from the runtime catalog and its per-project tallies from
// SetTaskCounts, so supplying both -- plus the operation system the shell binds
// -- is the whole of what mounting provides. Nothing here reaches an AppModel,
// which is what keeps the Hub's own coverage inside its own package.
type projectHubFixture struct {
	t *testing.T
	// runtime carries the catalog the Hub lists.
	runtime *config.RuntimeState
	// counted is the subset of projects the shell's count pass could read. A
	// project left out reaches the Hub with an unknown tally, exactly as one
	// whose tasks.md failed to parse does.
	counted map[string]bool
	// projects preserves catalog order so a later add rebuilds the same list.
	projects []*models.Project
	// bulkTags records the last bulk tag message the sheet produced, which is
	// where the shell's own handler would have taken over.
	bulkTags *ProjectBulkTagsMsg
}

// newProjectHubFixture catalogs the given projects and counts every one of
// them. A project added later with addUncounted keeps its tally unknown.
func newProjectHubFixture(t *testing.T, projects ...*models.Project) *projectHubFixture {
	t.Helper()
	fixture := &projectHubFixture{
		t:       t,
		runtime: newTestConfig(),
		counted: make(map[string]bool),
	}
	for _, project := range projects {
		fixture.add(project)
	}
	return fixture
}

func (f *projectHubFixture) add(project *models.Project) {
	f.t.Helper()
	f.counted[project.ID] = true
	f.catalog(project)
}

// addUncounted catalogs a project the count pass could not read, so its Hub row
// reports an unknown tally rather than a zero.
func (f *projectHubFixture) addUncounted(project *models.Project) {
	f.t.Helper()
	f.catalog(project)
}

func (f *projectHubFixture) catalog(project *models.Project) {
	f.t.Helper()
	f.projects = append(f.projects, project)
	f.sync()
}

// sync rewrites the catalog from the fixture's projects. A test that edits one
// in place has to call it before the Hub reads the change back: the Hub lists
// rehydrated catalog records rather than the pointers it was handed, exactly as
// it does in the shell.
func (f *projectHubFixture) sync() {
	f.t.Helper()
	catalog := config.NewWorkspaceCatalog()
	for _, existing := range f.projects {
		if err := catalog.UpsertProject(existing); err != nil {
			f.t.Fatalf("UpsertProject(%q): %v", existing.ID, err)
		}
	}
	f.runtime.SetCatalogInMemory(catalog)
}

// tallies mirrors the shell's count pass: every counted project contributes the
// tally its loaded tree reports, and the rest are absent from the map.
func (f *projectHubFixture) tallies() map[string]ProjectTaskTally {
	counts := make(map[string]ProjectTaskTally, len(f.projects))
	for _, project := range f.projects {
		if f.counted[project.ID] {
			counts[project.ID] = ProjectTaskTallyOf(project)
		}
	}
	return counts
}

// open mounts a fresh Hub over the fixture's current catalog, wired to the same
// operation system the shell binds.
func (f *projectHubFixture) open(modes ...route.ProjectHubMode) *ProjectHubModel {
	f.t.Helper()
	hub := NewProjectHubModel(f.runtime, modes...)
	registry, executor := f.operationSystem(hub)
	hub.SetOperationSystem(registry, executor, func(
		strategyID operation.InputStrategyID,
		sequence []string,
		ctx operation.CommandContext,
	) (bool, tea.Cmd) {
		binding, _, resolution := registry.InputBindingForSequence(strategyID, sequence, ctx)
		if resolution != operation.ChordTerminal {
			return true, nil
		}
		result := executor.Execute(operation.OperationExecutionRequest{
			ID:       binding.OperationID,
			Context:  ctx,
			Value:    binding.PresetValue,
			HasValue: binding.HasPresetValue,
		})
		switch typed := result.AsMessage().(type) {
		case nil:
			return true, nil
		case operation.OperationCommandResultMsg:
			return true, typed.Cmd
		default:
			return true, func() tea.Msg { return typed }
		}
	})
	hub.SetPreviewTaskPainter(newStubPreviewPainter)
	hub.SetTaskCounts(f.tallies())
	hub.SetSize(120, 30)
	return hub
}

// stubPreviewPainter paints one line per task. The production painter is the
// Outliner's own row renderer, which sits beside this package rather than below
// it, and its row grammar is asserted where it lives; what the pane's paging
// arithmetic needs from a painter is a row count and one identifiable line per
// row.
type stubPreviewPainter struct {
	titles []string
}

func newStubPreviewPainter(
	_ *config.RuntimeState,
	_ *models.Project,
	tasks []*models.Task,
) PreviewTaskPainter {
	painter := stubPreviewPainter{titles: make([]string, 0, len(tasks))}
	for _, task := range tasks {
		painter.titles = append(painter.titles, task.Title)
	}
	return painter
}

func (p stubPreviewPainter) RowCount() int { return len(p.titles) }

func (p stubPreviewPainter) RenderRow(index, width int) string {
	return textfit.TruncateToVisualWidth(p.titles[index], width)
}

// operationSystem registers the Hub's surface grammar, the project operations a
// row carries, and the bulk tag set, binding each to the smallest handler that
// still runs the real registry and executor. The shell's own handlers name an
// AppModel; what the Hub needs from them is the message, so these produce it.
func (f *projectHubFixture) operationSystem(
	hub *ProjectHubModel,
) (*operation.OperationRegistry, *operation.OperationExecutor) {
	f.t.Helper()
	registry := operation.NewOperationRegistry()
	executor := operation.NewOperationExecutor(registry)
	register := func(descriptor operation.OperationDescriptor, binding operation.OperationBinding) {
		if err := registry.Register(descriptor); err != nil {
			f.t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
		binding.OperationID = descriptor.ID
		if err := executor.Register(binding); err != nil {
			f.t.Fatalf("Register binding(%s): %v", descriptor.ID, err)
		}
	}

	for _, descriptor := range ProjectHubOperationDescriptors() {
		register(descriptor, operation.OperationBinding{
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilityProjectHub),
			Handler:                  f.hubSurfaceHandler(hub),
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		})
	}
	for _, descriptor := range ProjectOperationDescriptors() {
		descriptor.Applicability = f.projectOperationApplicability(descriptor.ID)
		register(descriptor, operation.OperationBinding{
			SingleHandler:         f.projectOperationHandler,
			DelegateValueProvider: true,
			DelegateTransaction:   true,
			DelegateConfirmation:  true,
		})
	}
	for _, descriptor := range ProjectBulkOperationDescriptors() {
		register(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityProjectHub),
			BulkHandler:          f.projectBulkHandler,
			DelegateConfirmation: true,
			DelegateTransaction:  true,
		})
	}
	// The Hub's strategy documents the shared cursor, undo, help, and Esc
	// operations without dispatching them, so the registry has to know them for
	// the strategy to validate. They carry no binding here: the Hub answers all
	// of them from its own Update, and the shell owns the ledger.
	for _, descriptor := range operation.CoreInputOperationDescriptors() {
		if err := registry.Register(descriptor); err != nil {
			f.t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	if err := registry.RegisterInputStrategy(ProjectHubInputStrategy()); err != nil {
		f.t.Fatalf("RegisterInputStrategy: %v", err)
	}
	return registry, executor
}

// hubSurfaceHandler is the shell's executeProjectHubOperation over the one Hub
// this fixture mounted.
//
//nolint:exhaustive // Only Project Hub surface operations are registered with this handler.
func (f *projectHubFixture) hubSurfaceHandler(
	hub *ProjectHubModel,
) func(operation.OperationInvocation) operation.OperationHandlerResult {
	return func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
		switch invocation.Descriptor.ID {
		case OperationProjectHubSearch:
			return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: hub.ActivateSearch()}}
		case OperationProjectHubToggleSelection:
			hub.ToggleProjectHubSelection()
		case OperationProjectHubSelectAll:
			hub.SelectEveryMatchingProject()
		case OperationProjectHubActions:
			hub.OpenProjectHubActions()
		case OperationProjectHubTaskFilter:
			hub.CycleProjectHubTaskFilter()
		case OperationProjectHubPreviewPageDown:
			hub.PageProjectPreview(1)
		case OperationProjectHubPreviewPageUp:
			hub.PageProjectPreview(-1)
		case OperationProjectHubSplitLeft:
			hub.ShiftPreviewSplit(-1)
		case OperationProjectHubSplitRight:
			hub.ShiftPreviewSplit(1)
		}
		return operation.OperationHandlerResult{}
	}
}

func (f *projectHubFixture) projectByID(id string) *models.Project {
	for _, project := range f.projects {
		if project.ID == id {
			return project
		}
	}
	return nil
}

func (f *projectHubFixture) projectOperationApplicability(
	id operation.OperationID,
) operation.OperationApplicability {
	requirement, gated := ProjectOperationRequirements[id]
	return func(ctx operation.CommandContext) bool {
		if !ProjectOperationApplicable(ctx) {
			return false
		}
		project := f.projectByID(operation.CurrentEntityIdentity(ctx).ProjectID)
		if project == nil || !gated {
			return true
		}
		switch {
		case requirement.RequireArchived && !project.Archived:
			return false
		case requirement.RequireActive && project.Archived:
			return false
		}
		return true
	}
}

// openProjectRequest stands in for the scope message the shell answers Enter
// with, which is the shell's own type. What the Hub owes is dispatching Open
// against the row under the cursor; where that scope lands is asserted beside
// the shell.
type openProjectRequest struct {
	ProjectID string
}

// projectOperationHandler answers with the request message the shell would
// route. Nothing here commits: what the Hub owns is raising the request.
//
//nolint:exhaustive // Every other project operation shares the edit-request shape.
func (f *projectHubFixture) projectOperationHandler(
	invocation operation.OperationInvocation,
	target operation.OperationTarget,
) operation.OperationHandlerResult {
	project := f.projectByID(target.ProjectID)
	if project == nil {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("project %s is no longer available", target.ProjectID),
		}
	}
	switch invocation.Descriptor.ID {
	case operation.OperationProjectSetTags:
		return ProjectTagsOperationResult(invocation, project)
	case operation.OperationProjectOpen:
		return operation.OperationHandlerResult{Message: openProjectRequest{ProjectID: project.ID}}
	}
	return operation.OperationHandlerResult{Message: EditProjectMsg{Project: project}}
}

func (f *projectHubFixture) projectBulkHandler(
	invocation operation.OperationInvocation,
	targets []operation.OperationTarget,
) operation.OperationHandlerResult {
	message := ProjectBulkTagsMsg{
		ProjectIDs: make([]string, 0, len(targets)),
		Action:     ProjectBulkTagActions[invocation.Descriptor.ID],
	}
	for _, target := range targets {
		message.ProjectIDs = append(message.ProjectIDs, target.ProjectID)
	}
	if tag, ok := invocation.Value.(string); ok {
		message.Tag = tag
	}
	f.bulkTags = &message
	return operation.OperationHandlerResult{Message: message}
}

func focusProjectHubItem(t *testing.T, hub *ProjectHubModel, id listsurface.ListItemID) {
	t.Helper()
	if hub.surface == nil || !hub.surface.SetCursorToID(id) {
		t.Fatalf("project hub omitted item %q", id)
	}
}

func projectHubHasItem(hub *ProjectHubModel, id listsurface.ListItemID) bool {
	for _, item := range hub.surface.Items() {
		if item.ID == id {
			return true
		}
	}
	return false
}

func projectHubItem(t *testing.T, hub *ProjectHubModel, id listsurface.ListItemID) listsurface.ListItem {
	t.Helper()
	for _, item := range hub.surface.Items() {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("project hub omitted item %q", id)
	return listsurface.ListItem{}
}

// projectHubPreviewSummary reads the preview header's one muted meta line, which
// carries the lifecycle, the tally, the slug, and a pin when there is one. The
// five labeled fields it replaced spent six rows saying the same thing and left
// the task pane below them nowhere to go.
func projectHubPreviewSummary(t *testing.T, hub *ProjectHubModel, id listsurface.ListItemID) string {
	t.Helper()
	preview := projectHubItem(t, hub, id).Preview
	if len(preview.Fields) > 0 {
		t.Fatalf("project hub preview for %q carries %d labeled fields, want the meta line alone", id, len(preview.Fields))
	}
	if preview.Summary != "" {
		t.Fatalf("project hub preview for %q carries a joined Summary, want segments", id)
	}
	return strings.Join(preview.SummarySegments, " · ")
}

func projectHubActionIDs(hub *ProjectHubModel) []operation.OperationID {
	if hub.actions == nil || hub.actions.Surface == nil {
		return nil
	}
	ids := make([]operation.OperationID, 0)
	for _, item := range hub.actions.Surface.Items() {
		if descriptor, ok := item.Data.(operation.OperationDescriptor); ok {
			ids = append(ids, descriptor.ID)
		}
	}
	return ids
}
