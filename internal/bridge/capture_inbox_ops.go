package bridge

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
)

// CaptureFormat names the envelope a share extension or a shortcut leaves in
// the capture inbox (docs/mobile/capture-inbox.md).
const CaptureFormat = "tt-capture/v1"

// captureEnvelope is one saved capture. An extension writes it and never
// touches the workspace; the app's engine imports it. Either text is capture
// grammar, read as the composer reads it, or title is a literal title with an
// optional note; urls and files land on the first task either creates.
type captureEnvelope struct {
	Format    string        `json:"format"`
	ID        string        `json:"id"`
	Workspace string        `json:"workspace,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	Source    string        `json:"source"`
	Text      string        `json:"text,omitempty"`
	Title     string        `json:"title,omitempty"`
	Note      string        `json:"note,omitempty"`
	URLs      []string      `json:"urls,omitempty"`
	Files     []captureFile `json:"files,omitempty"`
}

// captureFile is one shared file's bytes, beside the envelope.
type captureFile struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
	Digest   string `json:"digest"`
	Size     int64  `json:"size"`
}

// captureStatus is the importer's note beside a capture it could not import,
// which the app and the widgets read to show why it is still pending.
type captureStatus struct {
	State     string      `json:"state"`
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Lines     []lineError `json:"lines,omitempty"`
	Attempted time.Time   `json:"attempted_at"`
}

// captureReceipt is the device's durable record that a capture was imported.
// It outlives the inbox entry, so a capture delivered again after its entry
// was removed — a retried extension, a restored container — imports nothing.
type captureReceipt struct {
	ID         string            `json:"id"`
	ImportedAt time.Time         `json:"imported_at"`
	Tasks      []app.TaskLocator `json:"tasks"`
}

const (
	captureEnvelopeName = "capture.json"
	captureStatusName   = "status.json"
	captureStagingDir   = ".staging"
	captureReceiptsDir  = "capture-receipts"
)

type importCapturesPayload struct {
	CapturesDir string `json:"captures_dir"`
}

type importedCapture struct {
	CaptureID string `json:"capture_id"`
	// Outcome is imported, resumed (an earlier attempt's commit had landed, so
	// only what it left undone ran), or already_imported (a receipt names it, so
	// nothing ran and the entry was removed).
	Outcome string     `json:"outcome"`
	Tasks   []taskView `json:"tasks"`
}

type pendingCapture struct {
	CaptureID string      `json:"capture_id"`
	Reason    string      `json:"reason"`
	Code      string      `json:"code,omitempty"`
	Message   string      `json:"message"`
	Lines     []lineError `json:"lines,omitempty"`
}

type importCapturesResult struct {
	Imported       []importedCapture `json:"imported"`
	Pending        []pendingCapture  `json:"pending"`
	TransactionIDs []string          `json:"transaction_ids"`
}

// importCaptures imports every complete capture in the inbox, oldest first.
//
// Each capture is idempotent across terminations and repeated delivery. Its
// tasks take identities derived from the capture's own, so after a commit
// that landed without its receipt the importer finds them instead of creating
// them again; each file is attached only when those bytes are not already
// there; and a receipt is written before the entry is removed. A capture that
// cannot be imported stays in the inbox with a status naming why, and is
// retried by the next import; one written for another workspace, or still
// being written, is left untouched.
func (b *Bridge) importCaptures(request Request) (any, error) {
	var payload importCapturesPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.CapturesDir == "" || !filepath.IsAbs(payload.CapturesDir) {
		return nil, newError(CodeInvalidRequest, "captures_dir must be an absolute path")
	}
	result := importCapturesResult{Imported: []importedCapture{}, Pending: []pendingCapture{}, TransactionIDs: []string{}}
	b.sweepCaptureStaging(payload.CapturesDir)
	entries, err := os.ReadDir(payload.CapturesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	type found struct {
		dir      string
		envelope captureEnvelope
	}
	var ready []found
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		dir := filepath.Join(payload.CapturesDir, name)
		envelope, refusal := readCaptureEnvelope(dir, name)
		if refusal != nil {
			if refusal.Code == "" {
				// No envelope yet: a writer that has not finished, which only the
				// staging rename may publish. Nothing to say about it.
				result.Pending = append(result.Pending, pendingCapture{CaptureID: name, Reason: "incomplete", Message: refusal.Message})
				continue
			}
			result.Pending = append(result.Pending, b.holdCapture(dir, name, refusal))
			continue
		}
		if envelope.Workspace != "" && envelope.Workspace != b.workspace.id {
			result.Pending = append(result.Pending, pendingCapture{
				CaptureID: name, Reason: "other_workspace",
				Message: fmt.Sprintf("saved for workspace %s, not the open one", envelope.Workspace),
			})
			continue
		}
		ready = append(ready, found{dir: dir, envelope: envelope})
	}
	slices.SortStableFunc(ready, func(a, b found) int {
		if c := a.envelope.CreatedAt.Compare(b.envelope.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.envelope.ID, b.envelope.ID)
	})
	for _, capture := range ready {
		imported, transactions, err := b.importCapture(capture.dir, capture.envelope)
		result.TransactionIDs = append(result.TransactionIDs, transactions...)
		if err != nil {
			var refusal *Error
			if !errors.As(classify(err), &refusal) || refusal.Code == CodeInternal || refusal.Code == CodeIO || errors.Is(err, errCaptureFault) {
				// A failure of the device rather than of the capture stops the
				// import; whatever committed is found again by the next one.
				return nil, err
			}
			result.Pending = append(result.Pending, b.holdCapture(capture.dir, capture.envelope.ID, refusal))
			continue
		}
		result.Imported = append(result.Imported, imported)
	}
	return result, nil
}

// readCaptureEnvelope reads and checks one entry's envelope. A refusal with
// no code means the entry holds no envelope yet.
func readCaptureEnvelope(dir, name string) (captureEnvelope, *Error) {
	data, err := os.ReadFile(filepath.Join(dir, captureEnvelopeName))
	if errors.Is(err, fs.ErrNotExist) {
		return captureEnvelope{}, &Error{Message: "the capture has no envelope yet"}
	}
	if err != nil {
		return captureEnvelope{}, newError(CodeIO, "reading the capture: %v", err)
	}
	var envelope captureEnvelope
	if err := decodeStrict(data, &envelope); err != nil {
		return captureEnvelope{}, newError(CodeValidation, "the capture cannot be read: %v", err).with("field", "capture")
	}
	switch {
	case envelope.Format != CaptureFormat:
		return captureEnvelope{}, newError(CodeValidation, "the capture's format %q is not %s", envelope.Format, CaptureFormat).with("field", "format")
	case envelope.ID != name || !isCanonicalUUID(envelope.ID):
		return captureEnvelope{}, newError(CodeValidation, "the capture's id %q does not name its entry %q", envelope.ID, name).with("field", "id")
	case strings.TrimSpace(envelope.Text) != "" && (envelope.Title != "" || envelope.Note != ""):
		return captureEnvelope{}, newError(CodeValidation, "a capture is either text or a title, not both").with("field", "text")
	}
	return envelope, nil
}

func isCanonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == value
}

// holdCapture leaves a capture in the inbox with its refusal beside it.
func (b *Bridge) holdCapture(dir, id string, refusal *Error) pendingCapture {
	held := pendingCapture{CaptureID: id, Reason: "refused", Code: refusal.Code, Message: refusal.Message}
	if lines, ok := refusal.Details["lines"].([]lineError); ok {
		held.Lines = lines
	}
	status := captureStatus{State: "refused", Code: held.Code, Message: held.Message, Lines: held.Lines, Attempted: b.now().UTC()}
	if data, err := json.MarshalIndent(status, "", "  "); err == nil {
		_ = writeFileAtomically(filepath.Join(dir, captureStatusName), append(data, '\n'))
	}
	return held
}

// errCaptureFault is what a test's fault injection returns, standing in for a
// termination at that point of an import.
var errCaptureFault = errors.New("capture import interrupted")

// importCapture imports one capture and removes its entry.
func (b *Bridge) importCapture(dir string, envelope captureEnvelope) (importedCapture, []string, error) {
	imported := importedCapture{CaptureID: envelope.ID, Outcome: "imported", Tasks: []taskView{}}
	var transactions []string
	if receipt, ok := b.readCaptureReceipt(envelope.ID); ok {
		imported.Outcome = "already_imported"
		for _, locator := range receipt.Tasks {
			if read, err := b.readTask(locator); err == nil {
				imported.Tasks = append(imported.Tasks, read.Task)
			}
		}
		return imported, nil, removeCaptureEntry(dir)
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return imported, nil, err
	}
	files, err := captureFiles(dir, envelope.Files)
	if err != nil {
		return imported, nil, err
	}
	locators, found, err := b.capturedTasks(envelope.ID)
	if err != nil {
		return imported, nil, err
	}
	if found {
		imported.Outcome = "resumed"
	} else {
		reqs, err := b.captureEnvelopeRequests(envelope)
		if err != nil {
			return imported, nil, err
		}
		result, err := b.commitCapture(reqs)
		if err != nil {
			return imported, nil, err
		}
		transactions = append(transactions, result.Committed.TransactionID)
		for _, task := range result.Tasks {
			locators = append(locators, app.TaskLocator{ProjectID: task.ProjectID, TaskID: task.TaskID})
		}
	}
	if err := b.captureFault("created", envelope.ID); err != nil {
		return imported, transactions, err
	}
	for _, file := range files {
		transaction, err := b.attachCapturedFile(locators[0], file)
		if err != nil {
			return imported, transactions, err
		}
		if transaction != "" {
			transactions = append(transactions, transaction)
		}
		if err := b.captureFault("attached", envelope.ID); err != nil {
			return imported, transactions, err
		}
	}
	if err := b.writeCaptureReceipt(captureReceipt{ID: envelope.ID, ImportedAt: b.now().UTC(), Tasks: locators}); err != nil {
		return imported, transactions, err
	}
	if err := b.captureFault("receipted", envelope.ID); err != nil {
		return imported, transactions, err
	}
	for _, locator := range locators {
		read, err := b.readTask(locator)
		if err != nil {
			return imported, transactions, err
		}
		imported.Tasks = append(imported.Tasks, read.Task)
	}
	return imported, transactions, removeCaptureEntry(dir)
}

func (b *Bridge) captureFault(stage, id string) error {
	if b.options.captureFault == nil {
		return nil
	}
	return b.options.captureFault(stage, id)
}

// capturedTaskID is the identity a capture's index-th task takes: a UUID
// derived from the capture's, so whether the capture's commit landed is a
// question the workspace answers.
func capturedTaskID(captureID string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("tt-capture-task/v1\x00%s\x00%d", captureID, index)))
	var id uuid.UUID
	copy(id[:], sum[:16])
	id[6] = (id[6] & 0x0f) | 0x80 // version 8: a custom, name-derived layout
	id[8] = (id[8] & 0x3f) | 0x80 // RFC 9562 variant
	return id.String()
}

// capturedTasks finds the tasks an earlier attempt at this capture created,
// in creation order, by their derived identities.
func (b *Bridge) capturedTasks(captureID string) ([]app.TaskLocator, bool, error) {
	snapshot, err := b.workspace.runtime.Snapshot()
	if err != nil {
		return nil, false, err
	}
	var locators []app.TaskLocator
	for index := 0; ; index++ {
		ref, status := snapshot.LookupTaskID(capturedTaskID(captureID, index))
		if status != app.TaskLookupUnique {
			break
		}
		locators = append(locators, app.TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID})
	}
	return locators, len(locators) > 0, nil
}

// captureEnvelopeRequests reads a capture into the requests the composer's
// own capture would make, each carrying its derived identity.
func (b *Bridge) captureEnvelopeRequests(envelope captureEnvelope) ([]app.CaptureRequest, error) {
	source, err := b.captureSource("")
	if err != nil {
		return nil, err
	}
	var reqs []app.CaptureRequest
	if strings.TrimSpace(envelope.Text) != "" {
		var problems []lineError
		built, refused := b.captureRequests(envelope.Text, source)
		if len(refused) > 0 {
			problems = viewLineErrors(refused)
			return nil, newError(CodeValidation, "%s", lineSummary(refused)).with("lines", problems)
		}
		reqs = built
	} else {
		title := strings.TrimSpace(envelope.Title)
		if title == "" {
			title = captureFallbackTitle(envelope)
		}
		if title == "" {
			return nil, newError(CodeValidation, "the capture holds nothing to create a task from").with("field", "title")
		}
		reqs = []app.CaptureRequest{{
			ProjectID:   source.ID,
			Title:       title,
			Description: strings.TrimSpace(envelope.Note),
			Prepend:     true,
		}}
	}
	for _, raw := range envelope.URLs {
		// A Mail link is not something url.Parse reads (its percent-encoded
		// message ID sits where the host goes), so the capture's own attachment
		// validation decides it instead.
		if app.IsMessageURL(raw) {
			reqs[0].Attachments = append(reqs[0].Attachments, strings.TrimSpace(raw))
			continue
		}
		link, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || link.Scheme == "" || strings.EqualFold(link.Scheme, "file") || (link.Host == "" && link.Opaque == "") {
			return nil, newError(CodeValidation, "%q is not a link a task can keep", raw).with("field", "urls")
		}
		reqs[0].Attachments = append(reqs[0].Attachments, link.String())
	}
	for index := range reqs {
		reqs[index].TaskID = capturedTaskID(envelope.ID, index)
	}
	return reqs, nil
}

// captureFallbackTitle names a capture that came with no words: its first
// link, else its first file.
func captureFallbackTitle(envelope captureEnvelope) string {
	if len(envelope.URLs) > 0 {
		return strings.TrimSpace(envelope.URLs[0])
	}
	if len(envelope.Files) > 0 {
		return strings.TrimSpace(envelope.Files[0].Filename)
	}
	return ""
}
