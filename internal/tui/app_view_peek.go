package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const inspectorWideBreakpoint = 100

// renderPeekOverlay renders the narrow-terminal inspector snapshot. The
// cursor-following wide pane and this frozen overlay both use
// renderTaskInspectorSummary; only their surrounding chrome differs.
func (m *AppModel) renderPeekOverlay() string {
	if !m.peekActive || m.peekTask == nil {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	title := titleStyle.Render("Inspector")
	footerText := HelpStyle.Render("Esc close")

	outerWidth := min(max(m.width*88/100, 36), m.width-2)
	outerHeight := min(max(m.height*88/100, 12), m.height-2)

	innerWidth := max(outerWidth-2, 16)
	// renderTitledBox uses one-cell borders plus one-cell horizontal padding,
	// so the content itself must be four cells narrower than the styled width.
	contentWidth := max(innerWidth-4, 8)
	contentHeight := max(outerHeight-2, 3)
	summary := renderTaskInspectorSummary(m.peekTask, m.peekProject, contentWidth, contentHeight)

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	return renderTitledBox(summary, innerWidth, title, footerText, borderStyle)
}

// dismissPeek clears all peek state.
func (m *AppModel) dismissPeek() {
	peekID := ""
	if m.peekTask != nil {
		peekID = m.peekTask.ID
	}
	m.peekTask = nil
	m.peekProject = nil
	m.peekActive = false
	m.peekLeftOffset = 0
	m.peekRightOffset = 0
	m.peekFocusRight = false
	m.peekLeftLines = 0
	m.peekRightLines = 0
	m.peekPaneHeight = 0
	m.closeTypedOverlay(OverlayPeek, peekID)
}

func (m *AppModel) showInspectorOverlay(task *models.Task, project *models.Project) {
	if task == nil || project == nil || m.width >= inspectorWideBreakpoint {
		return
	}
	m.dismissPeek()
	m.peekTask = task
	m.peekProject = project
	m.peekActive = true
	m.pushTypedOverlay(OverlayPeek, task.ID)
}

// renderTaskInspectorSummary is the one inspector content renderer. Every line
// is ANSI/Unicode-width bounded and the complete block is height bounded, so a
// long description or malformed optional metadata cannot expand either host.
func renderTaskInspectorSummary(task *models.Task, project *models.Project, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if task == nil {
		return fitInspectorLines(
			[]string{BoldStyle.Render("Inspector"), "", MutedStyle.Render("No task selected")},
			width,
			height,
		)
	}

	lines := []string{BoldStyle.Render("Inspector")}
	lines = appendInspectorField(lines, "Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title), width)
	lines = appendInspectorField(lines, "Path", inspectorTaskPath(task, project), width)

	state := task.State()
	stateText := state.String()
	if stateText == "" {
		stateText = "unknown"
	} else {
		stateText = strings.ReplaceAll(stateText, "_", " ")
	}
	lines = appendInspectorField(lines, "State", stateText, width)
	lines = appendInspectorField(lines, "Priority", inspectorPriority(task.Priority), width)

	due := "none"
	if task.DueDate != nil {
		due = task.DueDate.Format("2006-01-02")
	}
	lines = appendInspectorField(lines, "Due", due, width)
	snooze := "none"
	if task.SnoozedUntil != nil {
		snooze = task.SnoozedUntil.Format("2006-01-02")
	}
	lines = appendInspectorField(lines, "Snooze", snooze, width)

	tags := "none"
	if len(task.Tags) > 0 {
		tags = strings.Join(task.Tags, ", ")
	}
	lines = appendInspectorField(lines, "Tags", tags, width)
	lines = appendInspectorField(lines, "Dependencies", inspectorDependencies(task, project), width)

	recurrence := "none"
	if task.Recurrence != nil {
		if text := task.Recurrence.String(); text != "" {
			recurrence = text
		}
	}
	lines = appendInspectorField(lines, "Recurrence", recurrence, width)

	done := 0
	for _, item := range task.Checklist {
		if item.Done {
			done++
		}
	}
	lines = appendInspectorField(lines, "Checklist", fmt.Sprintf("%d/%d complete", done, len(task.Checklist)), width)

	lines = append(lines, "", BoldStyle.Render("Lifecycle"))
	lines = appendInspectorField(lines, "Created", inspectorTimestamp(task.CreatedAt), width)
	lines = appendInspectorField(lines, "Updated", inspectorTimestamp(task.UpdatedAt), width)
	completed := "none"
	if task.CompletedAt != nil {
		completed = inspectorTimestamp(*task.CompletedAt)
	}
	lines = appendInspectorField(lines, "Completed", completed, width)

	lines = append(lines, "", BoldStyle.Render("Description"))
	description := strings.TrimSpace(task.Description)
	if description == "" {
		lines = append(lines, MutedStyle.Render("No description"))
	} else {
		for raw := range strings.SplitSeq(strings.ReplaceAll(description, "\r\n", "\n"), "\n") {
			if raw == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, wrapText(raw, width)...)
		}
	}

	return fitInspectorLines(lines, width, height)
}

func appendInspectorField(lines []string, label, value string, width int) []string {
	if strings.TrimSpace(value) == "" {
		value = "none"
	}
	prefix := MutedStyle.Render(label + ": ")
	nextPrefix := strings.Repeat(" ", lipgloss.Width(prefix))
	return appendPeekWrappedItem(lines, prefix, nextPrefix, value, width)
}

func inspectorTaskPath(task *models.Task, project *models.Project) string {
	segments := make([]string, 0, 4)
	if project != nil {
		name := strings.TrimSpace(project.Name)
		if name == "" {
			name = strings.TrimSpace(project.Slug)
		}
		if name == "" {
			name = strings.TrimSpace(project.ID)
		}
		if name != "" {
			segments = append(segments, name)
		}
	}

	var parents []string
	seen := make(map[string]bool)
	parent := task.Parent
	if parent == nil && task.ParentID != nil && project != nil {
		parent = project.GetTaskByID(*task.ParentID)
	}
	for parent != nil && !seen[parent.ID] {
		seen[parent.ID] = true
		parents = append(parents, parent.Title)
		if parent.Parent != nil {
			parent = parent.Parent
		} else if parent.ParentID != nil && project != nil {
			parent = project.GetTaskByID(*parent.ParentID)
		} else {
			parent = nil
		}
	}
	for left, right := 0, len(parents)-1; left < right; left, right = left+1, right-1 {
		parents[left], parents[right] = parents[right], parents[left]
	}
	segments = append(segments, parents...)
	if len(segments) == 0 {
		return "unknown"
	}
	return strings.Join(segments, " › ")
}

func inspectorPriority(priority int) string {
	switch priority {
	case 0:
		return "none"
	case 1:
		return "P1 critical"
	case 2:
		return "P2 high"
	case 3:
		return "P3 normal"
	case 4:
		return "P4 low"
	case 5:
		return "P5 lowest"
	default:
		return fmt.Sprintf("P%d", priority)
	}
}

func inspectorDependencies(task *models.Task, project *models.Project) string {
	if len(task.DependsOn) == 0 {
		return "none"
	}
	labels := make([]string, 0, len(task.DependsOn))
	for _, stored := range task.DependsOn {
		ref := models.ParseTaskRef(stored)
		if !ref.IsCrossProject() && project != nil {
			if dependency := project.GetTaskByID(ref.TaskID); dependency != nil {
				labels = append(labels, fmt.Sprintf("#%d %s", dependency.ShortID, dependency.Title))
				continue
			}
		}
		labels = append(labels, stored)
	}
	return strings.Join(labels, ", ")
}

func inspectorTimestamp(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Format("2006-01-02 15:04")
}

func fitInspectorLines(lines []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	truncated := len(lines) > height
	if truncated {
		lines = append([]string(nil), lines[:height]...)
	}
	for i, line := range lines {
		lines[i] = truncateToVisualWidth(line, width)
	}
	if truncated {
		lines[len(lines)-1] = truncateToVisualWidth(MutedStyle.Render("…"), width)
	}
	return strings.Join(lines, "\n")
}

func appendPeekWrappedItem(lines []string, firstPrefix, nextPrefix, body string, width int) []string {
	if width <= 0 {
		return lines
	}

	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	parts := strings.Split(body, "\n")
	if len(parts) == 0 {
		parts = []string{""}
	}

	for partIndex, part := range parts {
		prefix := firstPrefix
		if partIndex > 0 {
			prefix = nextPrefix
		}
		wrapped := wrapStyled(part, peekBodyWrapWidth(width, prefix, nextPrefix))
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		for lineIndex, line := range wrapped {
			linePrefix := prefix
			if lineIndex > 0 {
				linePrefix = nextPrefix
			}
			lines = append(lines, truncatePeekLine(linePrefix+line, width))
		}
	}
	return lines
}

func peekBodyWrapWidth(width int, prefixes ...string) int {
	bodyWidth := width
	for _, prefix := range prefixes {
		bodyWidth = min(bodyWidth, width-lipgloss.Width(prefix))
	}
	return max(bodyWidth, 1)
}

func truncatePeekLine(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return truncateIntentional(line, width, "")
}

func peekAttachmentsBlock(task *models.Task, width int) []string {
	if len(task.Attachments) == 0 {
		return nil
	}
	lines := []string{BoldStyle.Render(fmt.Sprintf("Attachments (%d)", len(task.Attachments)))}
	for _, a := range task.Attachments {
		body := a.Filename
		if ref, ok := gitHubIssueAttachmentRef(a); ok {
			body = "GitHub issue " + formatGitHubIssueRef(ref)
		} else if a.IsCodeRef && a.LineNumber > 0 {
			body = fmt.Sprintf("%s:%d", a.Filename, a.LineNumber)
		} else if a.IsURL && a.OriginalPath != "" && a.OriginalPath != a.Filename {
			body = a.Filename + " " + MutedStyle.Render("("+a.OriginalPath+")")
		}
		lines = appendPeekWrappedItem(lines, "• ", "  ", body, width)
	}
	return lines
}
