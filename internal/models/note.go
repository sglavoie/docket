package models

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Pre-compiled regexp for note slugify to avoid recompilation on each call.
var noteMultiDashRe = regexp.MustCompile(`-{2,}`)

// Note represents a workspace note. Notes may belong to a project or to the
// workspace-global note collection; location is supplied separately by the
// application layer rather than persisted in the note itself.
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

	// RawRevision is the SHA-256 revision of the exact canonical file bytes that
	// were loaded. It is deliberately not serialized: callers carry it back to
	// the application mutation boundary as the optimistic-concurrency precondition
	// for update, rename, move, and delete operations.
	RawRevision string `yaml:"-" json:"-"`
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
	tag = normalizeTag(tag)
	if tag == "" {
		return
	}
	if slices.Contains(n.Tags, tag) {
		return
	}
	n.Tags = append(n.Tags, tag)
	n.ModifiedAt = time.Now()
}

// SetTags replaces the note's tags, filtering out empty strings and duplicates.
func (n *Note) SetTags(tags []string) {
	seen := make(map[string]bool)
	var cleaned []string
	for _, t := range tags {
		t = normalizeTag(t)
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
// Empty IDs are silently rejected so a malformed input cannot corrupt the
// LinkedTasks slice and cause every empty lookup to match.
func (n *Note) AddLink(taskID string) {
	if taskID == "" {
		return
	}
	if slices.Contains(n.LinkedTasks, taskID) {
		return
	}
	n.LinkedTasks = append(n.LinkedTasks, taskID)
	n.ModifiedAt = time.Now()
}

// RemoveLink removes a task UUID from the linked tasks list.
// Empty IDs are a no-op (mirrors AddLink so we never accidentally remove an
// orphaned empty entry that snuck in via a corrupted file).
func (n *Note) RemoveLink(taskID string) {
	if taskID == "" {
		return
	}
	for i, id := range n.LinkedTasks {
		if id == taskID {
			n.LinkedTasks = append(n.LinkedTasks[:i], n.LinkedTasks[i+1:]...)
			n.ModifiedAt = time.Now()
			return
		}
	}
}

// HasLink returns true if the note is linked to the given task UUID.
// An empty taskID always returns false so a missing argument cannot
// accidentally match a malformed empty entry stored on disk.
func (n *Note) HasLink(taskID string) bool {
	if taskID == "" {
		return false
	}
	return slices.Contains(n.LinkedTasks, taskID)
}

// TitleFromFilename derives a human-readable title from a note filename
// by stripping the date prefix and .md extension, then replacing hyphens with spaces.
// Handles three filename formats:
//   - New (8-hex): "2025-01-28-150405_xxxxxxxx-slug.md" (17+1+8+1 = 27-char prefix)
//   - Old (4-hex): "2025-01-28-150405_xxxx-slug.md" (17+1+4+1 = 23-char prefix)
//   - Legacy: "2025-01-28-150405-slug.md" (17+1 = 18-char prefix)
func TitleFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	if len(name) > 18 && name[4] == '-' && name[7] == '-' && name[10] == '-' {
		if len(name) > 19 && name[17] == '_' {
			// Suffixed format: find the first '-' after the '_' to locate slug start
			if idx := strings.Index(name[18:], "-"); idx >= 0 {
				name = name[18+idx+1:]
			}
		} else {
			// Legacy format: strip 18-char prefix
			name = name[18:]
		}
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
	result = noteMultiDashRe.ReplaceAllString(result, "-")
	result = strings.Trim(result, "-")

	if result == "" {
		result = "untitled"
	}

	// Limit length (rune-aware to avoid splitting multi-byte UTF-8 characters)
	runes := []rune(result)
	if len(runes) > 60 {
		result = string(runes[:60])
		result = strings.TrimRight(result, "-")
	}

	return result
}

// GenerateNoteFilename creates a dated slug filename from a title and time.
func GenerateNoteFilename(title string, t time.Time) string {
	return generateFilename(title, t)
}

// generateFilename creates a dated slug filename from a title.
// An 8-character random hex suffix is appended after the timestamp to reduce
// collision probability when two notes are created with the same title in the same second.
func generateFilename(title string, t time.Time) string {
	slug := slugify(title)
	var b [4]byte
	_, _ = rand.Read(b[:])
	suffix := fmt.Sprintf("%02x%02x%02x%02x", b[0], b[1], b[2], b[3])
	return fmt.Sprintf("%s_%s-%s.md", t.Format("2006-01-02-150405"), suffix, slug)
}
