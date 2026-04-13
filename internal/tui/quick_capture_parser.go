package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// parseQuickCapture extracts metadata from the input string.
// Syntax: "^bug Task title !1 #backend #urgent @tomorrow >3 %projectname"
// When skipProject is true, %project syntax is ignored (treated as literal text).
func parseQuickCapture(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*quickCaptureResult, error) {
	result := &quickCaptureResult{
		taskType: models.TaskTypeTask, // default type
		priority: 3,                   // default priority
	}

	// Extract task type: ^epic, ^story, ^task, ^bug, ^idea, ^note (case-insensitive)
	// Require ^ to be at start of string or preceded by whitespace
	typeRe := regexp.MustCompile(`(?i)(?:^|\s)\^(epic|story|task|bug|idea|note)`)
	if match := typeRe.FindStringSubmatch(input); len(match) > 1 {
		taskType, ok := models.ParseTaskTypeInsensitive(match[1])
		if !ok {
			return nil, fmt.Errorf("invalid task type: %s", match[1])
		}
		result.taskType = taskType
		input = typeRe.ReplaceAllString(input, " ")
	}

	// Extract priority: !1 through !5
	// Require ! to be at start of string or preceded by whitespace
	priorityRe := regexp.MustCompile(`(?:^|\s)!([1-5])`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags: #word
	// Require # to be at start of string or preceded by whitespace
	tagRe := regexp.MustCompile(`(?:^|\s)#([\w][\w.-]*)`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			result.tags = append(result.tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date: @value
	// Require @ to be at start of string or preceded by whitespace
	// Supports multi-word patterns: "next <weekday>", "in N days/weeks/months"
	dueDateRe := regexp.MustCompile(`(?i)(?:^|\s)@(next\s+\w+|in\s+\d+\s+\w+|\S+)`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
		date, err := ParseDate(match[1])
		if err != nil {
			return nil, fmt.Errorf("invalid date: %s", match[1])
		}
		result.dueDate = &date
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Extract recurrence: ~pattern (e.g., ~daily, ~weekly, ~2w)
	// Require ~ to be at start of string or preceded by whitespace
	recurRe := regexp.MustCompile(`(?:^|\s)~(\S+)`)
	if match := recurRe.FindStringSubmatch(input); len(match) > 1 {
		pattern := models.ParseRecurrence(match[1])
		if pattern == nil {
			return nil, fmt.Errorf("invalid recurrence: %s", match[1])
		}
		result.recurrence = pattern
		input = recurRe.ReplaceAllString(input, " ")
	}

	// Extract target project: %projectslug (for cross-project capture)
	// Require % to be at start of string or preceded by whitespace
	// Must be parsed before parent since parent is resolved against target project
	if !skipProject {
		projectRe := regexp.MustCompile(`(?:^|\s)%(\S+)`)
		if match := projectRe.FindStringSubmatch(input); len(match) > 1 {
			ref := match[1]
			targetProject := resolveProject(ref, allProjects)
			if targetProject == nil {
				return nil, fmt.Errorf("project not found: %s", ref)
			}
			result.targetProjectID = &targetProject.ID
			result.targetProjectName = targetProject.Name
			// Use target project for parent and section resolution
			project = targetProject
			input = projectRe.ReplaceAllString(input, " ")
		}
	}

	// Extract parent: >ref (short ID or title prefix)
	// Require > to be at start of string or preceded by whitespace
	parentRe := regexp.MustCompile(`(?:^|\s)>(\S+)`)
	if match := parentRe.FindStringSubmatch(input); len(match) > 1 {
		ref := match[1]
		parent := resolveParent(ref, project)
		if parent == nil {
			return nil, fmt.Errorf("parent not found: %s", ref)
		}
		if !result.taskType.CanBeChildOf(parent.Type) {
			return nil, fmt.Errorf("%s cannot be child of %s", result.taskType.HeadingPrefix(), parent.Type.HeadingPrefix())
		}
		result.parentID = &parent.ID
		input = parentRe.ReplaceAllString(input, " ")
	}

	// Extract section: /SectionName (case-insensitive matching)
	// Require / to be at start of string or preceded by whitespace
	sectionRe := regexp.MustCompile(`(?:^|\s)/(\S+)`)
	if match := sectionRe.FindStringSubmatch(input); len(match) > 1 {
		sectionName := match[1]
		// Find matching section (case-insensitive)
		found := false
		for _, s := range project.GetSections() {
			if strings.EqualFold(s, sectionName) {
				result.section = s
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("section not found: %s", sectionName)
		}
		input = sectionRe.ReplaceAllString(input, " ")
	}

	// Extract status: $start, $blocked, $done, $canceled
	// Require $ to be at start of string or preceded by whitespace
	statusRe := regexp.MustCompile(`(?:^|\s)\$(\w+)`)
	if match := statusRe.FindStringSubmatch(input); len(match) > 1 {
		statusStr := strings.ToLower(match[1])
		switch statusStr {
		case "start":
			result.status = models.StatusInProgress
			result.activate = true
		case "blocked":
			result.status = models.StatusBlocked
		case "done":
			result.status = models.StatusDone
		case "canceled":
			result.status = models.StatusCanceled
		case "review":
			result.status = models.StatusReview
		case "standby", "hold":
			result.status = models.StatusStandby
		default:
			return nil, fmt.Errorf("invalid status: $%s (use $start, $blocked, $done, $canceled, $review, $standby)", match[1])
		}
		input = statusRe.ReplaceAllString(input, " ")
	}

	// Clean up title
	result.title = strings.TrimSpace(input)
	result.title = regexp.MustCompile(`\s+`).ReplaceAllString(result.title, " ")

	if result.title == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	return result, nil
}

// resolveParent finds a task by short ID or title prefix.
func resolveParent(ref string, project *models.Project) *models.Task {
	// Try numeric short ID first
	if shortID, err := strconv.Atoi(ref); err == nil {
		for _, task := range project.Tasks {
			if task.ShortID == shortID {
				return task
			}
		}
	}

	// Fall back to case-insensitive title prefix match
	refLower := strings.ToLower(ref)
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), refLower) {
			return task
		}
	}

	return nil
}

// resolveProject finds a project by slug (exact match) or fuzzy name match.
func resolveProject(ref string, allProjects []*models.Project) *models.Project {
	// Try exact slug match first (case-insensitive)
	refLower := strings.ToLower(ref)
	for _, p := range allProjects {
		if !p.Archived && strings.ToLower(p.Slug) == refLower {
			return p
		}
	}

	// Fall back to fuzzy name match
	type scoredProject struct {
		project *models.Project
		score   int
	}
	var scored []scoredProject
	for _, p := range allProjects {
		if p.Archived {
			continue
		}
		// Match against name and slug
		searchText := p.Name + " " + p.Slug
		score := fuzzy.Score(searchText, ref)
		if score > 0 {
			scored = append(scored, scoredProject{project: p, score: score})
		}
	}

	if len(scored) == 0 {
		return nil
	}

	// Return highest scoring match
	best := scored[0]
	for _, s := range scored[1:] {
		if s.score > best.score {
			best = s
		}
	}
	return best.project
}

// parseQuickCaptureForPreview parses input for live preview without failing early.
// Returns partial results and any validation error string.
func parseQuickCaptureForPreview(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*quickCaptureResult, string) { //nolint:gocyclo
	result := &quickCaptureResult{
		taskType: models.TaskTypeTask,
		priority: 3,
	}
	var errMsg string

	// Extract task type
	// Require ^ to be at start of string or preceded by whitespace
	typeRe := regexp.MustCompile(`(?i)(?:^|\s)\^(epic|story|task|bug|idea|note)`)
	if match := typeRe.FindStringSubmatch(input); len(match) > 1 {
		if taskType, ok := models.ParseTaskTypeInsensitive(match[1]); ok {
			result.taskType = taskType
		}
		input = typeRe.ReplaceAllString(input, " ")
	}

	// Extract priority
	// Require ! to be at start of string or preceded by whitespace
	priorityRe := regexp.MustCompile(`(?:^|\s)!([1-5])`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags
	// Require # to be at start of string or preceded by whitespace
	tagRe := regexp.MustCompile(`(?:^|\s)#([\w][\w.-]*)`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			result.tags = append(result.tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date (capture error but don't stop)
	// Require @ to be at start of string or preceded by whitespace
	// Supports multi-word patterns: "next <weekday>", "in N days/weeks/months"
	dueDateRe := regexp.MustCompile(`(?i)(?:^|\s)@(next\s+\w+|in\s+\d+\s+\w+|\S+)`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
		date, err := ParseDate(match[1])
		if err != nil {
			errMsg = fmt.Sprintf("invalid date: %s", match[1])
		} else {
			result.dueDate = &date
		}
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Extract recurrence
	// Require ~ to be at start of string or preceded by whitespace
	recurRe := regexp.MustCompile(`(?:^|\s)~(\S+)`)
	if match := recurRe.FindStringSubmatch(input); len(match) > 1 {
		pattern := models.ParseRecurrence(match[1])
		if pattern == nil && errMsg == "" {
			errMsg = fmt.Sprintf("invalid recurrence: %s", match[1])
		} else if pattern != nil {
			result.recurrence = pattern
		}
		input = recurRe.ReplaceAllString(input, " ")
	}

	// Extract target project (for cross-project capture)
	// Require % to be at start of string or preceded by whitespace
	// Must be parsed before parent since parent is resolved against target project
	if !skipProject {
		projectRe := regexp.MustCompile(`(?:^|\s)%(\S+)`)
		if match := projectRe.FindStringSubmatch(input); len(match) > 1 {
			ref := match[1]
			targetProject := resolveProject(ref, allProjects)
			if targetProject == nil && errMsg == "" {
				errMsg = fmt.Sprintf("project not found: %s", ref)
			} else if targetProject != nil {
				result.targetProjectID = &targetProject.ID
				result.targetProjectName = targetProject.Name
				// Use target project for parent and section resolution
				project = targetProject
			}
			input = projectRe.ReplaceAllString(input, " ")
		}
	}

	// Extract parent
	// Require > to be at start of string or preceded by whitespace
	parentRe := regexp.MustCompile(`(?:^|\s)>(\S+)`)
	if match := parentRe.FindStringSubmatch(input); len(match) > 1 {
		ref := match[1]
		parent := resolveParent(ref, project)
		if parent == nil && errMsg == "" {
			errMsg = fmt.Sprintf("parent not found: %s", ref)
		} else if parent != nil {
			if !result.taskType.CanBeChildOf(parent.Type) && errMsg == "" {
				errMsg = fmt.Sprintf("%s cannot be child of %s", result.taskType.HeadingPrefix(), parent.Type.HeadingPrefix())
			} else {
				result.parentID = &parent.ID
			}
		}
		input = parentRe.ReplaceAllString(input, " ")
	}

	// Extract section
	// Require / to be at start of string or preceded by whitespace
	sectionRe := regexp.MustCompile(`(?:^|\s)/(\S+)`)
	if match := sectionRe.FindStringSubmatch(input); len(match) > 1 {
		sectionName := match[1]
		found := false
		for _, s := range project.GetSections() {
			if strings.EqualFold(s, sectionName) {
				result.section = s
				found = true
				break
			}
		}
		if !found && errMsg == "" {
			errMsg = fmt.Sprintf("section not found: %s", sectionName)
		}
		input = sectionRe.ReplaceAllString(input, " ")
	}

	// Extract status
	// Require $ to be at start of string or preceded by whitespace
	statusRe := regexp.MustCompile(`(?:^|\s)\$(\w+)`)
	if match := statusRe.FindStringSubmatch(input); len(match) > 1 {
		statusStr := strings.ToLower(match[1])
		switch statusStr {
		case "start":
			result.status = models.StatusInProgress
			result.activate = true
		case "blocked":
			result.status = models.StatusBlocked
		case "done":
			result.status = models.StatusDone
		case "canceled":
			result.status = models.StatusCanceled
		case "review":
			result.status = models.StatusReview
		case "standby", "hold":
			result.status = models.StatusStandby
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

	return result, errMsg
}
