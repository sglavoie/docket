package grammar

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// renderPreview formats the parsed preview into labeled groups so the user can
// scan what the parser understood. Only populated groups are shown.
func (q *QuickCaptureInput) renderPreview() string {
	if q.PreviewErr != "" {
		return theme.ValidationErrorStyle.Render(q.PreviewErr)
	}
	groups, ok := q.previewGroups()
	if !ok {
		return ""
	}

	var lines []string
	for _, g := range groups {
		lines = append(lines, q.renderPreviewLine(g.label, strings.Join(g.chunks, " · ")))
	}
	return strings.Join(lines, "\n")
}

// renderCompactPreview renders the same recognized metadata as renderPreview but
// as a single unlabeled " · "-joined line wrapped to at most two rows. Quick
// Capture falls back to it when the full grouped preview would displace the
// scrollable syntax guide or the app-shell footer, keeping the composed view
// within its height budget without dropping what the parser understood.
func (q *QuickCaptureInput) renderCompactPreview() string {
	if q.PreviewErr != "" {
		return q.clampToRows(theme.ValidationErrorStyle.Render(q.PreviewErr), 2)
	}
	groups, ok := q.previewGroups()
	if !ok {
		return ""
	}
	var chunks []string
	for _, g := range groups {
		chunks = append(chunks, g.chunks...)
	}
	return q.clampToRows(strings.Join(chunks, " · "), 2)
}

// ComposePreviewBlock renders the live preview (or error) for a composed capture
// view, choosing the full grouped preview when it fits alongside the lower pane
// and falling back to the compact two-row form when it would otherwise displace
// the lower pane or the footer. It returns the rendered block, its row count, and
// whether a trailing blank separator should follow it (1 when non-empty). Shared
// by Quick Capture and the Linked Task Creator so both budget the preview identically.
func (q *QuickCaptureInput) ComposePreviewBlock(remaining, lowerMin int) (block string, rows, blankAfter int) {
	renderFull := q.renderFullPreviewBlock
	renderCompact := q.renderCompactPreviewBlock

	block = renderFull()
	if block != "" {
		rows = lipgloss.Height(block)
		blankAfter = 1
	}

	// Fall back to compact when a lower pane needs room the full preview would eat.
	if lowerMin > 0 && rows+blankAfter+lowerMin > remaining {
		block = renderCompact()
		rows, blankAfter = 0, 0
		if block != "" {
			rows = lipgloss.Height(block)
			blankAfter = 1
		}
	}

	// Safety net when there is no lower pane: never let the preview overflow the
	// budget. Clip trailing rows rather than push the footer off screen.
	if lowerMin == 0 && rows+blankAfter > remaining {
		lines := strings.Split(block, "\n")
		if keep := max(remaining-blankAfter, 1); len(lines) > keep {
			block = strings.Join(lines[:keep], "\n")
			rows = keep
		}
	}
	return block, rows, blankAfter
}

// renderFullPreviewBlock is the grouped preview, or the refusal that stands in
// for it. A parse error is what the preview has to say about the line.
func (q *QuickCaptureInput) renderFullPreviewBlock() string {
	if q.err != nil {
		return theme.ValidationErrorStyle.Render(q.err.Error())
	}
	return q.renderPreview()
}

// renderCompactPreviewBlock is the same content as one wrapped run of chunks.
func (q *QuickCaptureInput) renderCompactPreviewBlock() string {
	if q.err != nil {
		return q.clampToRows(theme.ValidationErrorStyle.Render(q.err.Error()), 2)
	}
	return q.renderCompactPreview()
}

// ComposeBoundedPreviewBlock renders the preview into a fixed row budget with
// no lower pane to protect. It is the panel's form of the same choice
// ComposePreviewBlock makes for a full-height view: the grouped preview while
// it fits whole, and otherwise the compact run, which wraps every recognized
// attribute rather than keeping the first few groups and silently dropping the
// rest. Clipping is the last resort, since a truncated group list reads as a
// complete one.
func (q *QuickCaptureInput) ComposeBoundedPreviewBlock(rows int) string {
	if rows < 1 {
		return ""
	}
	block := q.renderFullPreviewBlock()
	if block == "" || lipgloss.Height(block) <= rows {
		return block
	}
	return q.clampToRows(q.renderCompactPreviewBlock(), rows)
}

