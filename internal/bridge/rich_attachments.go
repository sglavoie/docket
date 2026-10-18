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
	"strings"
	"time"
	"unicode"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
