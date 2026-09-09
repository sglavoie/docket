package storage

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
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
	return validateTaskProjectDocument(document.ProjectName, document.Tasks, func(i int) error {
		return format.ValidateV6TaskRecord(document.Tasks[i])
	})
}

// validateTaskProjectDocument validates the project-local identity and
// hierarchy invariants every task schema shares. records carries the layout
// fields every schema shares; validateTask validates one task record in the
// caller's own schema, which is what keeps a schema free to spell a durable
// value differently without forking these invariants.
func validateTaskProjectDocument(projectName string, records []format.V6TaskRecord, validateTask func(int) error) error {
	if strings.TrimSpace(projectName) == "" ||
		projectName != strings.TrimSpace(projectName) {
		return fmt.Errorf("v6 project name is required and must be trimmed")
	}
	if len(projectName) > models.MaxProjectNameLength {
		return fmt.Errorf("v6 project name exceeds %d characters", models.MaxProjectNameLength)
	}
	if strings.ContainsAny(projectName, "\n\r") {
		return fmt.Errorf("v6 project name cannot contain newlines")
	}

	byID := make(map[string]format.V6TaskRecord, len(records))
	shortIDs := make(map[int]bool, len(records))
	for i, task := range records {
		if err := validateTask(i); err != nil {
			return err
		}
		if _, duplicate := byID[task.ID]; duplicate {
			return fmt.Errorf("v6 project %q has duplicate task id %q", projectName, task.ID)
		}
		if shortIDs[task.ShortID] {
			return fmt.Errorf("v6 project %q has duplicate short id %d", projectName, task.ShortID)
		}
		byID[task.ID] = task
		shortIDs[task.ShortID] = true
	}
	for _, task := range records {
		if task.ParentID != "" {
			if _, ok := byID[task.ParentID]; !ok {
				return fmt.Errorf("v6 task %q references missing project-local parent %q", task.ID, task.ParentID)
			}
		}
	}
	for _, task := range records {
		seen := map[string]bool{task.ID: true}
		depth := 0
		for parentID := task.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
			if seen[parentID] {
				return fmt.Errorf("v6 project %q contains a parent cycle at task %q", projectName, task.ID)
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
	return writeTaskProjectDocument(document.ProjectName, schemaMarker(SchemaVersionV6), document.Tasks, func(i int) (string, error) {
		return format.WriteV6TaskBody(document.Tasks[i])
	})
}

// writeTaskProjectDocument renders the shared project header, generic Task
// headings, and task bodies. records carries the layout fields every schema
// shares; writeBody renders one task body in the caller's own schema.
func writeTaskProjectDocument(projectName string, marker string, records []format.V6TaskRecord, writeBody func(int) (string, error)) (string, error) {
	byID := make(map[string]format.V6TaskRecord, len(records))
	for _, task := range records {
		byID[task.ID] = task
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "# Project: %s\n", projectName)
	fmt.Fprintf(&builder, "%s\n\n", marker)
	for i, task := range records {
		level := min(2+v6TaskDepth(task, byID), 6)
		fmt.Fprintf(&builder, "%s Task: %s\n", strings.Repeat("#", level), task.Title)
		body, err := writeBody(i)
		if err != nil {
			return "", err
		}
		builder.WriteString(body)
		if i < len(records)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String(), nil
}

// admitProjectCommentsDelimiter decides whether a delimiter line may open a
// block here. The block is document-level and singular, so it belongs after the
// schema marker, before the first task heading, and exactly once; a closing
// delimiter reached in this position closes nothing.
func admitProjectCommentsDelimiter(label, line string, schemaSeen bool, projectName string, hasCurrent, commentsSeen bool) error {
	if line == format.ProjectCommentsEndLine {
		return fmt.Errorf("%s PROJECT-COMMENTS block is closed before it is opened", label)
	}
	if !schemaSeen || projectName == "" {
		return fmt.Errorf("%s PROJECT-COMMENTS block appears before the schema marker", label)
	}
	if hasCurrent {
		return fmt.Errorf("%s PROJECT-COMMENTS block appears after a task heading", label)
	}
	if commentsSeen {
		return fmt.Errorf("duplicate %s PROJECT-COMMENTS block", label)
	}
	return nil
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
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV6, "v6")
	if err != nil {
		return V6ProjectDocument{}, err
	}
	document := V6ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV6TaskBody(block.Title, block.Body)
		if err != nil {
			return V6ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV6ProjectDocument(document); err != nil {
		return V6ProjectDocument{}, err
	}
	return document, nil
}

// scannedTaskDocument is the schema-independent split of project Markdown into
// its project name, optional project-comment block, and raw task blocks. Every
// task schema shares this frame and decodes the bodies itself.
type scannedTaskDocument struct {
	ProjectName string
	Comments    []format.ProjectCommentRecord
	Tasks       []scannedTaskBlock
}

type scannedTaskBlock struct {
	Title string
	Body  string
}

// scanTaskProjectDocument splits project Markdown for a schema that carries no
// document-level block. Every historical codec reads through it, so its accepted
// grammar is frozen: a PROJECT-COMMENTS block is an unexpected header line here,
// exactly as it was before that block existed.
func scanTaskProjectDocument(content string, schema int, label string) (scannedTaskDocument, error) {
	return scanTaskDocumentFrame(content, schema, label, false)
}

// scanTaskProjectDocumentWithComments splits project Markdown for a schema that
// admits the optional PROJECT-COMMENTS block between the schema marker and the
// first task heading.
func scanTaskProjectDocumentWithComments(content string, schema int, label string) (scannedTaskDocument, error) {
	return scanTaskDocumentFrame(content, schema, label, true)
}

//nolint:gocyclo // Keeping the whole strict header grammar in one pass makes the schema frame auditable.
func scanTaskDocumentFrame(content string, schema int, label string, allowProjectComments bool) (scannedTaskDocument, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, scannerInitBufSize), 16*1024*1024)

	var document scannedTaskDocument
	var schemaSeen bool
	var hasCurrent bool
	var currentTitle string
	var currentBody []string
	var commentsSeen bool
	var inComments bool
	var commentLines []string

	flush := func() {
		if !hasCurrent {
			return
		}
		document.Tasks = append(document.Tasks, scannedTaskBlock{
			Title: currentTitle,
			Body:  strings.Join(currentBody, "\n"),
		})
		hasCurrent = false
		currentTitle = ""
		currentBody = nil
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		// The block is read before any other rule, blank lines included: its bodies
		// are prose that may spell anything the escaping does not reserve, so a
		// comment line must never be measured against the header grammar.
		if inComments {
			if line == format.ProjectCommentsEndLine {
				parsed, err := format.ParseProjectCommentsBlock(commentLines)
				if err != nil {
					return scannedTaskDocument{}, fmt.Errorf("%s project comments: %w", label, err)
				}
				document.Comments = parsed
				inComments = false
				commentLines = nil
				continue
			}
			commentLines = append(commentLines, line)
			continue
		}
		if allowProjectComments && (line == format.ProjectCommentsStartLine || line == format.ProjectCommentsEndLine) {
			if err := admitProjectCommentsDelimiter(label, line, schemaSeen, document.ProjectName, hasCurrent, commentsSeen); err != nil {
				return scannedTaskDocument{}, err
			}
			commentsSeen = true
			inComments = true
			continue
		}
		if match := v6TaskHeadingRegex.FindStringSubmatch(line); match != nil {
			if !schemaSeen || document.ProjectName == "" {
				return scannedTaskDocument{}, fmt.Errorf("%s task heading appears before project header", label)
			}
			flush()
			title := match[2]
			if title != strings.TrimSpace(title) || title == "" {
				return scannedTaskDocument{}, fmt.Errorf("%s task heading title is empty or untrimmed", label)
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
				return scannedTaskDocument{}, fmt.Errorf("duplicate %s project heading", label)
			}
			if schemaSeen {
				return scannedTaskDocument{}, fmt.Errorf("%s project heading appears after schema marker", label)
			}
			document.ProjectName = strings.TrimSpace(match[1])
			continue
		}
		if match := schemaMarkerRegex.FindStringSubmatch(line); match != nil {
			if document.ProjectName == "" {
				return scannedTaskDocument{}, fmt.Errorf("%s schema marker appears before project heading", label)
			}
			if schemaSeen {
				return scannedTaskDocument{}, fmt.Errorf("duplicate %s schema marker", label)
			}
			if match[1] != strconv.Itoa(schema) {
				return scannedTaskDocument{}, fmt.Errorf("unsupported %s task schema %s", label, match[1])
			}
			schemaSeen = true
			continue
		}
		return scannedTaskDocument{}, fmt.Errorf("unexpected %s project header line %q", label, line)
	}
	if err := scanner.Err(); err != nil {
		return scannedTaskDocument{}, fmt.Errorf("scan %s task document: %w", label, err)
	}
	if inComments {
		return scannedTaskDocument{}, fmt.Errorf("unterminated %s PROJECT-COMMENTS block", label)
	}
	flush()
	if document.ProjectName == "" {
		return scannedTaskDocument{}, fmt.Errorf("%s task document is missing project name", label)
	}
	if !schemaSeen {
		return scannedTaskDocument{}, fmt.Errorf("%s task document is missing schema marker", label)
	}
	return document, nil
}
