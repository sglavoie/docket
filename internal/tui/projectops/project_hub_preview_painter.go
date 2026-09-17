package projectops

import (
	"slices"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// The Project Hub's preview pane draws Outliner task rows, and the Outliner is
// the Hub's sibling rather than its dependency. The Hub therefore declares the
// painter it needs and the shell, which reaches both, supplies this one.

// projectPreviewPainter paints one project's flattened tree with the Outliner's
// own row renderer. The Outliner it holds is never mounted: it exists so the
// pane's rows are the product's rows rather than a second row grammar that would
// drift from them.
type projectPreviewPainter struct {
	rows    []outliner.OutlinerRow
	painter *outliner.OutlinerModel
	// layout is the pane's own row geometry, measured over every row the pane
	// can scroll to rather than over the rows it currently shows, and cached
	// because the pane repaints one row at a time at one width. layoutWidth is
	// the width it was measured for; a resized pane measures again.
	layout      *chrome.ColumnLayout
	layoutWidth int
	layoutTaken bool
}

func (p *projectPreviewPainter) RowCount() int { return len(p.rows) }

func (p *projectPreviewPainter) RenderRow(index, width int) string {
	if index < 0 || index >= len(p.rows) {
		return ""
	}
	// rowWidth and width are the same pane column budget: the pane has no cursor,
	// so no row ever opens Peek and the wider measure never applies. The row
	// geometry is the Outliner's own, measured over the whole flattened tree, so
	// the pane's rows are the product's rows down to the column their badges
	// align in. The absolute row index keeps zebra striping anchored to the task
	// rather than to the scroll position.
	return p.painter.RenderOutlinerTaskRow(p.rows[index], false, width, index, p.rowLayout(width))
}

func (p *projectPreviewPainter) rowLayout(width int) *chrome.ColumnLayout {
	if !p.layoutTaken || p.layoutWidth != width {
		p.layout = p.painter.ComputeOutlinerRowLayout(p.rows, width)
		p.layoutWidth = width
		p.layoutTaken = true
	}
	return p.layout
}

// NewProjectPreviewPainter is the Hub's NewPreviewTaskPainter. Building one costs
// a few maps and a text input, which is why the Hub caches the result.
func NewProjectPreviewPainter(
	runtime *config.RuntimeState,
	project *models.Project,
	tasks []*models.Task,
) projecthub.PreviewTaskPainter {
	return &projectPreviewPainter{
		rows:    PreviewTaskRows(project.ID, tasks),
		painter: outliner.NewOutlinerModel(project, nil, runtime),
	}
}

// PreviewTaskRows flattens a project's task tree into Outliner rows in
// tree order. Every task is listed, done and canceled included: the state badge
// already tells them apart, and a pane that hid them would disagree with the
// total the header just claimed.
//
// Roots are the tasks no other task claims as a child rather than the tasks
// reporting no parent, so a task whose parent reference dangles is still listed
// once instead of disappearing between the two readings.
func PreviewTaskRows(projectID string, tasks []*models.Task) []outliner.OutlinerRow {
	if len(tasks) == 0 {
		return nil
	}
	claimed := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		for _, child := range task.Children {
			if child != nil {
				claimed[child.ID] = true
			}
		}
	}
	roots := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task != nil && !claimed[task.ID] {
			roots = append(roots, task)
		}
	}

	rows := make([]outliner.OutlinerRow, 0, len(tasks))
	visited := make(map[string]bool, len(tasks))
	var walk func(siblings []*models.Task, ancestorIsLast []bool)
	walk = func(siblings []*models.Task, ancestorIsLast []bool) {
		for index, task := range siblings {
			if task == nil || visited[task.ID] {
				continue
			}
			visited[task.ID] = true
			isLast := index == len(siblings)-1
			rows = append(rows, outliner.OutlinerRow{
				Task:               task,
				Prefix:             outliner.BuildTreePrefix(ancestorIsLast, isLast),
				HasVisibleChildren: len(task.Children) > 0,
				ProjectID:          projectID,
			})
			if len(task.Children) > 0 {
				walk(task.Children, append(slices.Clone(ancestorIsLast), isLast))
			}
		}
	}
	walk(roots, nil)
	return rows
}
