package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const notesDirName = "notes"

// NotesDir returns the path to the notes directory for a project.
func NotesDir(project *models.Project) string {
	return filepath.Join(project.Path, notesDirName)
}

// EnsureNotesDir creates the notes directory if it doesn't exist.
func EnsureNotesDir(project *models.Project) error {
	return os.MkdirAll(NotesDir(project), 0755)
}

// LoadNote reads a single note from its file.
func LoadNote(project *models.Project, filename string) (*models.Note, error) {
	path := filepath.Join(NotesDir(project), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseNote(filename, string(data))
}

// SaveNote writes a note to disk as a markdown file with YAML frontmatter.
func SaveNote(project *models.Project, note *models.Note) error {
	if err := EnsureNotesDir(project); err != nil {
		return err
	}

	content := renderNote(note)
	path := filepath.Join(NotesDir(project), note.Filename)
	return util.AtomicWriteFile(path, []byte(content), 0644)
}

// ListNotes returns all notes in a project sorted by modified date (newest first),
// with pinned notes always appearing first.
func ListNotes(project *models.Project) ([]*models.Note, error) {
	dir := NotesDir(project)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var notes []*models.Note
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		note, err := LoadNote(project, entry.Name())
		if err != nil {
			continue // skip unparseable files
		}
		notes = append(notes, note)
	}

	sort.Slice(notes, func(i, j int) bool {
		if notes[i].Pinned != notes[j].Pinned {
			return notes[i].Pinned
		}
		return notes[i].ModifiedAt.After(notes[j].ModifiedAt)
	})

	return notes, nil
}

// DeleteNote removes a note file from disk.
func DeleteNote(project *models.Project, filename string) error {
	path := filepath.Join(NotesDir(project), filename)
	return os.Remove(path)
}

// NotesExist returns true if the project has any notes.
func NotesExist(project *models.Project) bool {
	dir := NotesDir(project)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			return true
		}
	}
	return false
}

// NoteFilePath returns the full path to a note file.
func NoteFilePath(project *models.Project, filename string) string {
	return filepath.Join(NotesDir(project), filename)
}

// RenderNoteContent serializes a Note to markdown with YAML frontmatter.
func RenderNoteContent(note *models.Note) string {
	return renderNote(note)
}

// ParseNoteContent parses a markdown string with YAML frontmatter into a Note.
func ParseNoteContent(filename, content string) (*models.Note, error) {
	return parseNote(filename, content)
}

// frontmatter holds the YAML frontmatter fields for serialization.
type frontmatter struct {
	Title       string   `yaml:"title"`
	Tags        []string `yaml:"tags,omitempty"`
	Created     string   `yaml:"created"`
	Modified    string   `yaml:"modified"`
	LinkedTasks []string `yaml:"linked_tasks,omitempty"`
	Pinned      bool     `yaml:"pinned,omitempty"`
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// parseNote parses a markdown file with YAML frontmatter into a Note.
func parseNote(filename, content string) (*models.Note, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("invalid frontmatter in %s: %w", filename, err)
	}

	var meta frontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("invalid YAML in %s: %w", filename, err)
	}

	note := &models.Note{
		Title:       meta.Title,
		Tags:        meta.Tags,
		LinkedTasks: meta.LinkedTasks,
		Pinned:      meta.Pinned,
		Filename:    filename,
		Body:        body,
	}

	if note.Tags == nil {
		note.Tags = []string{}
	}
	if note.LinkedTasks == nil {
		note.LinkedTasks = []string{}
	}

	if t, err := parseTime(meta.Created); err == nil {
		note.CreatedAt = t
	}
	if t, err := parseTime(meta.Modified); err == nil {
		note.ModifiedAt = t
	}

	if note.Title == "" {
		note.Title = strings.TrimSuffix(filename, ".md")
	}

	return note, nil
}

// renderNote serializes a Note to markdown with YAML frontmatter.
func renderNote(note *models.Note) string {
	meta := frontmatter{
		Title:       note.Title,
		Tags:        note.Tags,
		Created:     note.CreatedAt.Format(timeFormat),
		Modified:    note.ModifiedAt.Format(timeFormat),
		LinkedTasks: note.LinkedTasks,
		Pinned:      note.Pinned,
	}

	yamlBytes, _ := yaml.Marshal(&meta)
	fm := strings.TrimSpace(string(yamlBytes))

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fm)
	b.WriteString("\n---\n")
	if note.Body != "" {
		b.WriteString("\n")
		b.WriteString(note.Body)
		if !strings.HasSuffix(note.Body, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// splitFrontmatter splits "---\n...\n---\n" from the rest of the content.
func splitFrontmatter(content string) (frontmatter string, body string, err error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", content, fmt.Errorf("missing frontmatter delimiter")
	}

	// Find the closing ---
	rest := content[3:] // skip opening ---
	rest = strings.TrimLeft(rest, "\r\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content, fmt.Errorf("missing closing frontmatter delimiter")
	}

	fm := rest[:idx]
	body = strings.TrimPrefix(rest[idx+4:], "\n")
	body = strings.TrimLeft(body, "\r\n")

	return fm, body, nil
}

// parseTime attempts to parse a time string in the standard format.
func parseTime(s string) (time.Time, error) {
	return time.Parse(timeFormat, s)
}