// PreviewBlockRows is how many rows the grouped preview would take if it were
// given every row it asked for. It reads the parse the last keystroke already
// left behind — the stored result, or the refusal standing in for it — so a host
// deciding its own height from its content never re-parses the line to learn
// what the preview wants. Nothing to preview is zero rows rather than the one
// lipgloss.Height reports for an empty string.
func (q *QuickCaptureInput) PreviewBlockRows() int {
	block := q.renderFullPreviewBlock()
	if block == "" {
		return 0
	}
	return lipgloss.Height(block)
}

// clampToRows wraps a styled string to q.width and keeps at most rows lines,
// silently dropping any overflow. Used for the best-effort compact preview,
// where losing a trailing detail is acceptable but a fixed row budget is not.
func (q *QuickCaptureInput) clampToRows(styled string, rows int) string {
	if rows < 1 {
		rows = 1
	}
	var lines []string
	if q.Width <= 0 {
		lines = strings.Split(styled, "\n")
	} else {
		lines = textfit.WrapStyled(styled, q.Width)
	}
	if len(lines) > rows {
		lines = lines[:rows]
	}
	return strings.Join(lines, "\n")
}

// renderPreviewLine emits a single "Label   body" line, wrapping the body
// when width is set so no rendered line exceeds q.width visual columns.
// Continuation lines are indented to the label column for readability.
func (q *QuickCaptureInput) renderPreviewLine(label, body string) string {
	prefix := theme.MutedStyle.Render(fmt.Sprintf("%-*s", previewGroupLabelWidth, label))
	if q.Width <= 0 {
		return prefix + body
	}
	bodyWidth := q.Width - previewGroupLabelWidth
	if bodyWidth <= 0 {
		// Width is too narrow for any body alongside the label; let the
		// label sit on its own line and wrap the body on the next.
		return prefix + "\n" + strings.Join(textfit.WrapStyled(body, q.Width), "\n")
	}
	wrapped := textfit.WrapStyled(body, bodyWidth)
	indent := strings.Repeat(" ", previewGroupLabelWidth)
	for i, line := range wrapped {
		if i == 0 {
			wrapped[i] = prefix + line
		} else {
			wrapped[i] = indent + line
		}
	}
	return strings.Join(wrapped, "\n")
}

// MatchingCaptureTasks returns unfinished title matches in the resolved project,
// with siblings first when capturing under a parent, then by short ID. Sorting
// owns a separate slice and never changes the project's task order.
func (q *QuickCaptureInput) MatchingCaptureTasks() ([]*models.Task, *models.Project) {
	if q.Preview == nil || q.PreviewErr != "" {
		return nil, nil
	}
	project := q.Project
	if q.Preview.TargetProject != nil {
		project = q.Preview.TargetProject
	}
	if project == nil {
		return nil, nil
	}
	parentID := q.Preview.ParentID
	if parentID == nil {
		parentID = q.DefaultParentID
	}
	sameParent := func(task *models.Task) bool {
		return parentID != nil && task.ParentID != nil && *task.ParentID == *parentID
	}
	var matches []*models.Task
	for _, task := range project.Tasks {
		if task != nil && !task.IsTerminalState() && strings.EqualFold(strings.TrimSpace(task.Title), strings.TrimSpace(q.Preview.Title)) {
			matches = append(matches, task)
		}
	}
	slices.SortStableFunc(matches, func(a, b *models.Task) int {
		if sameParent(a) != sameParent(b) {
			if sameParent(a) {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.ShortID, b.ShortID)
	})
	return matches, project
}

// MatchingCaptureTask resolves the preferred advisory match shown in capture.
// A repeated title is legal; matches never change whether a draft can be saved.
func (q *QuickCaptureInput) MatchingCaptureTask() (*models.Task, *models.Project, int) {
	matches, project := q.MatchingCaptureTasks()
	if len(matches) == 0 {
		return nil, project, 0
	}
	return matches[0], project, len(matches)
}

func (q *QuickCaptureInput) duplicateCaptureHint() string {
	return q.DuplicateCaptureHint()
}

// DuplicateCaptureHint shares the terminal's advisory wording with capture hosts.
func (q *QuickCaptureInput) DuplicateCaptureHint() string {
	first, project, count := q.MatchingCaptureTask()
	if first == nil {
		return ""
	}
	hint := fmt.Sprintf("Matching title: %s#%d", project.Slug, first.ShortID)
	if count > 1 {
		hint += fmt.Sprintf(" (+%d more)", count-1)
	}
	hint += " · " + models.WorkflowStateGroupForState(first.WorkflowState).Label()
	if first.ParentID != nil {
		if parent := project.GetTaskByID(*first.ParentID); parent != nil {
			hint += fmt.Sprintf(" · under #%d %s", parent.ShortID, parent.Title)
		}
	}
	return hint
}
