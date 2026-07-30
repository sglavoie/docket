package storage

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// LegacyV4Note is the migration-only representation of rich v4 note
// frontmatter. It does not depend on the live note model, whose retired fields
// can therefore be deleted after migration activation.
type LegacyV4Note struct {
	Filename    string
	Title       string
	Tags        []string
	CreatedAt   time.Time
	ModifiedAt  time.Time
	LinkedTasks []string
	Pinned      bool
	Body        string
}

// V5NoteRecord is the complete target note shape. Location remains represented
// by the artifact path, not by frontmatter.
type V5NoteRecord struct {
	Filename   string
	Title      string
	CreatedAt  time.Time
	ModifiedAt time.Time
	Body       string
}

type V5NoteMigrationAuditEntry struct {
	Filename string `json:"filename"`
	Action   string `json:"action"`
	Field    string `json:"field"`
	From     string `json:"from,omitempty"`
	Reason   string `json:"reason"`
}

type V5NoteConversionResult struct {
	Note  V5NoteRecord
	Data  []byte
	Audit []V5NoteMigrationAuditEntry
}

type legacyV4NoteFrontmatter struct {
	Title       string   `yaml:"title"`
	Tags        []string `yaml:"tags,omitempty"`
	Created     string   `yaml:"created"`
	Modified    string   `yaml:"modified"`
	LinkedTasks []string `yaml:"linked_tasks,omitempty"`
	Pinned      bool     `yaml:"pinned,omitempty"`
}

type v5NoteFrontmatter struct {
	Title    string `yaml:"title"`
	Created  string `yaml:"created"`
	Modified string `yaml:"modified"`
}

// DecodeLegacyV4Note decodes rich note frontmatter without constructing a live
// runtime note. Invalid YAML or lifecycle timestamps are hard validation
// failures so a workspace dry run can abort before any migration write.
func DecodeLegacyV4Note(filename string, content []byte) (LegacyV4Note, error) {
	if err := validateNoteFilename(filename); err != nil {
		return LegacyV4Note{}, err
	}
	fm, body, err := splitFrontmatter(string(content))
	if err != nil {
		return LegacyV4Note{}, fmt.Errorf("decode legacy v4 note %q: invalid frontmatter: %w", filename, err)
	}
	var meta legacyV4NoteFrontmatter
	if yamlErr := yaml.Unmarshal([]byte(fm), &meta); yamlErr != nil {
		return LegacyV4Note{}, fmt.Errorf("decode legacy v4 note %q: invalid YAML: %w", filename, yamlErr)
	}
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		return LegacyV4Note{}, fmt.Errorf("decode legacy v4 note %q: title is empty", filename)
	}
	createdAt, err := time.Parse(time.RFC3339, meta.Created)
	if err != nil {
		return LegacyV4Note{}, fmt.Errorf("decode legacy v4 note %q: invalid created timestamp %q", filename, meta.Created)
	}
	modifiedAt, err := time.Parse(time.RFC3339, meta.Modified)
	if err != nil {
		return LegacyV4Note{}, fmt.Errorf("decode legacy v4 note %q: invalid modified timestamp %q", filename, meta.Modified)
	}
	return LegacyV4Note{
		Filename:    filename,
		Title:       meta.Title,
		Tags:        append([]string(nil), meta.Tags...),
		CreatedAt:   createdAt,
		ModifiedAt:  modifiedAt,
		LinkedTasks: append([]string(nil), meta.LinkedTasks...),
		Pinned:      meta.Pinned,
		Body:        body,
	}, nil
}

// ConvertLegacyV4Note produces canonical minimal frontmatter without mutating
// its decoded input.
func ConvertLegacyV4Note(note LegacyV4Note) (V5NoteConversionResult, error) {
	target := V5NoteRecord{
		Filename:   note.Filename,
		Title:      note.Title,
		CreatedAt:  note.CreatedAt,
		ModifiedAt: note.ModifiedAt,
		Body:       note.Body,
	}
	if err := target.Validate(); err != nil {
		return V5NoteConversionResult{}, err
	}

	result := V5NoteConversionResult{Note: target}
	addAudit := func(field, from, reason string) {
		result.Audit = append(result.Audit, V5NoteMigrationAuditEntry{
			Filename: note.Filename,
			Action:   "snapshot-only",
			Field:    field,
			From:     from,
			Reason:   reason,
		})
	}
	if len(note.Tags) > 0 {
		addAudit("tags", strings.Join(note.Tags, ";"), "v5 notes have no tags")
	}
	if len(note.LinkedTasks) > 0 {
		addAudit("linked_tasks", strings.Join(note.LinkedTasks, ";"), "v5 notes have no task links")
	}
	if note.Pinned {
		addAudit("pinned", "true", "v5 notes have no pin state")
	}

	data, err := renderV5Note(target)
	if err != nil {
		return V5NoteConversionResult{}, err
	}
	if err := validateV5NoteBytes(target, data); err != nil {
		return V5NoteConversionResult{}, err
	}
	result.Data = data
	return result, nil
}

