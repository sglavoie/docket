package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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

// ensureDirPath creates a directory if it doesn't exist.
func ensureDirPath(dir string) error {
	return os.MkdirAll(dir, DirPermissions)
}

func validateNoteFilename(filename string) error {
	switch {
	case filename == "":
		return fmt.Errorf("invalid note filename %q: empty filename", filename)
	case filepath.IsAbs(filename):
		return fmt.Errorf("invalid note filename %q: absolute paths are not allowed", filename)
	case filename == "." || filename == "..":
		return fmt.Errorf("invalid note filename %q: path traversal is not allowed", filename)
	case strings.Contains(filename, "/") || strings.Contains(filename, "\\"):
		return fmt.Errorf("invalid note filename %q: path separators are not allowed", filename)
	default:
		return nil
	}
}

// LoadNote reads a single note from its file.
func LoadNote(project *models.Project, filename string) (*models.Note, error) {
	return LoadNoteFrom(NotesDir(project), filename)
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
	return parseNote(filename, string(data))
}

// SaveNote writes a note to disk as a markdown file with YAML frontmatter.
// If the note is new (file doesn't exist yet) and a collision is detected,
// it appends -2, -3 etc. to the base name to avoid overwriting.
func SaveNote(project *models.Project, note *models.Note) error {
	return SaveNoteTo(NotesDir(project), note)
}

// SaveNoteTo writes a note to a directory path.
func SaveNoteTo(dir string, note *models.Note) error {
	if err := ensureDirPath(dir); err != nil {
		return err
	}

	path, err := NoteFilePathIn(dir, note.Filename)
	if err != nil {
		return err
	}

	// Check for collision: if file exists but this is a new note (no prior content on disk)
	// we need to deduplicate the filename. Treat unreadable existing files
	// as a collision (refuse to overwrite) rather than silently clobbering
	// them — losing the existing content would be unrecoverable.
	if _, err := os.Stat(path); err == nil {
		existing, loadErr := LoadNoteFrom(dir, note.Filename)
		needsDedup := loadErr != nil || !existing.CreatedAt.Equal(note.CreatedAt)
		if needsDedup {
			base := strings.TrimSuffix(note.Filename, ".md")
			const maxDedup = 1000
			found := false
			for i := 2; i <= maxDedup; i++ {
				candidate := fmt.Sprintf("%s-%d.md", base, i)
				candidatePath := filepath.Join(dir, candidate)
				if _, statErr := os.Stat(candidatePath); os.IsNotExist(statErr) {
					note.Filename = candidate
					path = candidatePath
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("too many notes with base name %q (tried %d variants)", base, maxDedup)
			}
		}
	}

	content, err := renderNote(note)
	if err != nil {
		return err
	}
	return util.AtomicWriteFile(path, []byte(content), FilePermissions)
}

// ListNotes returns all notes in a project sorted by modified date (newest first),
// with pinned notes always appearing first.
func ListNotes(project *models.Project) ([]*models.Note, error) {
	return ListNotesFrom(NotesDir(project))
}

// ListNotesFrom returns all notes from a directory path.
func ListNotesFrom(dir string) ([]*models.Note, error) {
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
			logging.Warn("skipping unreadable note %s: %v", entry.Name(), err)
			continue
		}
		if note.ParseError != "" {
			logging.Warn("note %s has parse error: %s", entry.Name(), note.ParseError)
		}
		notes = append(notes, note)
	}

	sort.Slice(notes, func(i, j int) bool {
		// Pinned first, then normal, then broken
		iErr := notes[i].ParseError != ""
		jErr := notes[j].ParseError != ""
		if iErr != jErr {
			return !iErr // non-error sorts before error
		}
		if !iErr && notes[i].Pinned != notes[j].Pinned {
			return notes[i].Pinned
		}
		return notes[i].ModifiedAt.After(notes[j].ModifiedAt)
	})

	return notes, nil
}

// DeleteNote removes a note file from disk.
func DeleteNote(project *models.Project, filename string) error {
	return DeleteNoteFrom(NotesDir(project), filename)
}

// DeleteNoteFrom removes a note file from a directory path.
func DeleteNoteFrom(dir, filename string) error {
	path, err := NoteFilePathIn(dir, filename)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// NotesExist returns true if the project has any notes.
func NotesExist(project *models.Project) bool {
	return notesExistIn(NotesDir(project))
}

// notesExistIn returns true if the given directory has any notes.
func notesExistIn(dir string) bool {
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

// RenameNoteIn changes a note's title and filename in a directory path.
func RenameNoteIn(dir, oldFilename, newTitle string) (*models.Note, error) {
	note, err := LoadNoteFrom(dir, oldFilename)
	if err != nil {
		return nil, fmt.Errorf("load note: %w", err)
	}

	note.Title = newTitle
	note.ModifiedAt = time.Now()
	newFilename := models.GenerateNoteFilename(newTitle, note.CreatedAt)

	if newFilename != oldFilename {
		base := strings.TrimSuffix(newFilename, ".md")
		const maxDedup = 1000
		candidate := newFilename
		found := false
		candidatePath := filepath.Join(dir, candidate)
		if _, statErr := os.Stat(candidatePath); os.IsNotExist(statErr) {
			found = true
		}
		if !found {
			for i := 2; i <= maxDedup; i++ {
				candidate = fmt.Sprintf("%s-%d.md", base, i)
				candidatePath = filepath.Join(dir, candidate)
				if _, statErr := os.Stat(candidatePath); os.IsNotExist(statErr) {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("too many notes with base name %q (tried %d variants)", base, maxDedup)
		}
		newFilename = candidate
	}
	note.Filename = newFilename

	if err := SaveNoteTo(dir, note); err != nil {
		return nil, fmt.Errorf("save renamed note: %w", err)
	}

	if note.Filename != oldFilename {
		oldPath := filepath.Join(dir, oldFilename)
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			logging.Warn("note rename: old file remains at %s: %v", oldPath, err)
		}
	}

	return note, nil
}

// NoteFilePath returns the full path to a note file.
func NoteFilePath(project *models.Project, filename string) (string, error) {
	return NoteFilePathIn(NotesDir(project), filename)
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
// On parse failure, it returns a partial Note with ParseError set instead of an error.
func parseNote(filename, content string) (*models.Note, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return &models.Note{
			Filename:    filename,
			Title:       models.TitleFromFilename(filename),
			Tags:        []string{},
			LinkedTasks: []string{},
			ParseError:  fmt.Sprintf("invalid frontmatter: %v", err),
		}, nil
	}

	var meta frontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return &models.Note{
			Filename:    filename,
			Title:       models.TitleFromFilename(filename),
			Tags:        []string{},
			LinkedTasks: []string{},
			Body:        body,
			ParseError:  fmt.Sprintf("invalid YAML: %v", err),
		}, nil
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
func renderNote(note *models.Note) (string, error) {
	meta := frontmatter{
		Title:       note.Title,
		Tags:        note.Tags,
		Created:     note.CreatedAt.Format(timeFormat),
		Modified:    note.ModifiedAt.Format(timeFormat),
		LinkedTasks: note.LinkedTasks,
		Pinned:      note.Pinned,
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

	rest := content[3:] // skip opening ---
	rest = strings.TrimLeft(rest, "\r\n")

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
