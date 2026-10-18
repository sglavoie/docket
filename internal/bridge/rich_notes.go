package bridge

import (
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// ---- notes ----

// noteLocation is a note collection on the wire: global, or one project's.
type noteLocation struct {
	Scope     string `json:"scope"`
	ProjectID string `json:"project_id,omitempty"`
}

func (l noteLocation) app() (app.NoteLocation, *Error) {
	switch app.NoteScope(l.Scope) {
	case app.NoteScopeGlobal:
		if l.ProjectID != "" {
			return app.NoteLocation{}, newError(CodeInvalidRequest, "a global note location names no project_id")
		}
		return app.GlobalNoteLocation(), nil
	case app.NoteScopeProject:
		if l.ProjectID == "" {
			return app.NoteLocation{}, newError(CodeInvalidRequest, "a project note location needs project_id")
		}
		return app.ProjectNoteLocation(l.ProjectID), nil
	}
	return app.NoteLocation{}, newError(CodeInvalidRequest, "note scope %q is not global or project", l.Scope)
}

func wireLocation(location app.NoteLocation) noteLocation {
	return noteLocation{Scope: string(location.Scope), ProjectID: location.ProjectID}
}

// noteView is one note. A list carries an excerpt; a single read and a
// mutation carry the whole body. revision is the raw revision of the bytes
// read, which an update, rename, move, or delete hands back.
type noteView struct {
	Location    noteLocation `json:"location"`
	ProjectName string       `json:"project_name,omitempty"`
	Filename    string       `json:"filename"`
	Title       string       `json:"title"`
	CreatedAt   time.Time    `json:"created_at"`
	ModifiedAt  time.Time    `json:"modified_at"`
	Revision    string       `json:"revision"`
	ParseError  string       `json:"parse_error,omitempty"`
	Excerpt     string       `json:"excerpt"`
	Body        *string      `json:"body,omitempty"`
}

func viewNote(location app.NoteLocation, projectName string, note *models.Note, withBody bool) noteView {
	view := noteView{
		Location:    wireLocation(location),
		ProjectName: projectName,
		Filename:    note.Filename,
		Title:       note.Title,
		CreatedAt:   note.CreatedAt,
		ModifiedAt:  note.ModifiedAt,
		Revision:    note.RawRevision,
		ParseError:  note.ParseError,
		Excerpt:     noteExcerpt(note.Body),
	}
	if view.Title == "" {
		view.Title = models.TitleFromFilename(note.Filename)
	}
	if withBody {
		body := note.Body
		view.Body = &body
	}
	return view
}

// noteExcerpt is a note body's first nonblank line, bounded.
func noteExcerpt(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > 120 {
			return string(runes[:119]) + "…"
		}
		return line
	}
	return ""
}

type notesPayload struct {
	Location *noteLocation `json:"location,omitempty"`
}

type notesResult struct {
	Notes []noteView `json:"notes"`
}

// queryNotes lists one note collection, or every collection a phone can
// reach: the global notes and each available project's. Archived projects are
// excluded, as the desktop's Notes surface excludes them. Newest first.
func (b *Bridge) queryNotes(request Request) (any, error) {
	var payload notesPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	snapshot, err := runtime.Snapshot()
	if err != nil {
		return nil, err
	}
	names := make(map[string]string)
	var locations []app.NoteLocation
	for _, project := range snapshot.Projects() {
		names[project.ID] = project.Name
	}
	if payload.Location != nil {
		location, refusal := payload.Location.app()
		if refusal != nil {
			return nil, refusal
		}
		locations = append(locations, location)
	} else {
		locations = append(locations, app.GlobalNoteLocation())
		for _, project := range snapshot.Projects() {
			locations = append(locations, app.ProjectNoteLocation(project.ID))
		}
	}
	collections, err := runtime.Service().NotesAtLocations(locations)
	if err != nil {
		return nil, err
	}
	result := notesResult{Notes: []noteView{}}
	for _, collection := range collections {
		if collection.Err != nil {
			if payload.Location != nil {
				return nil, collection.Err
			}
			continue
		}
		for _, note := range collection.Notes {
			result.Notes = append(result.Notes, viewNote(collection.Location, names[collection.Location.ProjectID], note, false))
		}
	}
	sort.SliceStable(result.Notes, func(i, j int) bool {
		if !result.Notes[i].ModifiedAt.Equal(result.Notes[j].ModifiedAt) {
			return result.Notes[i].ModifiedAt.After(result.Notes[j].ModifiedAt)
		}
		return result.Notes[i].Filename < result.Notes[j].Filename
	})
	return result, nil
}

type notePayload struct {
	Location noteLocation `json:"location"`
	Filename string       `json:"filename"`
}

type noteResult struct {
	Note noteView `json:"note"`
}

// queryNote reads one note whole.
func (b *Bridge) queryNote(request Request) (any, error) {
	var payload notePayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	location, refusal := payload.Location.app()
	if refusal != nil {
		return nil, refusal
	}
	if err := b.workspace.runtime.Refresh(); err != nil {
		return nil, err
	}
	note, name, err := b.findNote(location, payload.Filename)
	if err != nil {
		return nil, err
	}
	return noteResult{Note: viewNote(location, name, note, true)}, nil
}

