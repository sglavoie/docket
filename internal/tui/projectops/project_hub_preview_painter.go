package projectops

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The Project Hub's preview pane draws Outliner task rows, and the Outliner is
// the Hub's sibling rather than its dependency. The Hub therefore declares the
// painter it needs and the shell, which reaches both, supplies this one.

// projectPreviewPainter paints one project's flattened tree with the Outliner's
// own row renderer. The Outliner it holds is never mounted: it exists so the
// pane's rows are the product's rows rather than a second row grammar that would
// drift from them.
type projectPreviewPainter struct {
	rows []outliner.OutlinerRow
	// settled is the one row standing for every terminal task, drawn after the
	// open tree and absent when the project holds none.
	settled PreviewSettledTally
	painter *outliner.OutlinerModel
	// layout is the pane's own row geometry, measured over every row the pane
	// can scroll to rather than over the rows it currently shows, and cached
	// because the pane repaints one row at a time at one width. layoutWidth is
	// the width it was measured for; a resized pane measures again.
	layout      *chrome.ColumnLayout
	layoutWidth int
	layoutTaken bool
}

func (p *projectPreviewPainter) RowCount() int {
	if p.settled.Empty() {
		return len(p.rows)
	}
	return len(p.rows) + 1
}

func (p *projectPreviewPainter) RenderRow(index, width int) string {
	if index == len(p.rows) && !p.settled.Empty() {
		// The summary is not a task row, so it takes no part in the row layout
		// and no tree connector: it is muted text the pane's width bounds.
		return theme.MutedStyle.Render(textfit.TruncateToVisualWidth(p.settled.Label(), width))
	}
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
	rows, settled := PreviewTaskRows(project.ID, tasks)
	return &projectPreviewPainter{
		rows:    rows,
		settled: settled,
		painter: outliner.NewOutlinerModel(project, nil, runtime),
	}
}

// PreviewSettledTally counts the terminal tasks the pane folds into its one
// summary row.
type PreviewSettledTally struct {
	Done     int
	Canceled int
}

// Empty reports whether there is no settled work to summarize.
func (t PreviewSettledTally) Empty() bool { return t.Done == 0 && t.Canceled == 0 }

// Label is the summary row's text: `✓ N done`, then `N canceled` when any are.
func (t PreviewSettledTally) Label() string {
	label := fmt.Sprintf("✓ %d done", t.Done)
	if t.Canceled > 0 {
		label += fmt.Sprintf(" · %d canceled", t.Canceled)
	}
	return label
}

// PreviewTaskRows flattens a project's open task tree into Outliner rows in
// tree order and tallies the terminal tasks rather than listing them: finished
// work is what a project is least read for, so it costs the pane one summary
// row instead of a row each, and the open rows plus that tally still add up to
// the total the header just claimed. A terminal task's subtree is terminal too,
// since Done and Canceled reject a nonterminal child, so skipping one hides no
// open work. Tree connectors are measured over the open siblings alone.
//
// Roots are the tasks no other task claims as a child rather than the tasks
// reporting no parent, so a task whose parent reference dangles is still listed
// once instead of disappearing between the two readings.
func PreviewTaskRows(projectID string, tasks []*models.Task) ([]outliner.OutlinerRow, PreviewSettledTally) {
	if len(tasks) == 0 {
		return nil, PreviewSettledTally{}
	}
	var settled PreviewSettledTally
	counted := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		if task == nil || counted[task.ID] {
			continue
		}
		counted[task.ID] = true
		switch task.WorkflowState {
		case models.StateDone:
			settled.Done++
		case models.StateCanceled:
			settled.Canceled++
		default:
		}
	}
	open := func(siblings []*models.Task) []*models.Task {
		kept := make([]*models.Task, 0, len(siblings))
		for _, task := range siblings {
			if task != nil && !task.IsTerminalState() {
				kept = append(kept, task)
			}
		}
		return kept
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
		siblings = open(siblings)
		for index, task := range siblings {
			if task == nil || visited[task.ID] {
				continue
			}
			visited[task.ID] = true
			isLast := index == len(siblings)-1
			hasOpenChildren := len(open(task.Children)) > 0
			rows = append(rows, outliner.OutlinerRow{
				Task:               task,
				Prefix:             outliner.BuildTreePrefix(ancestorIsLast, isLast),
				HasVisibleChildren: hasOpenChildren,
				ProjectID:          projectID,
			})
			if hasOpenChildren {
				walk(task.Children, append(slices.Clone(ancestorIsLast), isLast))
			}
		}
	}
	walk(roots, nil)
	return rows, settled
}
