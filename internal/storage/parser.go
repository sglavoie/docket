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
)

var (
	// Matches headings like "## Epic: Title" or "### Story: Title"
	headingRegex = regexp.MustCompile(`^(#{2,6})\s+(Epic|Story|Task|Bug|Idea):\s*(.+)$`)

	// Matches metadata comments like "<!-- id: uuid, status: todo, priority: 1 -->"
	metadataRegex = regexp.MustCompile(`^<!--\s*(.+?)\s*-->$`)

	// Matches project title like "# Project: Name"
	projectRegex = regexp.MustCompile(`^#\s+Project:\s*(.+)$`)
)

// ParsedTask represents a task parsed from markdown with its raw data.
type ParsedTask struct {
	Task       *models.Task
	Level      int    // Heading level (2 for ##, 3 for ###, etc.)
	RawContent string // The description content between this task and the next
}

// ParseTasksMarkdown parses a tasks.md file content into tasks.
// Returns the project name and a slice of tasks.
func ParseTasksMarkdown(content string) (string, []*models.Task, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var projectName string
	var tasks []*models.Task
	var currentTask *ParsedTask
	var descBuilder strings.Builder
	var nextShortID int = 1

	for scanner.Scan() {
		line := scanner.Text()

		// Check for project title
		if matches := projectRegex.FindStringSubmatch(line); matches != nil {
			projectName = strings.TrimSpace(matches[1])
			continue
		}

		// Check for task heading
		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			// Save previous task's description
			if currentTask != nil {
				currentTask.Task.Description = strings.TrimSpace(descBuilder.String())
				tasks = append(tasks, currentTask.Task)
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
		tasks = append(tasks, currentTask.Task)
	}

	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("error reading markdown: %w", err)
	}

	// Build parent-child relationships based on heading levels
	buildHierarchy(tasks, content)

	return projectName, tasks, nil
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
			if p, err := strconv.Atoi(value); err == nil && p >= 1 && p <= 5 {
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
			if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
				task.CreatedAt = t
			}
		case "updated":
			if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
				task.UpdatedAt = t
			}
		case "completed":
			if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
				task.CompletedAt = &t
			}
		case "due":
			if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
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
func buildHierarchy(tasks []*models.Task, content string) {
	// Re-parse to get levels
	scanner := bufio.NewScanner(strings.NewReader(content))
	taskIndex := 0
	levels := make([]int, len(tasks))

	for scanner.Scan() {
		line := scanner.Text()
		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			if taskIndex < len(tasks) {
				levels[taskIndex] = len(matches[1])
				taskIndex++
			}
		}
	}

	// Build hierarchy using a stack
	// Stack tracks the most recent task at each level
	levelStack := make(map[int]*models.Task)

	for i, task := range tasks {
		level := levels[i]

		// If task already has an explicit parent ID from metadata, skip
		if task.ParentID != nil {
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
}
