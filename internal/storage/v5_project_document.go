package storage

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

// SchemaVersionV5 is the strict live task Markdown schema.
const SchemaVersionV5 = 5

const scannerInitBufSize = 64 * 1024

var (
	v5TaskHeadingRegex = regexp.MustCompile(`^(#{2,6})\s+Task:\s*(.*)$`)
	headingRegex       = regexp.MustCompile(`^(#{2,6})\s+Task:\s*(.*)$`)
	projectRegex       = regexp.MustCompile(`^#\s+Project:\s*(.+)$`)
	schemaMarkerRegex  = regexp.MustCompile(`^<!--\s*tt-schema:\s*(\d+)\s*-->\s*$`)
)

// V5ProjectDocument is the target project task file. Tags are deliberately not
// stored at project level; callers derive them from Tasks.
type V5ProjectDocument struct {
	ProjectName string
	Tasks       []format.V5TaskRecord
}

// ValidateV5ProjectDocument validates target task records, project-local
// parent references, and identity uniqueness without touching live runtime
// models or tag catalogs.
func ValidateV5ProjectDocument(document V5ProjectDocument) error {
	if strings.TrimSpace(document.ProjectName) == "" ||
		document.ProjectName != strings.TrimSpace(document.ProjectName) {
		return fmt.Errorf("v5 project name is required and must be trimmed")
	}
	if len(document.ProjectName) > models.MaxProjectNameLength {
		return fmt.Errorf("v5 project name exceeds %d characters", models.MaxProjectNameLength)
	}
	if strings.ContainsAny(document.ProjectName, "\n\r") {
		return fmt.Errorf("v5 project name cannot contain newlines")
	}

	byID := make(map[string]format.V5TaskRecord, len(document.Tasks))
	shortIDs := make(map[int]bool, len(document.Tasks))
	for _, task := range document.Tasks {
		if err := format.ValidateV5TaskRecord(task); err != nil {
			return err
		}
		if _, duplicate := byID[task.ID]; duplicate {
			return fmt.Errorf("v5 project %q has duplicate task id %q", document.ProjectName, task.ID)
		}
		if shortIDs[task.ShortID] {
			return fmt.Errorf("v5 project %q has duplicate short id %d", document.ProjectName, task.ShortID)
		}
		byID[task.ID] = task
		shortIDs[task.ShortID] = true
	}
	for _, task := range document.Tasks {
		if task.ParentID != "" {
			if _, ok := byID[task.ParentID]; !ok {
				return fmt.Errorf("v5 task %q references missing project-local parent %q", task.ID, task.ParentID)
			}
		}
	}
	for _, task := range document.Tasks {
		seen := map[string]bool{task.ID: true}
		depth := 0
		for parentID := task.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
			if seen[parentID] {
				return fmt.Errorf("v5 project %q contains a parent cycle at task %q", document.ProjectName, task.ID)
			}
			seen[parentID] = true
			depth++
			if depth > models.MaxNestingDepth {
				return fmt.Errorf("v5 task %q is nested at depth %d, exceeding maximum depth %d", task.ID, depth, models.MaxNestingDepth)
			}
		}
	}
	return nil
}

// WriteV5ProjectDocument renders canonical schema-v5 Markdown. Every heading
// is the generic Task label, and no project tag catalog metadata is emitted.
func WriteV5ProjectDocument(document V5ProjectDocument) (string, error) {
	if err := ValidateV5ProjectDocument(document); err != nil {
		return "", err
	}
	byID := make(map[string]format.V5TaskRecord, len(document.Tasks))
	for _, task := range document.Tasks {
		byID[task.ID] = task
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "# Project: %s\n", document.ProjectName)
	fmt.Fprintf(&builder, "<!-- tt-schema: %d -->\n\n", SchemaVersionV5)
	for i, task := range document.Tasks {
		level := 2 + v5TaskDepth(task, byID)
		level = min(level, 6)
		fmt.Fprintf(&builder, "%s Task: %s\n", strings.Repeat("#", level), task.Title)
		body, err := format.WriteV5TaskBody(task)
		if err != nil {
			return "", err
		}
		builder.WriteString(body)
		if i < len(document.Tasks)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String(), nil
}

func v5TaskDepth(task format.V5TaskRecord, byID map[string]format.V5TaskRecord) int {
	depth := 0
	for parentID := task.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
		depth++
	}
	return depth
}

// ParseV5ProjectDocument strictly parses target schema-v5 Markdown. Legacy
// typed headings, legacy metadata, project tag catalogs, and other schema
// versions are rejected rather than normalized by the live v2 parser.
func ParseV5ProjectDocument(content string) (V5ProjectDocument, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, scannerInitBufSize), 16*1024*1024)

	var document V5ProjectDocument
	var schemaSeen bool
	var hasCurrent bool
	var currentTitle string
	var currentBody []string

	flush := func() error {
		if !hasCurrent {
			return nil
		}
		task, err := format.ParseV5TaskBody(currentTitle, strings.Join(currentBody, "\n"))
		if err != nil {
			return err
		}
		document.Tasks = append(document.Tasks, task)
		hasCurrent = false
		currentTitle = ""
		currentBody = nil
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if match := v5TaskHeadingRegex.FindStringSubmatch(line); match != nil {
			if !schemaSeen || document.ProjectName == "" {
				return V5ProjectDocument{}, fmt.Errorf("v5 task heading appears before project header")
			}
			if err := flush(); err != nil {
				return V5ProjectDocument{}, err
			}
			hasCurrent = true
			currentTitle = strings.TrimSpace(match[2])
			currentBody = nil
			continue
		}
		if headingRegex.MatchString(line) {
			return V5ProjectDocument{}, fmt.Errorf("v5 task document accepts only generic Task headings: %q", line)
		}
		if hasCurrent {
			currentBody = append(currentBody, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if match := projectRegex.FindStringSubmatch(line); match != nil {
			if document.ProjectName != "" {
				return V5ProjectDocument{}, fmt.Errorf("duplicate v5 project heading")
			}
			document.ProjectName = strings.TrimSpace(match[1])
			continue
		}
		if match := schemaMarkerRegex.FindStringSubmatch(line); match != nil {
			if schemaSeen {
				return V5ProjectDocument{}, fmt.Errorf("duplicate v5 schema marker")
			}
			if match[1] != "5" {
				return V5ProjectDocument{}, fmt.Errorf("unsupported v5 task schema %s", match[1])
			}
			schemaSeen = true
			continue
		}
		return V5ProjectDocument{}, fmt.Errorf("unexpected v5 project header line %q", line)
	}
	if err := scanner.Err(); err != nil {
		return V5ProjectDocument{}, fmt.Errorf("scan v5 task document: %w", err)
	}
	if err := flush(); err != nil {
		return V5ProjectDocument{}, err
	}
	if document.ProjectName == "" {
		return V5ProjectDocument{}, fmt.Errorf("v5 task document is missing project name")
	}
	if !schemaSeen {
		return V5ProjectDocument{}, fmt.Errorf("v5 task document is missing schema marker")
	}
	if err := ValidateV5ProjectDocument(document); err != nil {
		return V5ProjectDocument{}, err
	}
	return document, nil
}
