package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Note represents a project-level markdown document.
type Note struct {
	Title       string    `yaml:"title"`
	Tags        []string  `yaml:"tags,omitempty"`
	CreatedAt   time.Time `yaml:"created"`
	ModifiedAt  time.Time `yaml:"modified"`
	LinkedTasks []string  `yaml:"linked_tasks,omitempty"`
	Pinned      bool      `yaml:"pinned,omitempty"`

	// Filename is the on-disk filename (e.g., "2025-01-28-my-note.md").
	Filename string `yaml:"-"`

	// Body is the markdown content after the frontmatter.
	Body string `yaml:"-"`

	// ParseError holds the parse error message if frontmatter failed to parse.
	// Non-empty means the note is in an error state and only Filename is reliable.
	ParseError string `yaml:"-"`
}

// NewNote creates a new note with a dated slug filename.
func NewNote(title string) *Note {
	now := time.Now()
	return &Note{
		Title:       title,
		Tags:        []string{},
		CreatedAt:   now,
		ModifiedAt:  now,
		LinkedTasks: []string{},
		Filename:    generateFilename(title, now),
		Body:        "",
	}
}

// AddTag adds a tag if it is non-empty and not already present.
func (n *Note) AddTag(tag string) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return
	}
	for _, t := range n.Tags {
		if t == tag {
			return
		}
	}
	n.Tags = append(n.Tags, tag)
	n.ModifiedAt = time.Now()
}

// SetTags replaces the note's tags, filtering out empty strings and duplicates.
func (n *Note) SetTags(tags []string) {
	seen := make(map[string]bool)
	var cleaned []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		cleaned = append(cleaned, t)
	}
	if cleaned == nil {
		cleaned = []string{}
	}
	n.Tags = cleaned
	n.ModifiedAt = time.Now()
}

// AddLink adds a task UUID to the linked tasks list if not already present.
func (n *Note) AddLink(taskID string) {
	for _, id := range n.LinkedTasks {
		if id == taskID {
			return
		}
	}
	n.LinkedTasks = append(n.LinkedTasks, taskID)
	n.ModifiedAt = time.Now()
}

// RemoveLink removes a task UUID from the linked tasks list.
func (n *Note) RemoveLink(taskID string) {
	for i, id := range n.LinkedTasks {
		if id == taskID {
			n.LinkedTasks = append(n.LinkedTasks[:i], n.LinkedTasks[i+1:]...)
			n.ModifiedAt = time.Now()
			return
		}
	}
}

// HasLink returns true if the note is linked to the given task UUID.
func (n *Note) HasLink(taskID string) bool {
	for _, id := range n.LinkedTasks {
		if id == taskID {
			return true
		}
	}
	return false
}

// TitleFromFilename derives a human-readable title from a note filename
// by stripping the date prefix and .md extension, then replacing hyphens with spaces.
func TitleFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	// Strip date prefix like "2025-01-28-150405-"
	if len(name) > 18 && name[4] == '-' && name[7] == '-' && name[10] == '-' {
		name = name[18:]
	}
	return strings.ReplaceAll(name, "-", " ")
}

// slugify converts a title to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)

	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}

	// Collapse multiple dashes
	result := b.String()
	re := regexp.MustCompile(`-{2,}`)
	result = re.ReplaceAllString(result, "-")
	result = strings.Trim(result, "-")

	if result == "" {
		result = "untitled"
	}

	// Limit length
	if len(result) > 60 {
		result = result[:60]
		result = strings.TrimRight(result, "-")
	}

	return result
}

// GenerateNoteFilename creates a dated slug filename from a title and time.
func GenerateNoteFilename(title string, t time.Time) string {
	return generateFilename(title, t)
}

// generateFilename creates a dated slug filename from a title.
func generateFilename(title string, t time.Time) string {
	slug := slugify(title)
	return fmt.Sprintf("%s-%s.md", t.Format("2006-01-02-150405"), slug)
}
