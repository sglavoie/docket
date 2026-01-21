package storage

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

var (
	// Matches headings like "## Epic: Title" or "### Story: Title"
	headingRegex = regexp.MustCompile(`^(#{2,6})\s+(Epic|Story|Task|Bug|Idea|Note):\s*(.+)$`)

	// Matches metadata comments like "<!-- id: uuid, status: todo, priority: 1 -->"
	metadataRegex = regexp.MustCompile(`^<!--\s*(.+?)\s*-->$`)

	// Matches project title like "# Project: Name"
	projectRegex = regexp.MustCompile(`^#\s+Project:\s*(.+)$`)

	// Matches sections metadata like "<!-- sections: Backlog;Active;Done -->"
	sectionsRegex = regexp.MustCompile(`^<!--\s*sections:\s*(.+?)\s*-->$`)

	// Matches comment start marker like "<!-- COMMENTS -->"
	commentsStartRegex = regexp.MustCompile(`^<!--\s*COMMENTS\s*-->$`)

	// Matches comment end marker like "<!-- /COMMENTS -->"
	commentsEndRegex = regexp.MustCompile(`^<!--\s*/COMMENTS\s*-->$`)

	// Matches individual comment header like "<!-- comment: 2025-12-27 10:30 -->"
	commentHeaderRegex = regexp.MustCompile(`^<!--\s*comment:\s*(.+?)\s*-->$`)
)

// ParsedTask represents a task parsed from markdown with its raw data.
type ParsedTask struct {
	Task       *models.Task
	Level      int    // Heading level (2 for ##, 3 for ###, etc.)
	RawContent string // The description content between this task and the next
}

// ParseWarning represents a non-fatal issue found during parsing.
type ParseWarning struct {
	TaskID  string // ID of the task with the issue (may be empty for global warnings)
	Field   string // Which field had the issue (e.g., "priority", "parent")
	Message string // Human-readable description of the issue
}

// ParseResult contains the results of parsing a tasks.md file.
type ParseResult struct {
	ProjectName   string
	Tasks         []*models.Task
	Sections      []string
	SectionColors map[string]string
	Warnings      []ParseWarning // Non-fatal issues found during parsing
}

// ParseTasksMarkdown parses a tasks.md file content into tasks.
// Returns the project name and a slice of tasks.
func ParseTasksMarkdown(content string) (string, []*models.Task, error) {
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		return "", nil, err
	}
	return result.ProjectName, result.Tasks, nil
}

// ParseTasksMarkdownWithSections parses a tasks.md file content into tasks and sections.
// Returns a ParseResult containing the project name, tasks, and sections.
func ParseTasksMarkdownWithSections(content string) (*ParseResult, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var projectName string
	var sections []string
	sectionColors := make(map[string]string)
	var parsedTasks []*ParsedTask
	var currentTask *ParsedTask
	var descBuilder strings.Builder
	var nextShortID int = 1
	var foundFirstTask bool
	var metadataWarnings []ParseWarning

	for scanner.Scan() {
		line := scanner.Text()

		// Check for project title
		if matches := projectRegex.FindStringSubmatch(line); matches != nil {
			projectName = strings.TrimSpace(matches[1])
			continue
		}

		// Check for sections metadata (only before first task)
		// Format: "<!-- sections: Backlog;Active:green;Done:blue -->"
		if !foundFirstTask {
			if matches := sectionsRegex.FindStringSubmatch(line); matches != nil {
				sectionList := strings.Split(matches[1], ";")
				for _, s := range sectionList {
					s = strings.TrimSpace(s)
					if s == "" {
						continue
					}
					// Parse "SectionName:color" or just "SectionName"
					parts := strings.SplitN(s, ":", 2)
					name := strings.TrimSpace(parts[0])
					if name != "" {
						sections = append(sections, name)
						if len(parts) == 2 {
							color := strings.TrimSpace(parts[1])
							if color != "" {
								sectionColors[name] = color
							}
						}
					}
				}
				continue
			}
		}

		// Check for task heading
		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			foundFirstTask = true
			// Save previous task's description
			if currentTask != nil {
				currentTask.Task.Description = strings.TrimSpace(descBuilder.String())
				parsedTasks = append(parsedTasks, currentTask)
			}

			level := len(matches[1]) // Number of # characters
			typeStr := strings.ToLower(matches[2])
			title := strings.TrimSpace(matches[3])

			taskType, ok := models.ParseTaskType(typeStr)
			if !ok {
				taskType = models.TaskTypeTask
			}

			currentTask = &ParsedTask{
				Task:  models.NewTask("", nextShortID, taskType, title),
				Level: level,
			}
			nextShortID++
			descBuilder.Reset()
			continue
		}

		// Check for metadata comment
		if currentTask != nil && currentTask.Task.ID == "" {
			if matches := metadataRegex.FindStringSubmatch(line); matches != nil {
				warnings := parseMetadata(currentTask.Task, matches[1])
				// Add task ID to warnings now that we have it
				for i := range warnings {
					warnings[i].TaskID = currentTask.Task.ID
				}
				metadataWarnings = append(metadataWarnings, warnings...)
				continue
			}
		}

		// Accumulate description content
		if currentTask != nil {
			descBuilder.WriteString(line)
			descBuilder.WriteString("\n")
		}
	}

	// Don't forget the last task
	if currentTask != nil {
		currentTask.Task.Description = strings.TrimSpace(descBuilder.String())
		parsedTasks = append(parsedTasks, currentTask)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading markdown: %w", err)
	}

	// Build parent-child relationships using levels captured during initial parse
	tasks := buildHierarchy(parsedTasks)

	// Extract comments from task descriptions
	for _, t := range tasks {
		extractCommentsFromDescription(t)
	}

	// Validate parent references - clear any that reference non-existent tasks
	taskByID := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		taskByID[t.ID] = true
	}
	for _, t := range tasks {
		if t.ParentID != nil && !taskByID[*t.ParentID] {
			metadataWarnings = append(metadataWarnings, ParseWarning{
				TaskID:  t.ID,
				Field:   "parent",
				Message: fmt.Sprintf("task %s references non-existent parent %s, clearing parent reference", t.ID, *t.ParentID),
			})
			t.ParentID = nil
		}
	}

	// Default to Backlog section if none specified
	if len(sections) == 0 {
		sections = []string{models.SectionBacklog}
	}

	return &ParseResult{
		ProjectName:   projectName,
		Tasks:         tasks,
		Sections:      sections,
		SectionColors: sectionColors,
		Warnings:      metadataWarnings,
	}, nil
}

