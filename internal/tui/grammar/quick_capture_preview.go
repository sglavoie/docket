package grammar

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// UpdatePreview parses the logical line under the caret, never combining
// attributes from separate tasks in a multiline draft.
func (q *QuickCaptureInput) UpdatePreview() {
	q.previewMinute = time.Now().Truncate(time.Minute)
	q.batchDuplicateHint = ""
	q.HostRoute = nil
	if q.HostRouteSummary != nil {
		summary := q.HostRouteSummary()
		q.HostRoute = &summary
	}
	lines := strings.Split(q.Input.Value(), "\n")
	input := strings.TrimSpace(lines[min(q.Input.Line(), len(lines)-1)])
	if input == "" {
		q.Preview = nil
		q.PreviewErr = ""
		return
	}
	q.Preview, q.PreviewErr = capturetext.ParseQuickCaptureForPreview(input, q.Project, q.allProjects, q.skipProjectSyntax)
	if q.Preview == nil || q.PreviewErr != "" || q.Preview.Title == "" {
		return
	}
	project := q.capturePreviewProject(q.Preview)
	if project == nil {
		return
	}
	for i, line := range lines {
		if i == q.Input.Line() || strings.TrimSpace(line) == "" {
			continue
		}
		other, err := capturetext.ParseQuickCapture(line, q.Project, q.allProjects, q.skipProjectSyntax)
		if err != nil {
			continue
		}
		otherProject := q.capturePreviewProject(other)
		if otherProject != nil && otherProject.ID == project.ID && strings.EqualFold(strings.TrimSpace(other.Title), strings.TrimSpace(q.Preview.Title)) {
			q.batchDuplicateHint = fmt.Sprintf("Same title on line %d", i+1)
			break
		}
	}
}

func (q *QuickCaptureInput) capturePreviewProject(result *QuickCaptureResult) *models.Project {
	if result.TargetProject != nil {
		return result.TargetProject
	}
	return q.Project
}

// previewGroupLabelWidth fits the longest group label plus a trailing space
// so the body column lines up.
const previewGroupLabelWidth = 9

// previewGroup is one labeled cluster of parsed attributes. Both the full
// grouped preview and the compact two-row fallback use the same groups so they
// can never disagree about what the parser understood.
type previewGroup struct {
	label  string
	chunks []string
}

// previewGroups builds the populated preview groups from the parsed result, or
// reports ok=false when there is nothing to preview. It is the single source of
// recognized attributes shown by both preview renderers.
func (q *QuickCaptureInput) previewGroups() (groups []previewGroup, ok bool) {
	if q.Preview == nil || q.Preview.Title == "" {
		return nil, false
	}

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
	if routed := capturetext.ComposerDeferredDestination(q.Preview, destination); routed != destination {
		destination, deferred = routed, true
	}
	previewTask := &models.Task{
		Title:         q.Preview.Title,
		WorkflowState: destination.WorkflowState(),
		Priority:      q.Preview.Priority,
		Tags:          q.Preview.Tags,
	}
	taskTitle := textfit.RenderTaskTitle(q.Preview.Title, nil, theme.TaskTitleStyleFor(previewTask, theme.TaskTitleStyleOptions{}))
	groups = append(groups, previewGroup{label: "Task", chunks: []string{taskTitle}})
	if hint := q.duplicateCaptureHint(); hint != "" {
		groups = append(groups, previewGroup{label: "Notice", chunks: []string{hint}})
	}
	if q.batchDuplicateHint != "" {
		groups = append(groups, previewGroup{label: "Notice", chunks: []string{q.batchDuplicateHint}})
	}

	if route, shown := q.previewRouteGroup(previewTask, destination, deferred); shown {
		groups = append(groups, route)
	}

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
	if len(q.Preview.DependencyRefs) > 0 {
		dependencyDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDependency)
		dependencyChunks := make([]string, len(q.Preview.DependencyRefs))
		for i, ref := range q.Preview.DependencyRefs {
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
			tagStrs[i] += captureVocabularyMarker(t, q.TagCache)
		}
		groups = append(groups, previewGroup{label: "Tags", chunks: []string{strings.Join(tagStrs, " ")}})
	}
	if q.Preview.Context != "" {
		contextDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldContext)
		groups = append(groups, previewGroup{label: "Context", chunks: []string{contextDefinition.Trigger + q.Preview.Context + captureVocabularyMarker(q.Preview.Context, q.contextCache)}})
	}

	return groups, true
}

// previewRouteGroup is the Route group, or shown=false where the host's own
// route row already says all of it.
func (q *QuickCaptureInput) previewRouteGroup(previewTask *models.Task, destination app.CaptureDestination, deferred bool) (group previewGroup, shown bool) {
	destinationDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDestination)
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
	if q.Preview.TargetProjectName != "" {
		routeChunks = append(routeChunks, "→ "+theme.ProjectColorStyle(q.Preview.TargetProject).Render(q.Preview.TargetProjectName))
	}
	if q.hostRouteStates(destination, deferred) {
		return previewGroup{}, false
	}
	return previewGroup{label: "Route", chunks: routeChunks}, true
}

// hostRouteStates reports whether the host's route row already says everything
// the Route group would: the same project and the same destination, read as one
// value. A row reading Mixed names neither, and a deferral is a landing the row
// cannot show, so both keep the group, as does a host drawing no such row.
func (q *QuickCaptureInput) hostRouteStates(destination app.CaptureDestination, deferred bool) bool {
	if q.HostRoute == nil || q.HostRoute.Mixed || deferred {
		return false
	}
	return q.HostRoute.Destination == destination && q.HostRoute.ProjectID == q.previewProjectID()
}

// previewProjectID is the project the Route group names: the line's target,
// else the host's own project.
func (q *QuickCaptureInput) previewProjectID() string {
	switch {
	case q.Preview.TargetProjectID != nil:
		return *q.Preview.TargetProjectID
	case q.Preview.TargetProject != nil:
		return q.Preview.TargetProject.ID
	case q.Project != nil:
		return q.Project.ID
	}
	return ""
}

// The marker uses the autocomplete vocabulary; tags and contexts remain separate.
func captureVocabularyMarker(value string, vocabulary []string) string {
	for _, existing := range vocabulary {
		if strings.EqualFold(value, existing) {
			return ""
		}
	}
	return theme.MutedStyle.Render(" (new)")
}

// captureDuePreview keeps the resolved weekday and clock visible before save.
// Only dates outside the current year need the extra year qualifier.
func captureDuePreview(due, now time.Time) string {
	return chrome.FormatWeekdayDate(due, now, (&models.Task{DueDate: &due}).DueHasTime())
}
