package projecthub

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// PreviewTaskPainter paints one project's flattened task tree into the Hub's
// preview pane. The pane draws the product's own task rows rather than a second
// row grammar that would drift from them, and those rows are the Outliner's: the
// row type, the tree prefix and the renderer are a type and methods on
// OutlinerModel, which sits beside this package rather than below it. So the Hub
// is handed a painter over one project's tasks instead of naming a renderer it
// cannot reach.
type PreviewTaskPainter interface {
	// RowCount is how many task rows the previewed project flattened to.
	RowCount() int
	// RenderRow paints one row at the pane's column budget.
	RenderRow(index, width int) string
}

// NewPreviewTaskPainter builds the painter for one project's tasks. Building one
// costs a few maps and a text input, so the Hub caches the result rather than
// calling this per frame.
type NewPreviewTaskPainter func(
	runtime *config.RuntimeState,
	project *models.Project,
	tasks []*models.Task,
) PreviewTaskPainter

// projectHubPreviewState is the preview pane's scroll position and paint cache
// for one project.
type projectHubPreviewState struct {
	projectID string
	// built separates "no painter because the cache is cold" from "no painter
	// because none could be made", so a Hub with no painter factory draws an empty
	// pane instead of rebuilding the cache every frame.
	built bool
	// loaded distinguishes a project with no tasks from one whose tasks could not
	// be read. Drawing both as an empty pane would report the second as the first.
	loaded bool
	offset int
	// pageSize is the pane's row budget, recorded by the render because only the
	// render knows how many rows the header left. The key handler would otherwise
	// have to guess at a height it never sees.
	pageSize int
	painter  PreviewTaskPainter
}

// rowCount reads the cache's row total through a painter that may be absent.
func (s *projectHubPreviewState) rowCount() int {
	if s.painter == nil {
		return 0
	}
	return s.painter.RowCount()
}

// invalidate drops the cache so the next frame re-flattens. The Hub calls it
// whenever the project population is rebuilt, since the tasks behind a row may
// have changed underneath a cached tree.
func (s *projectHubPreviewState) invalidate() {
	*s = projectHubPreviewState{}
}

// previewTaskSource resolves the tasks the pane should list for the cursor's
// project, and whether they are known at all. The tally is preferred over the
// row's own project because a parked or archived project reaches the Hub as a
// rehydrated catalog record holding no tasks, while the count pass has already
// read its tree from disk.
func (m *ProjectHubModel) previewTaskSource() (*models.Project, []*models.Task, bool) {
	entry, ok := m.selectedProjectHubEntry()
	if !ok || entry.Project == nil {
		return nil, nil, false
	}
	project := entry.Project
	if tally, ok := m.taskCounts[project.ID]; ok && (len(tally.Tasks) > 0 || tally.Total == 0) {
		return project, tally.Tasks, true
	}
	if project.IsLoaded() {
		return project, project.Tasks, true
	}
	return project, nil, false
}

// renderProjectPreviewTasks fills the rows the preview header left with the
// cursor project's tasks. It is the ListSurfaceRenderConfig.PreviewBody hook, so
// height is the pane's remaining capacity and this is the only place that knows
// it: the scroll offset is therefore clamped here rather than where Ctrl+F moved
// it, the same arrangement the list pane's own viewport sizing uses.
func (m *ProjectHubModel) renderProjectPreviewTasks(width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	project, tasks, loaded := m.previewTaskSource()
	if project == nil {
		return nil
	}
	m.syncProjectPreview(project, tasks, loaded)
	if !m.preview.loaded {
		return []string{theme.MutedStyle.Render(textfit.TruncateToVisualWidth("Tasks not loaded.", width))}
	}
	if m.preview.rowCount() == 0 {
		return []string{theme.MutedStyle.Render(textfit.TruncateToVisualWidth("No tasks.", width))}
	}

	start, count, above, below := listsurface.ListPreviewScrollWindow(m.preview.rowCount(), m.preview.offset, height)
	m.preview.offset = start
	// A page is the row budget less both continuation rows, not the rows this
	// frame happened to show. The visible count changes by one as continuations
	// come and go, and paging by it would leave Ctrl+B a row short of where Ctrl+F
	// started. Spending both rows always costs a row of overlap instead, which
	// reads as context rather than as drift.
	m.preview.pageSize = max(height-2, 1)
	lines := make([]string, 0, height)
	if above {
		lines = append(lines, listsurface.RenderListContinuation("↑", start, width))
	}
	for index := start; index < start+count; index++ {
		lines = append(lines, m.preview.painter.RenderRow(index, width))
	}
	if below {
		lines = append(lines, listsurface.RenderListContinuation("↓", m.preview.rowCount()-start-count, width))
	}
	return lines
}

// syncProjectPreview rebuilds the cache when the cursor reaches another project,
// and resets the scroll with it: an offset taken from the previous project's
// length would open the new one part-way down for no reason the user can see.
func (m *ProjectHubModel) syncProjectPreview(project *models.Project, tasks []*models.Task, loaded bool) {
	if m.preview.projectID == project.ID && m.preview.built {
		return
	}
	m.preview = projectHubPreviewState{
		projectID: project.ID,
		built:     true,
		loaded:    loaded,
	}
	if m.newPainter != nil {
		m.preview.painter = m.newPainter(m.runtime, project, tasks)
	}
}

// PageProjectPreview moves the pane a full page, vim's Ctrl+F and Ctrl+B. Only
// the lower bound is applied here: the upper one needs the pane's row capacity,
// which the render pass owns, so an offset past the tail is corrected there
// rather than guessed at from a stale page size.
func (m *ProjectHubModel) PageProjectPreview(direction int) {
	if direction == 0 {
		return
	}
	m.preview.offset = max(m.preview.offset+direction*max(m.preview.pageSize, 1), 0)
}

// previewTasksScrollable gates the Ctrl+F/B hint on the pane being on screen and
// holding tasks at all. It is deliberately not a "has more than one page" test:
// the pane's height is settled during the render this footer is part of, so a
// page count here would be one frame stale.
func (m *ProjectHubModel) previewTasksScrollable() bool {
	if m.surface == nil || m.actions != nil {
		return false
	}
	if _, previewWidth := listsurface.ListSurfacePreviewGeometry(m.projectHubRenderConfig(0)); previewWidth <= 0 {
		return false
	}
	_, tasks, loaded := m.previewTaskSource()
	return loaded && len(tasks) > 0
}