func (note V5NoteRecord) Validate() error {
	if err := validateNoteFilename(note.Filename); err != nil {
		return err
	}
	if strings.TrimSpace(note.Title) == "" {
		return fmt.Errorf("v5 note %q has an empty title", note.Filename)
	}
	if note.CreatedAt.IsZero() || note.ModifiedAt.IsZero() {
		return fmt.Errorf("v5 note %q requires created and modified timestamps", note.Filename)
	}
	return nil
}

func renderV5Note(note V5NoteRecord) ([]byte, error) {
	meta := v5NoteFrontmatter{
		Title:    note.Title,
		Created:  note.CreatedAt.Format(time.RFC3339),
		Modified: note.ModifiedAt.Format(time.RFC3339),
	}
	yamlBytes, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("encode v5 note %q frontmatter: %w", note.Filename, err)
	}
	var out strings.Builder
	out.WriteString("---\n")
	out.WriteString(strings.TrimSpace(string(yamlBytes)))
	out.WriteString("\n---\n")
	if note.Body != "" {
		out.WriteString("\n")
		out.WriteString(note.Body)
		if !strings.HasSuffix(note.Body, "\n") {
			out.WriteString("\n")
		}
	}
	return []byte(out.String()), nil
}

func validateV5NoteBytes(expected V5NoteRecord, data []byte) error {
	filename := expected.Filename
	fm, body, err := splitFrontmatter(string(data))
	if err != nil {
		return fmt.Errorf("validate v5 note %q: %w", filename, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(fm), &node); err != nil {
		return fmt.Errorf("validate v5 note %q YAML: %w", filename, err)
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("validate v5 note %q: frontmatter is not a mapping", filename)
	}
	allowed := map[string]bool{"title": true, "created": true, "modified": true}
	seen := make(map[string]bool, len(allowed))
	for index := 0; index < len(node.Content[0].Content); index += 2 {
		key := node.Content[0].Content[index].Value
		if !allowed[key] {
			return fmt.Errorf("validate v5 note %q: unexpected frontmatter field %q", filename, key)
		}
		if seen[key] {
			return fmt.Errorf("validate v5 note %q: duplicate frontmatter field %q", filename, key)
		}
		seen[key] = true
	}
	for key := range allowed {
		if !seen[key] {
			return fmt.Errorf("validate v5 note %q: missing frontmatter field %q", filename, key)
		}
	}
	var meta v5NoteFrontmatter
	if yamlErr := yaml.Unmarshal([]byte(fm), &meta); yamlErr != nil {
		return fmt.Errorf("validate v5 note %q: %w", filename, yamlErr)
	}
	if strings.TrimSpace(meta.Title) == "" {
		return fmt.Errorf("validate v5 note %q: empty title", filename)
	}
	if _, err := time.Parse(time.RFC3339, meta.Created); err != nil {
		return fmt.Errorf("validate v5 note %q: invalid created timestamp", filename)
	}
	if _, err := time.Parse(time.RFC3339, meta.Modified); err != nil {
		return fmt.Errorf("validate v5 note %q: invalid modified timestamp", filename)
	}
	if meta.Title != expected.Title ||
		meta.Created != expected.CreatedAt.Format(time.RFC3339) ||
		meta.Modified != expected.ModifiedAt.Format(time.RFC3339) ||
		body != expected.Body {
		return fmt.Errorf("validate v5 note %q: rendered content does not match candidate", filename)
	}
	return nil
}

// ValidateV5NoteContent validates a standalone target artifact without relying
// on the runtime note model.
func ValidateV5NoteContent(filename string, data []byte) error {
	if err := validateNoteFilename(filename); err != nil {
		return err
	}
	fm, body, err := splitFrontmatter(string(data))
	if err != nil {
		return fmt.Errorf("validate v5 note %q: %w", filename, err)
	}
	var meta v5NoteFrontmatter
	if yamlErr := yaml.Unmarshal([]byte(fm), &meta); yamlErr != nil {
		return fmt.Errorf("validate v5 note %q: %w", filename, yamlErr)
	}
	createdAt, err := time.Parse(time.RFC3339, meta.Created)
	if err != nil {
		return fmt.Errorf("validate v5 note %q: invalid created timestamp", filename)
	}
	modifiedAt, err := time.Parse(time.RFC3339, meta.Modified)
	if err != nil {
		return fmt.Errorf("validate v5 note %q: invalid modified timestamp", filename)
	}
	expected := V5NoteRecord{
		Filename:   filename,
		Title:      meta.Title,
		CreatedAt:  createdAt,
		ModifiedAt: modifiedAt,
		Body:       body,
	}
	if err := expected.Validate(); err != nil {
		return err
	}
	return validateV5NoteBytes(expected, data)
}
