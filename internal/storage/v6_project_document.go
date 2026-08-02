package storage

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

// SchemaVersionV6 identifies the strict consolidated task Markdown schema.
// The codec is introduced independently of normal-startup activation.
const SchemaVersionV6 = 6

var v6TaskHeadingRegex = regexp.MustCompile(`^(#{2,6})\s+Task:\s*(.*)$`)

// V6ProjectDocument is one project task file in schema 6. Project-level tag
// catalogs and retired task concepts deliberately have no representation.
type V6ProjectDocument struct {
	ProjectName string
	Tasks       []format.V6TaskRecord
}

// ValidateV6ProjectDocument validates task records plus project-local identity
// and hierarchy invariants without consulting runtime models or schema 5.
func ValidateV6ProjectDocument(document V6ProjectDocument) error {
	if strings.TrimSpace(document.ProjectName) == "" ||
		document.ProjectName != strings.TrimSpace(document.ProjectName) {
		return fmt.Errorf("v6 project name is required and must be trimmed")
	}
	if len(document.ProjectName) > models.MaxProjectNameLength {
		return fmt.Errorf("v6 project name exceeds %d characters", models.MaxProjectNameLength)
	}
	if strings.ContainsAny(document.ProjectName, "\n\r") {
		return fmt.Errorf("v6 project name cannot contain newlines")
	}

	byID := make(map[string]format.V6TaskRecord, len(document.Tasks))
	shortIDs := make(map[int]bool, len(document.Tasks))
	for _, task := range document.Tasks {
		if err := format.ValidateV6TaskRecord(task); err != nil {
			return err
		}
		if _, duplicate := byID[task.ID]; duplicate {
			return fmt.Errorf("v6 project %q has duplicate task id %q", document.ProjectName, task.ID)
		}
		if shortIDs[task.ShortID] {
			return fmt.Errorf("v6 project %q has duplicate short id %d", document.ProjectName, task.ShortID)
		}
		byID[task.ID] = task
		shortIDs[task.ShortID] = true
	}
	for _, task := range document.Tasks {
		if task.ParentID != "" {
			if _, ok := byID[task.ParentID]; !ok {
				return fmt.Errorf("v6 task %q references missing project-local parent %q", task.ID, task.ParentID)
			}
		}
	}
	for _, task := range document.Tasks {
		seen := map[string]bool{task.ID: true}
		depth := 0
		for parentID := task.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
			if seen[parentID] {
				return fmt.Errorf("v6 project %q contains a parent cycle at task %q", document.ProjectName, task.ID)
			}
			seen[parentID] = true
			depth++
			if depth > models.MaxNestingDepth {
				return fmt.Errorf("v6 task %q is nested at depth %d, exceeding maximum depth %d", task.ID, depth, models.MaxNestingDepth)
			}
		}
	}
	return nil
}

// WriteV6ProjectDocument renders canonical schema-6 Markdown using only
// generic Task headings. Parent metadata remains authoritative for hierarchy;
// heading levels are a readable projection and cap at Markdown's h6.
func WriteV6ProjectDocument(document V6ProjectDocument) (string, error) {
	if err := ValidateV6ProjectDocument(document); err != nil {
		return "", err
	}
	byID := make(map[string]format.V6TaskRecord, len(document.Tasks))
	for _, task := range document.Tasks {
		byID[task.ID] = task
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "# Project: %s\n", document.ProjectName)
	fmt.Fprintf(&builder, "<!-- tt-schema: %d -->\n\n", SchemaVersionV6)
	for i, task := range document.Tasks {
		level := min(2+v6TaskDepth(task, byID), 6)
		fmt.Fprintf(&builder, "%s Task: %s\n", strings.Repeat("#", level), task.Title)
		body, err := format.WriteV6TaskBody(task)
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

func v6TaskDepth(task format.V6TaskRecord, byID map[string]format.V6TaskRecord) int {
	depth := 0
	for parentID := task.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
		depth++
	}
	return depth
}

// ParseV6ProjectDocument strictly parses schema-6 project Markdown. Other
// schema versions, typed task headings, project catalogs, unknown metadata,
// and retired rich blocks are rejected rather than normalized.
func ParseV6ProjectDocument(content string) (V6ProjectDocument, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, scannerInitBufSize), 16*1024*1024)

	var document V6ProjectDocument
	var schemaSeen bool
	var hasCurrent bool
	var currentTitle string
	var currentBody []string

	flush := func() error {
		if !hasCurrent {
			return nil
		}
		task, err := format.ParseV6TaskBody(currentTitle, strings.Join(currentBody, "\n"))
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
		if match := v6TaskHeadingRegex.FindStringSubmatch(line); match != nil {
			if !schemaSeen || document.ProjectName == "" {
				return V6ProjectDocument{}, fmt.Errorf("v6 task heading appears before project header")
			}
			if err := flush(); err != nil {
				return V6ProjectDocument{}, err
			}
			title := match[2]
			if title != strings.TrimSpace(title) || title == "" {
				return V6ProjectDocument{}, fmt.Errorf("v6 task heading title is empty or untrimmed")
			}
			hasCurrent = true
			currentTitle = title
			currentBody = nil
			continue
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
				return V6ProjectDocument{}, fmt.Errorf("duplicate v6 project heading")
			}
			if schemaSeen {
				return V6ProjectDocument{}, fmt.Errorf("v6 project heading appears after schema marker")
			}
			document.ProjectName = strings.TrimSpace(match[1])
			continue
		}
		if match := schemaMarkerRegex.FindStringSubmatch(line); match != nil {
			if document.ProjectName == "" {
				return V6ProjectDocument{}, fmt.Errorf("v6 schema marker appears before project heading")
			}
			if schemaSeen {
				return V6ProjectDocument{}, fmt.Errorf("duplicate v6 schema marker")
			}
			if match[1] != "6" {
				return V6ProjectDocument{}, fmt.Errorf("unsupported v6 task schema %s", match[1])
			}
			schemaSeen = true
			continue
		}
		return V6ProjectDocument{}, fmt.Errorf("unexpected v6 project header line %q", line)
	}
	if err := scanner.Err(); err != nil {
		return V6ProjectDocument{}, fmt.Errorf("scan v6 task document: %w", err)
	}
	if err := flush(); err != nil {
		return V6ProjectDocument{}, err
	}
	if document.ProjectName == "" {
		return V6ProjectDocument{}, fmt.Errorf("v6 task document is missing project name")
	}
	if !schemaSeen {
		return V6ProjectDocument{}, fmt.Errorf("v6 task document is missing schema marker")
	}
	if err := ValidateV6ProjectDocument(document); err != nil {
		return V6ProjectDocument{}, err
	}
	return document, nil
}
