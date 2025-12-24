package storage

import (
	"bufio"
	"fmt"
	"log"
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
)

// ParsedTask represents a task parsed from markdown with its raw data.
type ParsedTask struct {
	Task       *models.Task
	Level      int    // Heading level (2 for ##, 3 for ###, etc.)
	RawContent string // The description content between this task and the next
}

// ParseResult contains the results of parsing a tasks.md file.
type ParseResult struct {
	ProjectName   string
	Tasks         []*models.Task
	Sections      []string
	SectionColors map[string]string
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
				parseMetadata(currentTask.Task, matches[1])
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

	// Validate parent references - clear any that reference non-existent tasks
	taskByID := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		taskByID[t.ID] = true
	}
	for _, t := range tasks {
		if t.ParentID != nil && !taskByID[*t.ParentID] {
			log.Printf("warning: task %s references non-existent parent %s, clearing parent reference", t.ID, *t.ParentID)
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

// parseMetadata parses the metadata string and sets task fields.
func parseMetadata(task *models.Task, metaStr string) {
	pairs := strings.Split(metaStr, ",")
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
				log.Printf("warning: invalid priority '%s' for task, using default", value)
			} else if p < 1 || p > 5 {
				log.Printf("warning: priority %d out of range (1-5) for task, using default", p)
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
		}
	}
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
