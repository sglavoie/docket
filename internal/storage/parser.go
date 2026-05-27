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
	// Strip a leading UTF-8 BOM (0xEF 0xBB 0xBF) if present. Editors on
	// Windows occasionally save tasks.md with a BOM, which would otherwise
	// be glued onto the first line and prevent the project-title regex from
	// matching — silently losing the project name on load.
	content = strings.TrimPrefix(content, "\uFEFF")
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
	var metadataParsed bool
	var metadataWarnings []ParseWarning
	// insideNonDescriptionBlock tracks whether we're currently between
	// COMMENTS/ACTIVITY/DECISIONS open and close markers. Lines inside such
	// blocks must NOT receive the per-line heading-unescape pass because the
	// extractors that consume those blocks run their own unescape via
	// unescapeBlockText — applying both yields a double-unescape that
	// silently strips a backslash from user content on every round-trip.
	var insideNonDescriptionBlock bool

	for scanner.Scan() {
		line := scanner.Text()

		if parseProjectTitleLine(line, foundFirstTask, &projectName, &metadataWarnings) {
			continue
		}

		if parseSectionsMetadataLine(line, foundFirstTask, &sections, sectionColors, &metadataWarnings) {
			continue
		}

		if parseTaskHeadingLine(line, &currentTask, &parsedTasks, &descBuilder, &nextShortID, &foundFirstTask, &metadataParsed, &metadataWarnings) {
			insideNonDescriptionBlock = false
			continue
		}

		// Check for metadata comment on the first non-empty line after a heading.
		// Uses a dedicated flag rather than checking task.ID == "" because a
		// metadata comment might omit the id field, leaving ID empty. Once the
		// description starts, stop attempting metadata parsing so later HTML
		// comments in the description are preserved verbatim.
		if parseTaskMetadataLine(line, currentTask, &metadataParsed, &metadataWarnings) {
			continue
		}

		// Accumulate description content (unescape heading-like lines that
		// were escaped by the writer to prevent parser mis-interpretation).
		// Skip per-line unescape inside non-DESCRIPTION block markers; the
		// extractors for those blocks run their own unescape pass.
		if currentTask != nil {
			isNonDescBoundary := matchesNonDescriptionBlockMarker(line)
			if insideNonDescriptionBlock || isNonDescBoundary {
				descBuilder.WriteString(line)
			} else {
				descBuilder.WriteString(unescapeDescriptionLine(line))
			}
			descBuilder.WriteString("\n")
			if isNonDescBoundary {
				insideNonDescriptionBlock = !insideNonDescriptionBlock
			}
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

	// Extract activity and comments from task descriptions, then description block.
	// ACTIVITY/COMMENTS are stripped first (sibling blocks outside DESCRIPTION markers).
	// DESCRIPTION extraction runs on what remains.
	for _, t := range tasks {
		format.ExtractActivityFromDescription(t)
		format.ExtractCommentsFromDescription(t)
		format.ExtractDecisionsFromDescription(t)
		descWarnings := format.ExtractDescriptionBlock(t)
		metadataWarnings = append(metadataWarnings, descWarnings...)
		// Now that all block extractors have run, restore literal block-marker
		// lines that the writer escaped to prevent them from being mis-parsed.
		t.Description = unescapeBlockMarkers(t.Description)
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

func parseProjectTitleLine(line string, foundFirstTask bool, projectName *string, warnings *[]ParseWarning) bool {
	if foundFirstTask {
		return false
	}

	matches := projectRegex.FindStringSubmatch(line)
	if matches == nil {
		return false
	}

	*projectName = strings.TrimSpace(matches[1])
	if len(*projectName) > models.MaxProjectNameLength {
		*warnings = append(*warnings, ParseWarning{
			Type:    format.WarningValidation,
			Field:   "project_name",
			Message: fmt.Sprintf("project name exceeds %d chars", models.MaxProjectNameLength),
		})
	}

	return true
}

func parseSectionsMetadataLine(line string, foundFirstTask bool, sections *[]string, sectionColors map[string]string, warnings *[]ParseWarning) bool {
	if foundFirstTask {
		return false
	}

	matches := sectionsRegex.FindStringSubmatch(line)
	if matches == nil {
		return false
	}

	seenSections := make(map[string]bool, len(*sections))
	for _, existing := range *sections {
		seenSections[existing] = true
	}

	for sectionSpec := range strings.SplitSeq(matches[1], ";") {
		sectionSpec = strings.TrimSpace(sectionSpec)
		if sectionSpec == "" {
			continue
		}

		parts := strings.SplitN(sectionSpec, ":", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}

		if !seenSections[name] {
			*sections = append(*sections, name)
			seenSections[name] = true
		}
		if len(parts) == 2 {
			color := strings.TrimSpace(parts[1])
			if color == "" {
				continue
			}
			if models.IsValidSectionColor(color) {
				sectionColors[name] = color
				continue
			}
			*warnings = append(*warnings, ParseWarning{
				Type:    format.WarningValidation,
				Field:   "sections",
				Message: fmt.Sprintf("section %q has invalid color %q (valid: gray, red, orange, yellow, green, cyan, blue, purple, magenta, white), using default", name, color),
			})
		}
	}

	return true
}

func parseTaskHeadingLine(line string, currentTask **format.ParsedTask, parsedTasks *[]*format.ParsedTask, descBuilder *strings.Builder, nextShortID *int, foundFirstTask *bool, metadataParsed *bool, warnings *[]ParseWarning) bool {
	matches := headingRegex.FindStringSubmatch(line)
	if matches == nil {
		return false
	}

	*foundFirstTask = true
	if *currentTask != nil {
		finalizeParsedTask(*currentTask, parsedTasks, descBuilder)
	}

	shortID := *nextShortID
	taskType := parseTaskTypeForHeading(strings.ToLower(matches[2]), shortID, warnings)
	task := newTaskForHeading(shortID, taskType, strings.TrimSpace(matches[3]), warnings)
	*currentTask = &format.ParsedTask{
		Task:  task,
		Level: len(matches[1]),
	}
	*nextShortID = shortID + 1
	descBuilder.Reset()
	*metadataParsed = false

	return true
}

func parseTaskTypeForHeading(typeStr string, shortID int, warnings *[]ParseWarning) models.TaskType {
	taskType, ok := models.ParseTaskType(typeStr)
	if ok {
		return taskType
	}

	if typeStr == "action" || typeStr == "note" {
		*warnings = append(*warnings, ParseWarning{
			Type:    format.WarningDeprecation,
			Field:   "type",
			Message: fmt.Sprintf("task #%d has deprecated type '%s', migrated to 'task'", shortID, typeStr),
		})
		return models.TaskTypeTask
	}

	*warnings = append(*warnings, ParseWarning{
		Type:    format.WarningValidation,
		Field:   "type",
		Message: fmt.Sprintf("task #%d has unrecognized type '%s', treated as 'task'", shortID, typeStr),
	})

	return models.TaskTypeTask
}

func newTaskForHeading(shortID int, taskType models.TaskType, title string, warnings *[]ParseWarning) *models.Task {
	if len(title) > models.MaxTaskTitleLength {
		// Bypass NewTask validation for overlong titles to avoid a double
		// warning (one here, one from NewTask). The task already exists in
		// the file; we preserve it as-is rather than rejecting it.
		*warnings = append(*warnings, ParseWarning{
			Type:    format.WarningValidation,
			Field:   "title",
			Message: fmt.Sprintf("task #%d title is overlong (%d chars, max %d)", shortID, len(title), models.MaxTaskTitleLength),
		})
		return newTaskForLoad(shortID, taskType, title)
	}

	task, err := models.NewTask("", shortID, taskType, title)
	if err != nil {
		// Preserve tasks with invalid titles (e.g. empty) to avoid data loss.
		*warnings = append(*warnings, ParseWarning{
			Type:    format.WarningDataCorrection,
			Field:   "title",
			Message: fmt.Sprintf("task #%d has invalid title: %v", shortID, err),
		})
		return newTaskForLoad(shortID, taskType, title)
	}

	return resetParsedTaskTimestamps(task)
}

func parseTaskMetadataLine(line string, currentTask *format.ParsedTask, metadataParsed *bool, warnings *[]ParseWarning) bool {
	if currentTask == nil || *metadataParsed {
		return false
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	matches := format.MetadataRegex().FindStringSubmatch(line)
	if matches == nil {
		// Not an HTML comment at all — description has started, stop
		// attempting metadata parsing for this task.
		*metadataParsed = true
		return false
	}
	if !format.LooksLikeMetadata(matches[1]) {
		// HTML comment that isn't a metadata line (e.g. a stray editorial
		// `<!-- note -->`). Treat it as description content and keep looking
		// for the real metadata line on a subsequent row.
		return false
	}

	*metadataParsed = true
	parsedWarnings := format.ParseMetadata(currentTask.Task, matches[1])
	for i := range parsedWarnings {
		parsedWarnings[i].TaskID = currentTask.Task.ID
	}
	*warnings = append(*warnings, parsedWarnings...)

	return true
}

func finalizeParsedTask(currentTask *format.ParsedTask, parsedTasks *[]*format.ParsedTask, descBuilder *strings.Builder) {
	currentTask.Task.Description = strings.TrimSpace(descBuilder.String())
	*parsedTasks = append(*parsedTasks, currentTask)
}

func newTaskForLoad(shortID int, taskType models.TaskType, title string) *models.Task {
	return resetParsedTaskTimestamps(newFallbackTask(shortID, taskType, title))
}

func resetParsedTaskTimestamps(task *models.Task) *models.Task {
	// Zero out timestamps so they come from metadata parsing.
	// NewTask sets these to time.Now(), which is wrong for tasks
	// loaded from disk — their real timestamps are in the metadata.
	task.CreatedAt = time.Time{}
	task.UpdatedAt = time.Time{}
	return task
}

// newFallbackTask creates a Task with safe defaults, bypassing NewTask
// validation. Used when parsing tasks from disk that have invalid titles
// (overlong, empty, etc.) — we preserve them as-is to avoid data loss.
func newFallbackTask(shortID int, taskType models.TaskType, title string) *models.Task {
	return &models.Task{
		ShortID:         shortID,
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

// detectAndReassignDuplicateIDs detects duplicate task IDs and reassigns new
// UUIDs to duplicates. The first task with a given ID keeps it; subsequent
// duplicates get new UUIDs. References (ParentID, DependsOn, RelatedTo) are
// NOT remapped because the first task retains the original ID, so existing
// references to that ID remain valid. Returns the set of all known IDs and
// any warnings generated.
func detectAndReassignDuplicateIDs(tasks []*models.Task) (map[string]bool, []ParseWarning) {
	var warnings []ParseWarning
	taskByID := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if taskByID[t.ID] {
			oldID := t.ID
			t.ID = uuid.New().String()
			warnings = append(warnings, ParseWarning{
				Type:    format.WarningDataCorrection,
				TaskID:  t.ID,
				Field:   "id",
				Message: fmt.Sprintf("task #%d has duplicate ID %q, reassigned to %s", t.ShortID, oldID, t.ID),
			})
		}
		taskByID[t.ID] = true
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

	// Strip a leading UTF-8 BOM so files saved with one still expose their
	// project name. See the matching strip in ParseTasksMarkdownWithSections.
	text := strings.TrimPrefix(string(content), "\uFEFF")
	scanner := bufio.NewScanner(strings.NewReader(text))
	// Match the buffer growth used by the main parser so files with very long
	// first lines (e.g. base64 content, long single-line metadata) don't cause
	// the project-title scan to silently abort. Without this the default 64KB
	// limit would make ParseProjectNameFromFile report "project name not found"
	// for any file whose project line is preceded by an oversize line.
	scanner.Buffer(make([]byte, 0, scannerInitBufSize), scannerMaxBufSize)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := projectRegex.FindStringSubmatch(line); matches != nil {
			return strings.TrimSpace(matches[1]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("%w: %w", tterrors.ErrParseError, err)
	}

	return "", fmt.Errorf("%w: project name not found in file", tterrors.ErrParseError)
}