// parseTimeField attempts to parse a time value in datetime or date-only format.
// Returns the parsed time and true if successful, zero time and false otherwise.
func parseTimeField(value string) (time.Time, bool) {
	if t, err := time.ParseInLocation(util.DateTimeFormat, value, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// metadataKeys lists all known metadata keys for smart splitting.
var metadataKeys = []string{
	"id", "status", "priority", "parent", "tags", "created", "updated",
	"completed", "due", "depends", "section", "pinned", "active", "recur",
}

// splitMetadataPairs splits metadata string into key-value pairs, handling values
// that may contain commas (e.g., "recur: 1w:mon,wed,fri").
// It looks for ", key:" patterns where key is a known metadata key.
func splitMetadataPairs(metaStr string) []string {
	var pairs []string
	remaining := metaStr

	for len(remaining) > 0 {
		// Find the earliest occurrence of ", knownKey:" in the remaining string
		splitIdx := -1
		for _, key := range metadataKeys {
			pattern := ", " + key + ":"
			idx := strings.Index(remaining, pattern)
			if idx != -1 && (splitIdx == -1 || idx < splitIdx) {
				splitIdx = idx
			}
		}

		if splitIdx == -1 {
			// No more splits found, the rest is one pair
			pairs = append(pairs, strings.TrimSpace(remaining))
			break
		}

		// Add the pair before the split point
		pairs = append(pairs, strings.TrimSpace(remaining[:splitIdx]))
		// Continue with the rest (skip the ", ")
		remaining = remaining[splitIdx+2:]
	}

	return pairs
}

// parseMetadata parses the metadata string and sets task fields.
// Returns any warnings encountered during parsing.
func parseMetadata(task *models.Task, metaStr string) []ParseWarning {
	var warnings []ParseWarning

	// Parse key-value pairs, handling values that may contain commas (like recur: 1w:mon,wed,fri)
	// Format: "key1: value1, key2: value2, ..."
	// Strategy: find known keys and extract their values until the next known key
	pairs := splitMetadataPairs(metaStr)
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "id":
			task.ID = value
		case "status":
			if status, ok := models.ParseTaskStatus(value); ok {
				task.Status = status
			}
		case "priority":
			p, err := strconv.Atoi(value)
			if err != nil {
				warnings = append(warnings, ParseWarning{
					Field:   "priority",
					Message: fmt.Sprintf("invalid priority '%s', using default", value),
				})
			} else if p < 1 || p > 5 {
				warnings = append(warnings, ParseWarning{
					Field:   "priority",
					Message: fmt.Sprintf("priority %d out of range (1-5), using default", p),
				})
			} else {
				task.Priority = p
			}
		case "parent":
			task.ParentID = &value
		case "tags":
			tags := strings.Split(value, ";")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					task.Tags = append(task.Tags, tag)
				}
			}
		case "created":
			if t, ok := parseTimeField(value); ok {
				task.CreatedAt = t
			}
		case "updated":
			if t, ok := parseTimeField(value); ok {
				task.UpdatedAt = t
			}
		case "completed":
			if t, ok := parseTimeField(value); ok {
				task.CompletedAt = &t
			}
		case "due":
			if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
				task.DueDate = &t
			}
		case "depends":
			deps := strings.Split(value, ";")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					task.DependsOn = append(task.DependsOn, dep)
				}
			}
		case "section":
			task.Section = value
		case "pinned":
			task.Pinned = value == "true"
		case "active":
			task.Active = value == "true"
		case "recur":
			if pattern := models.ParseRecurrence(value); pattern != nil {
				task.Recurrence = pattern
			}
		}
	}
	return warnings
}

