package storage

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/tterrors"
)

const (
	scannerInitBufSize = 64 * 1024
	scannerMaxBufSize  = 1024 * 1024
)

var (
	// Matches headings like "## Epic: Title" or "### Story: Title"
	headingRegex = regexp.MustCompile(`^(#{2,6})\s+(Epic|Story|Task|Bug|Idea|Note|Action):\s*(.+)$`)

	// Matches project title like "# Project: Name"
	projectRegex = regexp.MustCompile(`^#\s+Project:\s*(.+)$`)

	// Matches sections metadata like "<!-- sections: Backlog;Active;Done -->"
	sectionsRegex = regexp.MustCompile(`^<!--\s*sections:\s*(.+?)\s*-->$`)
)

// ParseWarning represents a non-fatal issue found during parsing.
type ParseWarning = format.ParseWarning

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
	// Increase buffer to 1MB to handle files with long lines (e.g., base64
	// content in descriptions, very long metadata comments). The default 64KB
	// limit would cause silent truncation via scanner.Err().
	scanner.Buffer(make([]byte, 0, scannerInitBufSize), scannerMaxBufSize)
	var projectName string
	var sections []string
	sectionColors := make(map[string]string)
	var parsedTasks []*format.ParsedTask
	var currentTask *format.ParsedTask
	var descBuilder strings.Builder
	var nextShortID = 1
	var foundFirstTask bool
	var metadataWarnings []ParseWarning

	for scanner.Scan() {
		line := scanner.Text()

		// Check for project title
		if matches := projectRegex.FindStringSubmatch(line); matches != nil {
			projectName = strings.TrimSpace(matches[1])
			if len(projectName) > models.MaxProjectNameLength {
				metadataWarnings = append(metadataWarnings, ParseWarning{
					Type:    format.WarningValidation,
					Field:   "project_name",
					Message: fmt.Sprintf("project name exceeds %d chars", models.MaxProjectNameLength),
				})
			}
			continue
		}

		// Check for sections metadata (only before first task)
		// Format: "<!-- sections: Backlog;Active:green;Done:blue -->"
		if !foundFirstTask {
			if matches := sectionsRegex.FindStringSubmatch(line); matches != nil {
				sectionList := strings.SplitSeq(matches[1], ";")
				for s := range sectionList {
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
								if models.IsValidSectionColor(color) {
									sectionColors[name] = color
								} else {
									metadataWarnings = append(metadataWarnings, ParseWarning{
										Type:    format.WarningValidation,
										Field:   "sections",
										Message: fmt.Sprintf("section %q has invalid color %q (valid: gray, red, orange, yellow, green, cyan, blue, purple, magenta, white), using default", name, color),
									})
								}
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
				if typeStr == "action" {
					metadataWarnings = append(metadataWarnings, ParseWarning{
						Type:    format.WarningDeprecation,
						Field:   "type",
						Message: fmt.Sprintf("task #%d has deprecated type 'action', migrated to 'task'", nextShortID),
					})
				} else {
					metadataWarnings = append(metadataWarnings, ParseWarning{
						Type:    format.WarningValidation,
						Field:   "type",
						Message: fmt.Sprintf("task #%d has unrecognized type '%s', treated as 'task'", nextShortID, typeStr),
					})
				}
			}

			if len(title) > models.MaxTaskTitleLength {
				metadataWarnings = append(metadataWarnings, ParseWarning{
					Type:    format.WarningValidation,
					Field:   "title",
					Message: fmt.Sprintf("task #%d title is overlong (%d chars, max %d)", nextShortID, len(title), models.MaxTaskTitleLength),
				})
			}

			task, taskErr := models.NewTask("", nextShortID, taskType, title)
			if taskErr != nil {
				// Preserve tasks with invalid titles (e.g. empty) to avoid data loss
				metadataWarnings = append(metadataWarnings, ParseWarning{
					Type:    format.WarningDataCorrection,
					Field:   "title",
					Message: fmt.Sprintf("task #%d has invalid title: %v", nextShortID, taskErr),
				})
				task = &models.Task{
					ShortID:         nextShortID,
					Type:            taskType,
					Title:           title,
					Status:          models.StatusTodo,
					Priority:        models.PriorityDefault,
					Tags:            []string{},
					DependsOn:       []string{},
					RelatedTo:       []string{},
					ScoreDimensions: map[string]int{},
					Children:        []*models.Task{},
				}
			}
			// Zero out timestamps so they come from metadata parsing.
			// NewTask sets these to time.Now(), which is wrong for tasks
			// loaded from disk — their real timestamps are in the metadata.
			task.CreatedAt = time.Time{}
			task.UpdatedAt = time.Time{}
			currentTask = &format.ParsedTask{
				Task:  task,
				Level: level,
			}
			nextShortID++
			descBuilder.Reset()
			continue
		}

		// Check for metadata comment
		if currentTask != nil && currentTask.Task.ID == "" {
			if matches := format.MetadataRegex().FindStringSubmatch(line); matches != nil {
				warnings := format.ParseMetadata(currentTask.Task, matches[1])
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
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("%w: file exceeds the %dMB parse buffer — consider splitting this project into smaller files", tterrors.ErrParseError, scannerMaxBufSize/(1024*1024))
		}
		return nil, fmt.Errorf("%w: %w", tterrors.ErrParseError, err)
	}

	// Build parent-child relationships using levels captured during initial parse
	tasks, hierarchyWarnings := format.BuildHierarchy(parsedTasks)
	metadataWarnings = append(metadataWarnings, hierarchyWarnings...)

	idWarnings := assignMissingIDsAndTimestamps(tasks)
	metadataWarnings = append(metadataWarnings, idWarnings...)

	// Extract activity and comments from task descriptions
	for _, t := range tasks {
		format.ExtractActivityFromDescription(t)
		format.ExtractCommentsFromDescription(t)
	}

	normalizeUpdatedAt(tasks)

	taskByID, dupWarnings := detectAndReassignDuplicateIDs(tasks)
	metadataWarnings = append(metadataWarnings, dupWarnings...)

	parentWarnings := validateParentReferences(tasks, taskByID)
	metadataWarnings = append(metadataWarnings, parentWarnings...)

	// Default to Next section if none specified
	if len(sections) == 0 {
		sections = []string{models.SectionNext}
	}

	return &ParseResult{
		ProjectName:   projectName,
		Tasks:         tasks,
		Sections:      sections,
		SectionColors: sectionColors,
		Warnings:      metadataWarnings,
	}, nil
}

// assignMissingIDsAndTimestamps assigns UUIDs and timestamps to tasks that had
// no metadata comment. This happens when a user manually adds a heading without
// an HTML comment line. Without this, such tasks would have ID == "" which
// breaks lookups, tree building, etc.
func assignMissingIDsAndTimestamps(tasks []*models.Task) []ParseWarning {
	var warnings []ParseWarning
	now := time.Now()
	for _, t := range tasks {
		if t.ID == "" {
			t.ID = uuid.New().String()
			warnings = append(warnings, ParseWarning{
				Type:    format.WarningDataCorrection,
				TaskID:  t.ID,
				Field:   "id",
				Message: fmt.Sprintf("task #%d had no metadata comment, generated ID %s", t.ShortID, t.ID),
			})
		}
		if t.CreatedAt.IsZero() {
			t.CreatedAt = now
		}
	}
	return warnings
}

// normalizeUpdatedAt defaults UpdatedAt to CreatedAt when the metadata omitted
// the updated: field (because it equalled created:), so downstream code never
// sees a year-0001 timestamp.
func normalizeUpdatedAt(tasks []*models.Task) {
	for _, t := range tasks {
		if t.UpdatedAt.IsZero() && !t.CreatedAt.IsZero() {
			t.UpdatedAt = t.CreatedAt
		}
	}
}

// detectAndReassignDuplicateIDs detects duplicate task IDs, reassigns new UUIDs
// to duplicates, and updates all cross-references (ParentID, DependsOn,
// RelatedTo) that pointed to the old IDs. Returns the set of all known IDs and
// any warnings generated.
func detectAndReassignDuplicateIDs(tasks []*models.Task) (map[string]bool, []ParseWarning) {
	var warnings []ParseWarning
	taskByID := make(map[string]bool, len(tasks))
	idRemap := make(map[string]string) // oldID → newID
	for _, t := range tasks {
		if taskByID[t.ID] {
			oldID := t.ID
			t.ID = uuid.New().String()
			idRemap[oldID] = t.ID
			warnings = append(warnings, ParseWarning{
				Type:    format.WarningDataCorrection,
				TaskID:  t.ID,
				Field:   "id",
				Message: fmt.Sprintf("task #%d has duplicate ID %q, reassigned to %s", t.ShortID, oldID, t.ID),
			})
		}
		taskByID[t.ID] = true
	}
	// Update references that point to reassigned IDs.
	if len(idRemap) > 0 {
		for _, t := range tasks {
			if t.ParentID != nil {
				if newID, ok := idRemap[*t.ParentID]; ok {
					*t.ParentID = newID
				}
			}
			for i, dep := range t.DependsOn {
				if newID, ok := idRemap[dep]; ok {
					t.DependsOn[i] = newID
				}
			}
			for i, rel := range t.RelatedTo {
				if newID, ok := idRemap[rel]; ok {
					t.RelatedTo[i] = newID
				}
			}
		}
	}
	return taskByID, warnings
}

// validateParentReferences clears ParentID on any task whose parent no longer
// exists in the parsed task set (e.g. was deleted externally).
func validateParentReferences(tasks []*models.Task, taskByID map[string]bool) []ParseWarning {
	var warnings []ParseWarning
	for _, t := range tasks {
		if t.ParentID != nil && !taskByID[*t.ParentID] {
			orphanedID := *t.ParentID
			warnings = append(warnings, ParseWarning{
				Type:    format.WarningDataCorrection,
				TaskID:  t.ID,
				Field:   "parent",
				Message: fmt.Sprintf("task %s references non-existent parent %s, clearing parent reference", t.ID, orphanedID),
			})
			t.ParentID = nil
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

	return "", fmt.Errorf("%w: project name not found in file", tterrors.ErrParseError)
}