// findNote reads one note at a location by its filename, with the name of the
// project that owns the location.
func (b *Bridge) findNote(location app.NoteLocation, filename string) (*models.Note, string, error) {
	if filename == "" {
		return nil, "", newError(CodeInvalidRequest, "filename is required")
	}
	runtime := b.workspace.runtime
	notes, err := runtime.Service().NotesAt(location)
	if err != nil {
		return nil, "", err
	}
	name := ""
	if location.ProjectID != "" {
		if snapshot, snapErr := runtime.Snapshot(); snapErr == nil {
			if project := snapshot.ProjectByID(location.ProjectID); project != nil {
				name = project.Name
			}
		}
	}
	for _, note := range notes {
		if note.Filename == filename {
			return note, name, nil
		}
	}
	return nil, "", newError(CodeNotFound, "note %s does not exist at that location", filename).with("filename", filename)
}

type noteCommandPayload struct {
	Action      string        `json:"action"`
	Location    noteLocation  `json:"location"`
	Filename    string        `json:"filename,omitempty"`
	Revision    string        `json:"revision,omitempty"`
	Title       string        `json:"title,omitempty"`
	Body        *string       `json:"body,omitempty"`
	Destination *noteLocation `json:"destination,omitempty"`
}

type noteCommandResult struct {
	Note      *noteView `json:"note,omitempty"`
	Committed committed `json:"committed"`
}

// commandNote creates, edits, renames, moves, or deletes one note. Every
// action but create names the note and hands back the revision it read, and
// is refused stale_revision when the note's bytes moved since: another device's
// edit is never overwritten by one aimed at text the phone no longer showed.
func (b *Bridge) commandNote(request Request) (any, error) {
	var payload noteCommandPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	location, refusal := payload.Location.app()
	if refusal != nil {
		return nil, refusal
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	service := runtime.Service()
	if payload.Action == "create" {
		note := &models.Note{Title: strings.TrimSpace(payload.Title)}
		if payload.Body != nil {
			note.Body = canonicalNoteBody(*payload.Body)
		}
		result, err := service.CreateNote(app.NoteCreateRequest{Location: location, Note: note})
		if err != nil {
			return nil, err
		}
		return b.noteOutcome(result)
	}
	if payload.Revision == "" {
		return nil, newError(CodeInvalidRequest, "%s needs revision, the note's revision as read", payload.Action)
	}
	note, _, err := b.findNote(location, payload.Filename)
	if err != nil {
		return nil, err
	}
	if note.RawRevision != payload.Revision {
		return nil, newError(CodeStaleRevision, "note %s changed since it was read", payload.Filename).
			with("filename", payload.Filename).with("current_revision", note.RawRevision)
	}
	var result app.NoteResult
	switch payload.Action {
	case "update":
		if title := strings.TrimSpace(payload.Title); title != "" {
			note.Title = title
		}
		if payload.Body != nil {
			note.Body = canonicalNoteBody(*payload.Body)
		}
		note.ModifiedAt = time.Time{}
		result, err = service.UpdateNote(app.NoteUpdateRequest{Location: location, Note: note})
	case "rename":
		result, err = service.RenameNote(app.NoteRenameRequest{Location: location, Note: note, NewTitle: payload.Title})
	case "move":
		if payload.Destination == nil {
			return nil, newError(CodeInvalidRequest, "move needs destination")
		}
		destination, destinationRefusal := payload.Destination.app()
		if destinationRefusal != nil {
			return nil, destinationRefusal
		}
		result, err = service.MoveNote(app.NoteMoveRequest{Source: location, Destination: destination, Note: note})
	case "delete":
		result, err = service.RemoveNote(app.NoteDeleteRequest{Location: location, Note: note})
		if err != nil {
			return nil, err
		}
		return noteCommandResult{Committed: noteCommitted(result)}, nil
	default:
		return nil, newError(CodeInvalidRequest, "note action %q is not create, update, rename, move, or delete", payload.Action)
	}
	if err != nil {
		return nil, err
	}
	return b.noteOutcome(result)
}

// canonicalNoteBody is a body as the note file reads back: no leading blank
// lines and no trailing whitespace. Storing exactly that makes the body a
// client saved the body it reads, so a saved draft never looks unsaved.
func canonicalNoteBody(body string) string {
	return strings.TrimRight(strings.TrimLeft(body, "\r\n"), " \t\r\n")
}

func (b *Bridge) noteOutcome(result app.NoteResult) (any, error) {
	note, name, err := b.findNote(result.Location, result.Filename)
	if err != nil {
		return nil, err
	}
	view := viewNote(result.Location, name, note, true)
	return noteCommandResult{Note: &view, Committed: noteCommitted(result)}, nil
}

// noteCommitted reports a note commit. A note is its own artifact, so no
// project revision or domain moved.
func noteCommitted(result app.NoteResult) committed {
	return committed{TransactionID: result.TransactionID, Projects: map[string]string{}, Domains: map[string]string{}}
}
