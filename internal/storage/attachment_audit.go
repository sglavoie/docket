package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/docket/internal/models"
)

// AttachmentSkew is one disagreement between a project's stored attachment
// metadata and the bytes under its .attachments tree.
//
// The two halves of an attachment travel separately: the metadata entry rides
// inside tasks.md while the bytes sit in a per-task directory beside it. A sync
// client that copies one file before the other, or resolves a conflict on only
// one of them, leaves the pair disagreeing in whichever direction it happened to
// win. Both directions are worth naming and neither is repaired here: bytes with
// no metadata may be the only surviving copy of an attachment, and metadata with
// no bytes may be waiting for a download that has not finished.
type AttachmentSkew struct {
	// ShortID is the owning task's, or zero for a finding about a directory no
	// task claims. Detail always names the task in full, so nothing else about the
	// task is carried here; ShortID exists to sort the findings.
	ShortID int
	Detail  string
	// Orphaned distinguishes the two directions. True means bytes exist that no
	// metadata entry names, which may be the only surviving copy of an attachment
	// and so is never a thing to delete. False means metadata names bytes that are
	// not there, which is dead to every reader.
	Orphaned bool
}

// InspectProjectAttachments compares a loaded project's attachment metadata
// against its .attachments tree and returns every disagreement, sorted so the
// output is stable across runs.
//
// URL and code-reference attachments are skipped: neither stores bytes, so
// neither can skew. A project with no .attachments directory is clean, not
// missing something — the directory is created on first attachment.
func InspectProjectAttachments(project *models.Project) ([]AttachmentSkew, error) {
	if project == nil {
		return nil, nil
	}
	var findings []AttachmentSkew

	// Expected directory name per task, so an on-disk directory can be traced
	// back to the task that owns it without reversing the hash.
	owners := make(map[string]*models.Task, len(project.Tasks))
	for _, task := range project.Tasks {
		if task == nil || task.ID == "" {
			continue
		}
		owners[taskAttachmentsDirName(task.ID)] = task
	}

	for _, task := range project.Tasks {
		if task == nil || task.ID == "" {
			continue
		}
		skews, err := inspectTaskAttachments(project, task)
		if err != nil {
			return nil, err
		}
		findings = append(findings, skews...)
	}

	orphans, err := inspectOrphanedAttachmentDirs(project, owners)
	if err != nil {
		return nil, err
	}
	findings = append(findings, orphans...)

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].ShortID != findings[j].ShortID {
			return findings[i].ShortID < findings[j].ShortID
		}
		return findings[i].Detail < findings[j].Detail
	})
	return findings, nil
}

// inspectTaskAttachments reports metadata whose bytes are absent and bytes no
// metadata entry names, for one task.
func inspectTaskAttachments(project *models.Project, task *models.Task) ([]AttachmentSkew, error) {
	named := make(map[string]bool, len(task.Attachments))
	var findings []AttachmentSkew
	for _, attachment := range task.Attachments {
		if attachment.IsURL || attachment.IsCodeRef {
			continue
		}
		named[attachment.Filename] = true
		path, err := resolveAttachmentPath(project, task, attachment.Filename)
		if err != nil {
			findings = append(findings, AttachmentSkew{
				ShortID: task.ShortID,
				Detail:  fmt.Sprintf("attachment %q on task #%d %q has an unusable path: %v", attachment.Filename, task.ShortID, task.Title, err),
			})
			continue
		}
		info, statErr := os.Lstat(path)
		switch {
		case os.IsNotExist(statErr):
			findings = append(findings, AttachmentSkew{
				ShortID: task.ShortID,
				Detail:  fmt.Sprintf("attachment %q on task #%d %q has no file on disk", attachment.Filename, task.ShortID, task.Title),
			})
		case statErr != nil:
			return nil, fmt.Errorf("inspect attachment %q: %w", path, statErr)
		case !info.Mode().IsRegular():
			findings = append(findings, AttachmentSkew{
				ShortID: task.ShortID,
				Detail:  fmt.Sprintf("attachment %q on task #%d %q is not a regular file", attachment.Filename, task.ShortID, task.Title),
			})
		}
	}

	// A task whose attachments directory cannot even be addressed has no tree to
	// sweep for orphans, but the failure is reported rather than swallowed: it means
	// a stored task ID or project path no resolver accepts, which is a finding in its
	// own right and not the same thing as a clean task.
	dir, dirErr := resolveTaskAttachmentsDir(project, task)
	if dirErr != nil {
		findings = append(findings, AttachmentSkew{
			ShortID: task.ShortID,
			Detail:  fmt.Sprintf("task #%d %q has an unusable attachments directory: %v", task.ShortID, task.Title, dirErr),
		})
		return findings, nil
	}
	entries, readErr := os.ReadDir(dir)
	if os.IsNotExist(readErr) {
		return findings, nil
	}
	if readErr != nil {
		return nil, fmt.Errorf("read task attachments directory %q: %w", dir, readErr)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			findings = append(findings, AttachmentSkew{
				ShortID: task.ShortID,
				Detail:  fmt.Sprintf("unexpected nested directory %q under task #%d %q attachments", entry.Name(), task.ShortID, task.Title),
			})
			continue
		}
		if named[entry.Name()] {
			continue
		}
		findings = append(findings, AttachmentSkew{
			ShortID:  task.ShortID,
			Detail:   fmt.Sprintf("orphaned attachment file %q under task #%d %q; no metadata entry names it", entry.Name(), task.ShortID, task.Title),
			Orphaned: true,
		})
	}
	return findings, nil
}

// inspectOrphanedAttachmentDirs reports directories under .attachments that no
// retained task claims. The usual cause is a task deleted on another machine
// whose tasks.md arrived before the directory removal did, or a project whose
// attachment tree was restored without its tasks.md.
func inspectOrphanedAttachmentDirs(project *models.Project, owners map[string]*models.Task) ([]AttachmentSkew, error) {
	root := filepath.Join(project.Path, attachmentsDirName)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project attachments directory %q: %w", root, err)
	}
	var findings []AttachmentSkew
	for _, entry := range entries {
		if !entry.IsDir() {
			findings = append(findings, AttachmentSkew{
				Detail: fmt.Sprintf("unexpected file %q directly under %s", entry.Name(), attachmentsDirName),
			})
			continue
		}
		if owners[entry.Name()] != nil {
			continue
		}
		count, err := countAttachmentDirFiles(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		findings = append(findings, AttachmentSkew{
			Detail: fmt.Sprintf(
				"orphaned attachment directory %q holds %s; no retained task owns it",
				entry.Name(),
				pluralizeFiles(count),
			),
			Orphaned: true,
		})
	}
	return findings, nil
}

func countAttachmentDirFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read attachment directory %q: %w", dir, err)
	}
	return len(entries), nil
}

func pluralizeFiles(count int) string {
	if count == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", count)
}
