package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// Pre-compiled regexps for quick edit parsing to avoid recompilation on each call.
var (
	qePriorityRe  = regexp.MustCompile(`\s*!([1-5])\s*`)
	qeRemoveTagRe = regexp.MustCompile(`\s*-#([\w][\w.-]*)\s*`)
	qeAddTagRe    = regexp.MustCompile(`\s*#([\w][\w.-]*)\s*`)
	qeDueDateRe   = regexp.MustCompile(`\s*@(\S+)\s*`)
	qeSectionRe   = regexp.MustCompile(`\s*/(\S+)\s*`)
	qeStatusRe    = regexp.MustCompile(`\s*\$(\w+)\s*`)
	qeMultiSpace  = regexp.MustCompile(`\s+`)
)

// quickEditResult holds the parsed result of inline task editing.
// Unlike quickCaptureResult (for new tasks), this tracks which fields
// were explicitly specified so we only update those fields.
type quickEditResult struct {
	title    string
	titleSet bool

	priority    int
	prioritySet bool

	tagsToAdd    []string
	tagsToRemove []string

	dueDate      *time.Time
	dueDateSet   bool
	dueDateClear bool

	section    string
	sectionSet bool

	status    models.TaskStatus
	statusSet bool
}

// parseQuickEdit parses input for inline task editing.
// Syntax supports: !1-5 (priority), #tag (add tag), -#tag (remove tag),
// @date (due date), @0 (clear due), /section, $status
// Everything else becomes the title.
func parseQuickEdit(input string, _ *models.Task, project *models.Project) (*quickEditResult, error) {
	result := &quickEditResult{}

	// Extract priority: !1 through !5
	if match := qePriorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		result.prioritySet = true
		input = qePriorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags to remove: -#word
	removeTagMatches := qeRemoveTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range removeTagMatches {
		if len(match) > 1 {
			result.tagsToRemove = append(result.tagsToRemove, match[1])
		}
	}
	input = qeRemoveTagRe.ReplaceAllString(input, " ")

	// Extract tags to add: #word
	addTagMatches := qeAddTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range addTagMatches {
		if len(match) > 1 {
			result.tagsToAdd = append(result.tagsToAdd, match[1])
		}
	}
	input = qeAddTagRe.ReplaceAllString(input, " ")

	// Extract due date: @value or @0 to clear
	if match := qeDueDateRe.FindStringSubmatch(input); len(match) > 1 {
		if match[1] == "0" {
			result.dueDateClear = true
			result.dueDateSet = true
		} else {
			date, err := ParseDate(match[1])
			if err != nil {
				return nil, fmt.Errorf("invalid date: %s", match[1])
			}
			result.dueDate = &date
			result.dueDateSet = true
		}
		input = qeDueDateRe.ReplaceAllString(input, " ")
	}

	// Extract section: /SectionName (case-insensitive matching)
	if match := qeSectionRe.FindStringSubmatch(input); len(match) > 1 {
		sectionName := match[1]
		// Handle special "0" or "active" to clear section
		if sectionName == "0" || strings.EqualFold(sectionName, "active") {
			result.section = ""
			result.sectionSet = true
		} else {
			found := false
			for _, s := range project.GetSections() {
				if strings.EqualFold(s, sectionName) {
					result.section = s
					result.sectionSet = true
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("section not found: %s", sectionName)
			}
		}
		input = qeSectionRe.ReplaceAllString(input, " ")
	}

	// Extract status: $start, $blocked, $done, $canceled, $todo
	if match := qeStatusRe.FindStringSubmatch(input); len(match) > 1 {
		statusStr := strings.ToLower(match[1])
		switch statusStr {
		case "start", "started", "progress":
			result.status = models.StatusInProgress
			result.statusSet = true
		case "blocked", "block":
			result.status = models.StatusBlocked
			result.statusSet = true
		case "done", "complete", "completed":
			result.status = models.StatusDone
			result.statusSet = true
		case "canceled", "cancel":
			result.status = models.StatusCanceled
			result.statusSet = true
		case "todo", "open", "reopen":
			result.status = models.StatusTodo
			result.statusSet = true
		case "review":
			result.status = models.StatusReview
			result.statusSet = true
		case "standby", "hold":
			result.status = models.StatusStandby
			result.statusSet = true
		default:
			return nil, fmt.Errorf("invalid status: $%s", match[1])
		}
		input = qeStatusRe.ReplaceAllString(input, " ")
	}

	// Clean up title (everything remaining)
	result.title = strings.TrimSpace(input)
	result.title = qeMultiSpace.ReplaceAllString(result.title, " ")
	result.titleSet = true

	if result.title == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	return result, nil
}

// parseQuickEditForPreview parses input without failing, for live preview.
func parseQuickEditForPreview(input string, _ *models.Task, project *models.Project) (*quickEditResult, string) {
	result := &quickEditResult{}
	var errMsg string

	// Extract priority
	if match := qePriorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		result.prioritySet = true
		input = qePriorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags to remove
	removeTagMatches := qeRemoveTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range removeTagMatches {
		if len(match) > 1 {
			result.tagsToRemove = append(result.tagsToRemove, match[1])
		}
	}
	input = qeRemoveTagRe.ReplaceAllString(input, " ")

	// Extract tags to add
	addTagMatches := qeAddTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range addTagMatches {
		if len(match) > 1 {
			result.tagsToAdd = append(result.tagsToAdd, match[1])
		}
	}
	input = qeAddTagRe.ReplaceAllString(input, " ")

	// Extract due date
	if match := qeDueDateRe.FindStringSubmatch(input); len(match) > 1 {
		if match[1] == "0" {
			result.dueDateClear = true
			result.dueDateSet = true
		} else {
			date, err := ParseDate(match[1])
			if err != nil {
				errMsg = fmt.Sprintf("invalid date: %s", match[1])
			} else {
				result.dueDate = &date
				result.dueDateSet = true
			}
		}
		input = qeDueDateRe.ReplaceAllString(input, " ")
	}

	// Extract section
	if match := qeSectionRe.FindStringSubmatch(input); len(match) > 1 {
		sectionName := match[1]
		if sectionName == "0" || strings.EqualFold(sectionName, "active") {
			result.section = ""
			result.sectionSet = true
		} else {
			found := false
			for _, s := range project.GetSections() {
				if strings.EqualFold(s, sectionName) {
					result.section = s
					result.sectionSet = true
					found = true
					break
				}
			}
			if !found && errMsg == "" {
				errMsg = fmt.Sprintf("section not found: %s", sectionName)
			}
		}
		input = qeSectionRe.ReplaceAllString(input, " ")
	}

	// Extract status
	if match := qeStatusRe.FindStringSubmatch(input); len(match) > 1 {
		statusStr := strings.ToLower(match[1])
		switch statusStr {
		case "start", "started", "progress":
			result.status = models.StatusInProgress
			result.statusSet = true
		case "blocked", "block":
			result.status = models.StatusBlocked
			result.statusSet = true
		case "done", "complete", "completed":
			result.status = models.StatusDone
			result.statusSet = true
		case "canceled", "cancel":
			result.status = models.StatusCanceled
			result.statusSet = true
		case "todo", "open", "reopen":
			result.status = models.StatusTodo
			result.statusSet = true
		case "review":
			result.status = models.StatusReview
			result.statusSet = true
		case "standby", "hold":
			result.status = models.StatusStandby
			result.statusSet = true
		default:
			if errMsg == "" {
				errMsg = fmt.Sprintf("invalid status: $%s", match[1])
			}
		}
		input = qeStatusRe.ReplaceAllString(input, " ")
	}

	// Clean up title
	result.title = strings.TrimSpace(input)
	result.title = qeMultiSpace.ReplaceAllString(result.title, " ")
	result.titleSet = true

	if result.title == "" && errMsg == "" {
		errMsg = "task title cannot be empty"
	}

	return result, errMsg
}

// applyQuickEdit applies the parsed changes to the task.
// Returns an error message if a status transition is invalid, empty string on success.
func applyQuickEdit(task *models.Task, project *models.Project, result *quickEditResult) string {
	var statusErr string
	project.MutateTask(task.ID, func(t *models.Task) {
		if result.titleSet && result.title != "" {
			if models.ValidateTaskTitle(result.title) == nil {
				t.Title = result.title
			}
		}

		if result.prioritySet {
			t.Priority = result.priority
		}

		// Handle tag changes
		for _, tag := range result.tagsToRemove {
			t.RemoveTag(tag)
		}
		for _, tag := range result.tagsToAdd {
			t.AddTag(tag)
		}

		if result.dueDateSet {
			if result.dueDateClear {
				t.DueDate = nil
			} else {
				t.DueDate = result.dueDate
			}
		}

		if result.sectionSet {
			if models.IsGTDSection(result.section) || result.section == "" {
				t.Section = result.section
			} else {
				t.CustomSection = result.section
			}
		}

		if result.statusSet {
			if msg := t.CanTransitionTo(result.status); msg != "" {
				statusErr = msg
			} else {
				t.ApplyTransition(result.status)
			}
		}
	})
	return statusErr
}

// renderQuickEditPreview renders a preview of what will change.
func renderQuickEditPreview(result *quickEditResult, original *quickEditResult) string {
	if result == nil {
		return ""
	}

	var parts []string

	// Show priority change
	if result.prioritySet && result.priority != original.priority {
		parts = append(parts, fmt.Sprintf("!%d", result.priority))
	}

	// Show tags to add
	for _, tag := range result.tagsToAdd {
		parts = append(parts, fmt.Sprintf("+#%s", tag))
	}

	// Show tags to remove
	for _, tag := range result.tagsToRemove {
		parts = append(parts, fmt.Sprintf("-#%s", tag))
	}

	// Show due date change
	if result.dueDateSet {
		if result.dueDateClear {
			parts = append(parts, "@clear")
		} else if result.dueDate != nil {
			parts = append(parts, fmt.Sprintf("@%s", result.dueDate.Format("Jan 2")))
		}
	}

	// Show section change
	if result.sectionSet && result.section != original.section {
		if result.section == "" {
			parts = append(parts, "/Active")
		} else {
			parts = append(parts, fmt.Sprintf("/%s", result.section))
		}
	}

	// Show status change
	if result.statusSet && result.status != original.status {
		parts = append(parts, fmt.Sprintf("$%s", result.status))
	}

	if len(parts) == 0 {
		// Check if title changed
		if result.titleSet && result.title != original.title {
			return "(title changed)"
		}
		return "(no changes)"
	}

	return "-> " + strings.Join(parts, " ")
}
