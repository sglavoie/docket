package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/models"
)

const notesDirName = "notes"

// NotesDir returns the path to the notes directory for a project.
func NotesDir(project *models.Project) string {
	return filepath.Join(project.Path, notesDirName)
}

// GlobalNotesDir returns the path to the global notes directory.
func GlobalNotesDir(dataDir string) string {
	return filepath.Join(dataDir, ".tt", "global-notes")
}

func validateNoteFilename(filename string) error {
	if err := SafeChildName(filename); err != nil {
		return fmt.Errorf("invalid note filename %q: %w", filename, err)
	}
	return nil
}

// LoadNoteFrom reads a single note from a directory path.
func LoadNoteFrom(dir, filename string) (*models.Note, error) {
	path, err := NoteFilePathIn(dir, filename)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	note, err := parseNote(filename, string(data))
	if err != nil {
		return nil, err
	}
	// Retain the revision of the exact bytes read, including for a note whose
	// frontmatter parsed into a recoverable ParseError value. Any later canonical
	// mutation must compare against these bytes instead of an unlocked stat probe
	// or a re-rendered approximation.
	note.RawRevision = RawArtifactRevision(data)
	return note, nil
}

// ListNotes returns all notes in a project sorted by modified date (newest first),
// with pinned notes always appearing first.
func ListNotes(project *models.Project) ([]*models.Note, error) {
	return ListNotesFrom(NotesDir(project))
}

// ListNotesFrom returns all notes from a directory path.
func ListNotesFrom(dir string) ([]*models.Note, error) {
	return listNotesFrom(dir, false)
}

// ListNotesForMutationFrom returns every canonical note artifact with the exact
// raw revision observed from disk. Unlike the user-facing list path, it fails
// closed when any note cannot be read so a revision-checked whole-collection
// mutation cannot silently skip an artifact.
func ListNotesForMutationFrom(dir string) ([]*models.Note, error) {
	return listNotesFrom(dir, true)
}

func listNotesFrom(dir string, failOnUnreadable bool) ([]*models.Note, error) {
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
		note, err := LoadNoteFrom(dir, entry.Name())
		if err != nil {
			if failOnUnreadable {
				return nil, fmt.Errorf("load note artifact %q: %w", entry.Name(), err)
			}
			logging.Warn("skipping unreadable note %s: %v", entry.Name(), err)
			continue
		}
		if note.ParseError != "" {
			logging.Warn("note %s has parse error: %s", entry.Name(), note.ParseError)
		}
		notes = append(notes, note)
	}

	sort.Slice(notes, func(i, j int) bool {
		// Broken notes remain visible but sort after editable notes. Otherwise
		// the minimal model has one ordering: most recently modified first.
		iErr := notes[i].ParseError != ""
		jErr := notes[j].ParseError != ""
		if iErr != jErr {
			return !iErr // non-error sorts before error
		}
		return notes[i].ModifiedAt.After(notes[j].ModifiedAt)
	})

	return notes, nil
}

// NoteFilePathIn returns the full path to a note file in a directory.
func NoteFilePathIn(dir, filename string) (string, error) {
	if err := validateNoteFilename(filename); err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

// RenderNoteContent serializes a Note to markdown with YAML frontmatter.
func RenderNoteContent(note *models.Note) (string, error) {
	return renderNote(note)
}

// ParseNoteContent parses a markdown string with YAML frontmatter into a Note.
func ParseNoteContent(filename, content string) (*models.Note, error) {
	return parseNote(filename, content)
}

// frontmatter is the complete v5 note metadata contract. Note location is
// represented by the containing directory, while tags, links, and pin state
// are legacy metadata that must not be re-emitted after a note is loaded.
type frontmatter struct {
	Title    string `yaml:"title"`
	Created  string `yaml:"created"`
	Modified string `yaml:"modified"`
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// parseNote parses a markdown file with YAML frontmatter into a Note.
// On parse failure, it returns a partial Note with ParseError set instead of an error.
func parseNote(filename, content string) (*models.Note, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return &models.Note{
			Filename:   filename,
			Title:      models.TitleFromFilename(filename),
			ParseError: fmt.Sprintf("invalid frontmatter: %v", err),
		}, nil
	}

	var meta frontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return &models.Note{
			Filename:   filename,
			Title:      models.TitleFromFilename(filename),
			Body:       body,
			ParseError: fmt.Sprintf("invalid YAML: %v", err),
		}, nil
	}

	note := &models.Note{
		Title:    meta.Title,
		Filename: filename,
		Body:     body,
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
func renderNote(note *models.Note) (string, error) {
	meta := frontmatter{
		Title:    note.Title,
		Created:  note.CreatedAt.Format(timeFormat),
		Modified: note.ModifiedAt.Format(timeFormat),
	}

	yamlBytes, err := yaml.Marshal(&meta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal note frontmatter: %w", err)
	}
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
	return b.String(), nil
}

// splitFrontmatter splits "---\n...\n---\n" from the rest of the content.
// The closing "---" must appear alone on its line so a content line beginning
// with "----" (or any other "---<suffix>") is not mistaken for the delimiter.
// Without this check, a frontmatter value or body line starting with "----"
// would silently truncate the frontmatter and prepend stray characters to the
// body on every parse.
func splitFrontmatter(content string) (frontmatter string, body string, err error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", content, fmt.Errorf("missing frontmatter delimiter")
	}

	// Consume exactly one newline (LF or CRLF) after the opening "---" so the
	// closing delimiter on the very next line — i.e. an empty frontmatter such
	// as "---\n---\n…" — is still recognized. Previously this used
	// strings.TrimLeft(rest, "\r\n") which greedily ate every leading newline,
	// pulling the closing "---" up to position 0 of rest and causing the
	// subsequent "\n---" lookahead to miss it and report "missing closing
	// frontmatter delimiter".
	rest := content[3:]
	rest = strings.TrimPrefix(rest, "\r")
	rest = strings.TrimPrefix(rest, "\n")

	// Special case: empty frontmatter — rest now starts with "---" because the
	// closing delimiter was on the immediately-following line. Without this
	// fast path the lookahead below would search for "\n---" and miss it.
	if strings.HasPrefix(rest, "---") {
		after := rest[3:]
		if len(after) == 0 || after[0] == '\n' || after[0] == '\r' {
			body = strings.TrimLeft(after, "\r\n")
			return "", body, nil
		}
	}

	// Find a "\n---" sequence that is followed by either a newline, CR, or EOF
	// (a real closing delimiter alone on its line, not the prefix of "----").
	searchFrom := 0
	for {
		idx := strings.Index(rest[searchFrom:], "\n---")
		if idx == -1 {
			return "", content, fmt.Errorf("missing closing frontmatter delimiter")
		}
		absIdx := searchFrom + idx
		after := rest[absIdx+4:]
		if len(after) == 0 || after[0] == '\n' || after[0] == '\r' {
			fm := rest[:absIdx]
			body = strings.TrimLeft(after, "\r\n")
			return fm, body, nil
		}
		searchFrom = absIdx + 4
	}
}

// parseTime attempts to parse a time string in the standard format.
func parseTime(s string) (time.Time, error) {
	return time.Parse(timeFormat, s)
}
