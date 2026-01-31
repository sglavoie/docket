package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
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
func parseQuickEdit(input string, task *models.Task, project *models.Project) (*quickEditResult, error) {
	result := &quickEditResult{}

	// Extract priority: !1 through !5
	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		result.prioritySet = true
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags to remove: -#word
	removeTagRe := regexp.MustCompile(`\s*-#(\w+)\s*`)
	removeTagMatches := removeTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range removeTagMatches {
		if len(match) > 1 {
			result.tagsToRemove = append(result.tagsToRemove, match[1])
		}
	}
	input = removeTagRe.ReplaceAllString(input, " ")

	// Extract tags to add: #word
	addTagRe := regexp.MustCompile(`\s*#(\w+)\s*`)
	addTagMatches := addTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range addTagMatches {
		if len(match) > 1 {
			result.tagsToAdd = append(result.tagsToAdd, match[1])
		}
	}
	input = addTagRe.ReplaceAllString(input, " ")

	// Extract due date: @value or @0 to clear
	dueDateRe := regexp.MustCompile(`\s*@(\S+)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
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
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Extract section: /SectionName (case-insensitive matching)
	sectionRe := regexp.MustCompile(`\s*/(\S+)\s*`)
	if match := sectionRe.FindStringSubmatch(input); len(match) > 1 {
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
		input = sectionRe.ReplaceAllString(input, " ")
	}

	// Extract status: $start, $blocked, $done, $canceled, $todo
	statusRe := regexp.MustCompile(`\s*\$(\w+)\s*`)
	if match := statusRe.FindStringSubmatch(input); len(match) > 1 {
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
		default:
			return nil, fmt.Errorf("invalid status: $%s", match[1])
		}
		input = statusRe.ReplaceAllString(input, " ")
	}

	// Clean up title (everything remaining)
	result.title = strings.TrimSpace(input)
	result.title = regexp.MustCompile(`\s+`).ReplaceAllString(result.title, " ")
	result.titleSet = true

	if result.title == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	return result, nil
}

// parseQuickEditForPreview parses input without failing, for live preview.
func parseQuickEditForPreview(input string, task *models.Task, project *models.Project) (*quickEditResult, string) {
	result := &quickEditResult{}
	var errMsg string

	// Extract priority
	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		result.prioritySet = true
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags to remove
	removeTagRe := regexp.MustCompile(`\s*-#(\w+)\s*`)
	removeTagMatches := removeTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range removeTagMatches {
		if len(match) > 1 {
			result.tagsToRemove = append(result.tagsToRemove, match[1])
		}
	}
	input = removeTagRe.ReplaceAllString(input, " ")

	// Extract tags to add
	addTagRe := regexp.MustCompile(`\s*#(\w+)\s*`)
	addTagMatches := addTagRe.FindAllStringSubmatch(input, -1)
	for _, match := range addTagMatches {
		if len(match) > 1 {
			result.tagsToAdd = append(result.tagsToAdd, match[1])
		}
	}
	input = addTagRe.ReplaceAllString(input, " ")

	// Extract due date
	dueDateRe := regexp.MustCompile(`\s*@(\S+)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
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
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Extract section
	sectionRe := regexp.MustCompile(`\s*/(\S+)\s*`)
	if match := sectionRe.FindStringSubmatch(input); len(match) > 1 {
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
		input = sectionRe.ReplaceAllString(input, " ")
	}

	// Extract status
	statusRe := regexp.MustCompile(`\s*\$(\w+)\s*`)
	if match := statusRe.FindStringSubmatch(input); len(match) > 1 {
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
		default:
			if errMsg == "" {
				errMsg = fmt.Sprintf("invalid status: $%s", match[1])
			}
		}
		input = statusRe.ReplaceAllString(input, " ")
	}

	// Clean up title
	result.title = strings.TrimSpace(input)
	result.title = regexp.MustCompile(`\s+`).ReplaceAllString(result.title, " ")
	result.titleSet = true

	if result.title == "" && errMsg == "" {
		errMsg = "task title cannot be empty"
	}

	return result, errMsg
}

// applyQuickEdit applies the parsed changes to the task.
func applyQuickEdit(task *models.Task, result *quickEditResult) {
	if result.titleSet && result.title != "" {
		task.Title = result.title
	}

	if result.prioritySet {
		task.Priority = result.priority
	}

	// Handle tag changes
	for _, tag := range result.tagsToRemove {
		task.RemoveTag(tag)
	}
	for _, tag := range result.tagsToAdd {
		task.AddTag(tag)
	}

	if result.dueDateSet {
		if result.dueDateClear {
			task.DueDate = nil
		} else {
			task.DueDate = result.dueDate
		}
	}

	if result.sectionSet {
		if models.IsGTDSection(result.section) || result.section == "" {
			task.Section = result.section
		} else {
			task.CustomSection = result.section
		}
	}

	if result.statusSet {
		task.Status = result.status
	}

	task.UpdatedAt = time.Now()
}

// hasChanges returns true if the edit result differs from the original.
func (r *quickEditResult) hasChanges(original *quickEditResult) bool {
	if r.titleSet && r.title != original.title {
		return true
	}
	if r.prioritySet && r.priority != original.priority {
		return true
	}
	if len(r.tagsToAdd) > 0 || len(r.tagsToRemove) > 0 {
		return true
	}
	if r.dueDateSet {
		if r.dueDateClear && original.dueDate != nil {
			return true
		}
		if r.dueDate != nil && (original.dueDate == nil || !r.dueDate.Equal(*original.dueDate)) {
			return true
		}
	}
	if r.sectionSet && r.section != original.section {
		return true
	}
	if r.statusSet && r.status != original.status {
		return true
	}
	return false
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
