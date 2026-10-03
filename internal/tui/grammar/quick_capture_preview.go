package grammar

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// UpdatePreview updates the live preview state.
func (q *QuickCaptureInput) UpdatePreview() {
	input := strings.ReplaceAll(q.Input.Value(), "\n", " ")
	input = strings.TrimSpace(input)
	if input == "" {
		q.Preview = nil
		q.PreviewErr = ""
		return
	}
	q.Preview, q.PreviewErr = ParseQuickCaptureForPreview(input, q.Project, q.allProjects, q.skipProjectSyntax)
}

// previewGroupLabelWidth is the padded width used for the leading group label
// column ("Task", "Route", "Children", "Schedule", "Priority", "Tags"). Sized to
// the longest label plus a trailing space so the body column lines up.
const previewGroupLabelWidth = 9

// previewGroup is one labeled cluster of parsed attributes (Task, Route,
// Children, Schedule, Priority, Tags). Both the full grouped preview and the
// compact two-row fallback are built from the same groups so they can never
// disagree about what the parser understood.
type previewGroup struct {
	label  string
	chunks []string
}

// previewGroups builds the populated preview groups from the parsed result, or
// reports ok=false when there is nothing to preview. It is the single source of
// the recognized task, route, child, schedule, state, priority, tag, and project
// information shown by both preview renderers.
func (q *QuickCaptureInput) previewGroups() (groups []previewGroup, ok bool) {
	if q.Preview == nil || q.Preview.Title == "" {
		return nil, false
	}

	destinationDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDestination)
	dueDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDue)
	priorityDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPriority)
	tagDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldTag)
	childDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldChild)
	pinnedDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPinned)
	reviewDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldReview)

	destination := app.CaptureDestinationInbox
	if q.Preview.ParentID != nil {
		destination = app.CaptureDestinationBacklog
	}
	if q.Preview.DestinationSet {
		destination = q.Preview.Destination
	}
	// The deferral route runs through the same helper the submitted request uses,
	// so the preview never promises a landing place the capture will not honor.
	deferred := false
	if routed := ComposerDeferredDestination(q.Preview, destination); routed != destination {
		destination, deferred = routed, true
	}
	previewTask := &models.Task{
		Title:         q.Preview.Title,
		WorkflowState: destination.WorkflowState(),
		Priority:      q.Preview.Priority,
		Tags:          q.Preview.Tags,
	}
	taskTitle := theme.TaskTitleStyleFor(previewTask, theme.TaskTitleStyleOptions{}).Render(q.Preview.Title)
	groups = append(groups, previewGroup{label: "Task", chunks: []string{taskTitle}})

	destinationLabel := destinationDefinition.Trigger + string(destination)
	switch {
	case deferred:
		destinationLabel += " (deferred)"
	case !q.Preview.DestinationSet:
		destinationLabel += " (default)"
	}
	// The glyph is the row's own state badge, so a capture previews the mark
	// its row will wear, and the label wears the same inline badge as the
	// composer's route row. A planning sigil answers two questions with one
	// token — `$today` is a membership in a collection and a Backlog status —
	// but the route row directly above this preview already states that pair
	// under its Plan and Status labels, so the preview names the token alone
	// rather than saying the same two words a second time.
	routeChunks := []string{theme.GetTaskStateBadge(previewTask) + " " + CapturePlanAccentStyle(destination).Render(destinationLabel)}
	if q.Preview.targetProjectName != "" {
		routeChunks = append(routeChunks, "→ "+theme.ProjectColorStyle(q.Preview.targetProject).Render(q.Preview.targetProjectName))
	}
	groups = append(groups, previewGroup{label: "Route", chunks: routeChunks})

	// Children sits directly after Route because both answer where the task lands.
	// The parser already deduped aliases in first-reference order, so one chunk per
	// entry both survives the compact fallback's flattening and states the count.
	if len(q.Preview.Children) > 0 {
		childChunks := make([]string, len(q.Preview.Children))
		for i, child := range q.Preview.Children {
			childChunks[i] = fmt.Sprintf("%s%d %s", childDefinition.Trigger, child.ShortID, child.Title)
		}
		groups = append(groups, previewGroup{label: "Children", chunks: childChunks})
	}

	// Depends follows Children because both name other tasks, and it states one
	// chunk per reference for the same reason: the parser already deduped aliases
	// in first-reference order, so the group is also the count.
	if len(q.Preview.dependencies) > 0 {
		dependencyDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDependency)
		dependencyChunks := make([]string, len(q.Preview.dependencies))
		for i, ref := range q.Preview.dependencies {
			dependencyChunks[i] = dependencyDefinition.Trigger + ref.Label()
		}
		groups = append(groups, previewGroup{label: "Depends", chunks: dependencyChunks})
	}

	var scheduleChunks []string
	if q.Preview.DueDate != nil {
		scheduleChunks = append(scheduleChunks, dueDefinition.Trigger+captureDuePreview(*q.Preview.DueDate, time.Now()))
	}
	if q.Preview.DeferUntil != nil {
		deferDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDefer)
		scheduleChunks = append(scheduleChunks, deferDefinition.Trigger+chrome.FormatDeferDate(*q.Preview.DeferUntil, time.Now()))
	}
	if len(scheduleChunks) > 0 {
		groups = append(groups, previewGroup{label: "Schedule", chunks: scheduleChunks})
	}

	if q.Preview.Priority != models.PriorityUnset {
		groups = append(groups, previewGroup{label: "Priority", chunks: []string{fmt.Sprintf("%s%d", priorityDefinition.Trigger, q.Preview.Priority)}})
	}
	if q.Preview.Pinned {
		groups = append(groups, previewGroup{label: "Pinned", chunks: []string{pinnedDefinition.Trigger + "pin"}})
	}
	if q.Preview.Review {
		groups = append(groups, previewGroup{label: "Review", chunks: []string{reviewDefinition.Trigger + "review"}})
	}
	// Attachments sit below the metadata groups and one chunk per entry, so the
	// preview states how many the line would create. Without this group a pasted
	// URL would vanish from the title with nothing said in its place.
	if len(q.Preview.Attachments) > 0 {
		attachmentDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldAttachment)
		attachmentChunks := make([]string, len(q.Preview.Attachments))
		for i, value := range q.Preview.Attachments {
			attachmentChunks[i] = attachmentDefinition.Trigger + value
		}
		groups = append(groups, previewGroup{label: "Attach", chunks: attachmentChunks})
	}
	// Comments sit beside Attachments for the same reason: the span is peeled out
	// of the title, so without this group a comment whose closing delimiter is
	// still missing would be indistinguishable from one already recognized.
	if len(q.Preview.Comments) > 0 {
		commentDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldComment)
		commentChunks := make([]string, len(q.Preview.Comments))
		for i, text := range q.Preview.Comments {
			commentChunks[i] = commentDefinition.Trigger + text + commentDefinition.Trigger
		}
		groups = append(groups, previewGroup{label: "Comments", chunks: commentChunks})
	}
	if len(q.Preview.Tags) > 0 {
		tagStrs := make([]string, len(q.Preview.Tags))
		for i, t := range q.Preview.Tags {
			tagStrs[i] = theme.TagChipStyle(t).Render(tagDefinition.Trigger + t)
		}
		groups = append(groups, previewGroup{label: "Tags", chunks: []string{strings.Join(tagStrs, " ")}})
	}

	return groups, true
}

// captureDuePreview keeps the resolved weekday and clock visible before save.
// Only dates outside the current year need the extra year qualifier.
func captureDuePreview(due, now time.Time) string {
	layout := "Mon, Jan 2"
	if due.Year() != now.Year() {
		layout += " 2006"
	}
	value := due.Format(layout)
	if clock := chrome.FormatDueClock(&models.Task{DueDate: &due}); clock != "" {
		value += " " + clock
	}
	return value
}

// renderPreview formats the parsed preview for display. The output is chunked
// into labeled groups (Task / Route / Children / Schedule / Priority / Tags) so
// the user can scan what the parser understood without parsing a
// separator-soup. Only populated groups are shown.
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
