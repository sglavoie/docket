package storage

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/tterrors"
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

			task := models.NewTask("", nextShortID, taskType, title)
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
		return nil, fmt.Errorf("%w: %w", tterrors.ErrParseError, err)
	}

	// Build parent-child relationships using levels captured during initial parse
	tasks := format.BuildHierarchy(parsedTasks)

	// Extract activity and comments from task descriptions
	for _, t := range tasks {
		format.ExtractActivityFromDescription(t)
		format.ExtractCommentsFromDescription(t)
	}

	// Validate parent references - clear any that reference non-existent tasks
	taskByID := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		taskByID[t.ID] = true
	}
	for _, t := range tasks {
		if t.ParentID != nil && !taskByID[*t.ParentID] {
			metadataWarnings = append(metadataWarnings, ParseWarning{
				Type:    format.WarningDataCorrection,
				TaskID:  t.ID,
				Field:   "parent",
				Message: fmt.Sprintf("task %s references non-existent parent %s, clearing parent reference", t.ID, *t.ParentID),
			})
			t.ParentID = nil
		}
	}

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