// ParseProjectNameFromFile reads a tasks.md file and extracts the project name.
func ParseProjectNameFromFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := projectRegex.FindStringSubmatch(line); matches != nil {
			return strings.TrimSpace(matches[1]), nil
		}
	}

	return "", fmt.Errorf("project name not found in file")
}

// extractCommentsFromDescription parses the COMMENTS block from task description.
// It extracts comments and returns the cleaned description (without the comments block).
func extractCommentsFromDescription(task *models.Task) {
	if task.Description == "" {
		return
	}

	lines := strings.Split(task.Description, "\n")
	var cleanDesc strings.Builder
	var inCommentsBlock bool
	var currentCommentText strings.Builder
	var currentCommentTime time.Time
	var hasCurrentComment bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for comments block start
		if commentsStartRegex.MatchString(trimmed) {
			inCommentsBlock = true
			continue
		}

		// Check for comments block end
		if commentsEndRegex.MatchString(trimmed) {
			// Save any pending comment
			if hasCurrentComment {
				text := strings.TrimSpace(currentCommentText.String())
				if text != "" {
					task.Comments = append(task.Comments, &models.Comment{
						Text:      unescapeCommentText(text),
						CreatedAt: currentCommentTime,
					})
				}
			}
			inCommentsBlock = false
			hasCurrentComment = false
			currentCommentText.Reset()
			continue
		}

		if inCommentsBlock {
			// Check for new comment header
			if matches := commentHeaderRegex.FindStringSubmatch(trimmed); matches != nil {
				// Save previous comment if exists
				if hasCurrentComment {
					text := strings.TrimSpace(currentCommentText.String())
					if text != "" {
						task.Comments = append(task.Comments, &models.Comment{
							Text:      unescapeCommentText(text),
							CreatedAt: currentCommentTime,
						})
					}
				}
				// Start new comment
				if t, ok := parseTimeField(matches[1]); ok {
					currentCommentTime = t
				} else {
					currentCommentTime = time.Time{}
				}
				hasCurrentComment = true
				currentCommentText.Reset()
				continue
			}

			// Accumulate comment text
			if hasCurrentComment {
				if currentCommentText.Len() > 0 {
					currentCommentText.WriteString("\n")
				}
				currentCommentText.WriteString(line)
			}
		} else {
			// Normal description content
			cleanDesc.WriteString(line)
			cleanDesc.WriteString("\n")
		}
	}

	// Update description with cleaned version (without comments block)
	task.Description = strings.TrimSpace(cleanDesc.String())
}

// buildHierarchy establishes parent-child relationships based on heading levels.
// Tasks at level N+1 that follow a task at level N become children of that task.
// Uses the levels already captured in ParsedTask during initial parsing.
func buildHierarchy(parsedTasks []*ParsedTask) []*models.Task {
	tasks := make([]*models.Task, len(parsedTasks))
	for i, pt := range parsedTasks {
		tasks[i] = pt.Task
	}

	// Build hierarchy using a stack
	// Stack tracks the most recent task at each level
	levelStack := make(map[int]*models.Task)

	for i, pt := range parsedTasks {
		task := tasks[i]
		level := pt.Level

		// If task already has an explicit parent ID from metadata, skip
		if task.ParentID != nil {
			levelStack[level] = task
			continue
		}

		// Find potential parent at a higher level (lower number)
		for checkLevel := level - 1; checkLevel >= 2; checkLevel-- {
			if parent, ok := levelStack[checkLevel]; ok {
				// Check if this task type can be a child of the parent type
				if task.Type.CanBeChildOf(parent.Type) {
					task.ParentID = &parent.ID
					break
				}
			}
		}

		// Update the stack
		levelStack[level] = task

		// Clear any lower levels (higher numbers) since we've moved past them
		for l := level + 1; l <= 6; l++ {
			delete(levelStack, l)
		}
	}

	return tasks
}
