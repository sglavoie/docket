package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// The rich-content ops: task and project comments, project and global notes,
// and task attachments. Each one calls the application use case the desktop's
// panels call, so a comment, a note, or an attachment the phone writes is the
// same durable record and commits through the same transaction engine.

// commentView is one comment as a client reads it. A task comment is
// addressed by its index and the text read there; a project comment by its
// stored id and that text.
type commentView struct {
	Index     int       `json:"index"`
	ID        string    `json:"id,omitempty"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func viewTaskComments(task *models.Task) []commentView {
	views := make([]commentView, 0, len(task.Comments))
	for index, comment := range task.Comments {
		if comment == nil {
			continue
		}
		views = append(views, commentView{Index: index, Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	return views
}

func viewProjectComments(project *models.Project) []commentView {
	views := make([]commentView, 0, len(project.Comments))
	for index, comment := range project.Comments {
		if comment == nil {
			continue
		}
		views = append(views, commentView{Index: index, ID: comment.ID, Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	return views
}

// attachmentView is one attachment as a client reads it. Only a file has
// bytes in the workspace; available says whether this device holds them, and
// local_path is where, read-only. A URL or a code reference is kept whole even
// though a phone cannot open a code reference's target.
type attachmentView struct {
	Index         int       `json:"index"`
	Kind          string    `json:"kind"`
	Name          string    `json:"name"`
	URL           string    `json:"url,omitempty"`
	CodePath      string    `json:"code_path,omitempty"`
	Line          int       `json:"line,omitempty"`
	AddedAt       time.Time `json:"added_at"`
	Available     bool      `json:"available"`
	Size          int64     `json:"size,omitempty"`
	LocalPath     string    `json:"local_path,omitempty"`
	WorkspacePath string    `json:"workspace_path,omitempty"`
}

func viewAttachments(dataDir string, project *models.Project, task *models.Task) []attachmentView {
	views := make([]attachmentView, 0, len(task.Attachments))
	for index, attachment := range task.Attachments {
		view := attachmentView{Index: index, Name: attachment.Filename, AddedAt: attachment.AddedAt}
		switch {
		case attachment.IsURL:
			view.Kind = "url"
			view.URL = attachment.Filename
		case attachment.IsCodeRef:
			view.Kind = "code"
			view.CodePath = attachment.OriginalPath
			view.Line = attachment.LineNumber
		default:
			view.Kind = "file"
			path, err := storage.ResolveAttachmentPath(project, task, attachment.Filename)
			if err != nil {
				break
			}
			if relative, relErr := filepath.Rel(dataDir, path); relErr == nil {
				view.WorkspacePath = filepath.ToSlash(relative)
			}
			if info, statErr := os.Stat(path); statErr == nil && info.Mode().IsRegular() {
				view.Available = true
				view.Size = info.Size()
				view.LocalPath = path
			}
		}
		views = append(views, view)
	}
	return views
}

// attachmentDir is the workspace-relative directory a task's attachment bytes
// live in, so a client can match the paths its transport reports unavailable.
func attachmentDir(dataDir string, project *models.Project, task *models.Task) string {
	dir, err := storage.ResolveTaskAttachmentsDir(project, task)
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(dataDir, dir)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relative)
}

// changedSinceRead is the refusal for an index-addressed record that no
// longer holds what the client read there.
func changedSinceRead(field string, index int) *Error {
	return newError(CodeValidation, "%s %d changed since it was read; reload and try again", field, index).
		with("kind", "invalid_input").with("field", field).with("reason", "changed since it was read")
}

// ---- comments ----

type commentPayload struct {
	Task      *locator `json:"task,omitempty"`
	ProjectID string   `json:"project_id,omitempty"`
	Action    string   `json:"action"`
	Text      string   `json:"text,omitempty"`
	Index     *int     `json:"index,omitempty"`
	CommentID string   `json:"comment_id,omitempty"`
	Original  *string  `json:"original,omitempty"`
}

type commentResult struct {
	Task      *taskView     `json:"task,omitempty"`
	ProjectID string        `json:"project_id,omitempty"`
	CommentID string        `json:"comment_id,omitempty"`
	Comments  []commentView `json:"comments"`
	Committed committed     `json:"committed"`
}

// commandComment adds, edits, or deletes one task or project comment. An
// edit or a delete names the comment and carries the text the client read
// there, and is refused when the stored comment no longer holds it, so an
// import or another edit in the meantime never redirects it.
func (b *Bridge) commandComment(request Request) (any, error) {
	var payload commentPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	switch {
	case (payload.Task == nil) == (payload.ProjectID == ""):
		return nil, newError(CodeInvalidRequest, "a comment names exactly one owner: task or project_id")
	case payload.Action != "add" && payload.Action != "edit" && payload.Action != "delete":
		return nil, newError(CodeInvalidRequest, "comment action %q is not add, edit, or delete", payload.Action)
	case payload.Action != "add" && payload.Original == nil:
		return nil, newError(CodeInvalidRequest, "%s needs original, the text read", payload.Action)
	}
	if payload.Task != nil {
		return b.taskComment(payload)
	}
	return b.projectComment(payload)
}

func (b *Bridge) taskComment(payload commentPayload) (any, error) {
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	target := payload.Task.app()
	if payload.Action != "add" {
		if payload.Index == nil {
			return nil, newError(CodeInvalidRequest, "%s needs index", payload.Action)
		}
		read, err := b.readTask(target)
		if err != nil {
			return nil, err
		}
		index := *payload.Index
		comments := read.Comments
		if index < 0 || index >= len(comments) || comments[index].Text != *payload.Original {
			return nil, changedSinceRead("comment", index)
		}
	}
	mutation, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		var result app.CommentResult
		var err error
		switch payload.Action {
		case "add":
			result, err = service.AddComment(target, payload.Text)
		case "edit":
			result, err = service.EditComment(target, *payload.Index, payload.Text)
		default:
			result, err = service.RemoveComment(target, *payload.Index)
		}
		return result.Task, err
	})
	if err != nil {
		return nil, err
	}
	read, err := b.readTask(target)
	if err != nil {
		return nil, err
	}
	return commentResult{Task: &mutation.Task, Comments: read.Comments, Committed: mutation.Committed}, nil
}

func (b *Bridge) projectComment(payload commentPayload) (any, error) {
	service := b.workspace.runtime.Service()
	var result app.ProjectCommentResult
	var err error
	switch payload.Action {
	case "add":
		result, err = service.AddProjectComment(payload.ProjectID, payload.Text)
	default:
		if payload.CommentID == "" {
			return nil, newError(CodeInvalidRequest, "%s needs comment_id", payload.Action)
		}
		target := app.ProjectCommentTarget{ProjectID: payload.ProjectID, CommentID: payload.CommentID, Original: *payload.Original}
		if payload.Action == "edit" {
			result, err = service.EditProjectComment(target, payload.Text)
		} else {
			result, err = service.RemoveProjectComment(target)
		}
	}
	if err != nil {
		return nil, err
	}
	comments, err := b.projectComments(payload.ProjectID)
	if err != nil {
		return nil, err
	}
	return commentResult{
		ProjectID: result.ProjectID,
		CommentID: result.CommentID,
		Comments:  comments,
		Committed: committed{
			TransactionID: result.TransactionID,
			Projects:      map[string]string{result.ProjectID: result.Revision},
			Domains:       map[string]string{},
		},
	}, nil
}

// projectComments reads one available project's comments in stored order:
// creation time, then identity.
func (b *Bridge) projectComments(projectID string) ([]commentView, error) {
	snapshot, err := b.workspace.runtime.Snapshot()
	if err != nil {
		return nil, err
	}
	project := snapshot.ProjectByID(projectID)
	if project == nil {
		return []commentView{}, nil
	}
	return viewProjectComments(project), nil
}

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

// ---- attachments ----

type attachmentPayload struct {
	Task     locator `json:"task"`
	Action   string  `json:"action"`
	Path     string  `json:"path,omitempty"`
	Filename string  `json:"filename,omitempty"`
	Digest   string  `json:"digest,omitempty"`
	URL      string  `json:"url,omitempty"`
	Index    *int    `json:"index,omitempty"`
	Original string  `json:"original,omitempty"`
}

type attachmentResult struct {
	Task        taskView         `json:"task"`
	Attachment  *attachmentView  `json:"attachment,omitempty"`
	Attachments []attachmentView `json:"attachments"`
	// Outcome is added, present (these bytes were already attached under this
	// name, so nothing was committed), or removed.
	Outcome   string    `json:"outcome"`
	Committed committed `json:"committed"`
}

// commandAttachment adds a file or a URL to a task, or removes one.
//
// A file's bytes travel as a path under the device's runtime directory, never
// inline and never as a provider URL: the client copies the picked item there
// while its grant is valid, names the copy's digest, and the engine reads it
// into one transaction with the task's metadata, so a failure commits neither.
// Repeating an import whose bytes are already attached under that name answers
// present and commits nothing, which is what lets a client resume an import a
// termination interrupted without knowing whether its commit landed.
func (b *Bridge) commandAttachment(request Request) (any, error) {
	var payload attachmentPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	target := payload.Task.app()
	switch payload.Action {
	case "add_file":
		return b.importAttachment(target, payload)
	case "add_url":
		mutation, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
			return service.AttachURL(target, payload.URL)
		})
		if err != nil {
			return nil, err
		}
		return b.attachmentOutcome(target, mutation, "added", -1)
	case "remove":
		if payload.Index == nil {
			return nil, newError(CodeInvalidRequest, "remove needs index and original, the name read")
		}
		index := *payload.Index
		ref, err := b.lookupTask(target)
		if err != nil {
			return nil, err
		}
		if index < 0 || index >= len(ref.Task.Attachments) || ref.Task.Attachments[index].Filename != payload.Original {
			return nil, changedSinceRead("attachment", index)
		}
		mutation, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
			result, removeErr := service.RemoveAttachment(target, index, true)
			return result.Task, removeErr
		})
		if err != nil {
			return nil, err
		}
		return b.attachmentOutcome(target, mutation, "removed", -1)
	}
	return nil, newError(CodeInvalidRequest, "attachment action %q is not add_file, add_url, or remove", payload.Action)
}

func (b *Bridge) importAttachment(target app.TaskLocator, payload attachmentPayload) (any, error) {
	staged, err := b.stagedPath(payload.Path)
	if err != nil {
		return nil, err
	}
	if payload.Digest == "" {
		return nil, newError(CodeInvalidRequest, "add_file needs digest, the staged copy's sha256")
	}
	digest, err := fileDigest(staged)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, newError(CodeNotFound, "the staged copy %s does not exist", payload.Path).with("path", payload.Path)
	}
	if err != nil {
		return nil, err
	}
	if digest != payload.Digest {
		return nil, newError(CodeValidation, "the staged copy is incomplete: its digest is %s, not %s", digest, payload.Digest).
			with("kind", "invalid_input").with("field", "attachment").with("reason", "the staged copy is incomplete")
	}
	name := attachmentFilename(payload.Filename, staged)
	ref, err := b.lookupTask(target)
	if err != nil {
		return nil, err
	}
	if index, present := attachedBytes(ref.Project, ref.Task, name, digest); present {
		read, readErr := b.readTask(target)
		if readErr != nil {
			return nil, readErr
		}
		return b.attachmentOutcome(target, mutationResult{Task: read.Task, Committed: committed{Projects: map[string]string{}, Domains: map[string]string{}}}, "present", index)
	}
	name = freeAttachmentName(ref.Project, ref.Task, name)
	mutation, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		result, importErr := service.ImportAttachment(target, staged, name)
		return result.Task, importErr
	})
	if err != nil {
		return nil, err
	}
	return b.attachmentOutcome(target, mutation, "added", -1)
}

// attachmentOutcome reports a task's attachments after an attachment action.
// The attachment it names is the one at index, or the last when index is
// negative and the action added one.
func (b *Bridge) attachmentOutcome(target app.TaskLocator, mutation mutationResult, outcome string, index int) (any, error) {
	read, err := b.readTask(target)
	if err != nil {
		return nil, err
	}
	result := attachmentResult{Task: mutation.Task, Attachments: read.Attachments, Outcome: outcome, Committed: mutation.Committed}
	if index < 0 && outcome == "added" {
		index = len(read.Attachments) - 1
	}
	if index >= 0 && index < len(read.Attachments) {
		attachment := read.Attachments[index]
		result.Attachment = &attachment
	}
	return result, nil
}

func (b *Bridge) lookupTask(target app.TaskLocator) (app.TaskRef, error) {
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return app.TaskRef{}, err
	}
	snapshot, err := runtime.Snapshot()
	if err != nil {
		return app.TaskRef{}, err
	}
	ref, ok := snapshot.LookupLocator(target)
	if !ok {
		return app.TaskRef{}, newError(CodeNotFound, "task %s:%s does not resolve", target.ProjectID, target.TaskID).
			with("project_id", target.ProjectID).with("task_id", target.TaskID)
	}
	return ref, nil
}

// stagedPath admits a staged copy only from inside the device's runtime
// directory, judged after symlinks resolve, so a request can never hand the
// engine a path into the workspace, another app, or a provider's storage.
func (b *Bridge) stagedPath(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", newError(CodeInvalidRequest, "path must be an absolute path under runtime_dir")
	}
	root, err := filepath.EvalSymlinks(b.workspace.runtimeDir)
	if err != nil {
		return "", newError(CodeInvalidRequest, "runtime_dir %s cannot hold a staged copy: %v", b.workspace.runtimeDir, err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return "", newError(CodeNotFound, "the staged copy %s does not exist", path).with("path", path)
	}
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", newError(CodeInvalidRequest, "path %s is not under runtime_dir", path)
	}
	return resolved, nil
}

// fileDigest is a file's content address, in the form the exchange names
// blobs: "sha256:" and the hex digest.
func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// attachmentFilename makes a picked item's name storable: one path component
// with no attachment delimiter and no control character, never hidden.
func attachmentFilename(requested, staged string) string {
	name := strings.TrimSpace(requested)
	if name == "" {
		name = filepath.Base(staged)
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		switch {
		case r == ';' || r == '|' || r == '/' || r == ':' || unicode.IsControl(r):
			return '-'
		}
		return r
	}, name)
	name = strings.TrimLeft(strings.TrimSpace(name), ".")
	if name == "" {
		name = "attachment"
	}
	return name
}

// attachedBytes finds a file attachment named name, or a numbered variant of
// it, whose stored bytes have the given digest.
func attachedBytes(project *models.Project, task *models.Task, name, digest string) (int, bool) {
	stem, ext := splitName(name)
	for index, attachment := range task.Attachments {
		if attachment.IsURL || attachment.IsCodeRef {
			continue
		}
		if attachment.Filename != name && !isNumberedVariant(attachment.Filename, stem, ext) {
			continue
		}
		path, err := storage.ResolveAttachmentPath(project, task, attachment.Filename)
		if err != nil {
			continue
		}
		if stored, err := fileDigest(path); err == nil && stored == digest {
			return index, true
		}
	}
	return -1, false
}

// freeAttachmentName is name, or name with a number before its extension,
// whichever neither the task's attachments nor its directory already use.
func freeAttachmentName(project *models.Project, task *models.Task, name string) string {
	taken := func(candidate string) bool {
		for _, attachment := range task.Attachments {
			if !attachment.IsURL && !attachment.IsCodeRef && attachment.Filename == candidate {
				return true
			}
		}
		return storage.AttachmentExists(project, task, candidate)
	}
	if !taken(name) {
		return name
	}
	stem, ext := splitName(name)
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, n, ext)
		if !taken(candidate) {
			return candidate
		}
	}
}

func splitName(name string) (string, string) {
	ext := filepath.Ext(name)
	if ext == name {
		return name, ""
	}
	return strings.TrimSuffix(name, ext), ext
}

func isNumberedVariant(candidate, stem, ext string) bool {
	if !strings.HasPrefix(candidate, stem+"-") || !strings.HasSuffix(candidate, ext) {
		return false
	}
	number := strings.TrimSuffix(strings.TrimPrefix(candidate, stem+"-"), ext)
	if number == "" {
		return false
	}
	for _, r := range number {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
