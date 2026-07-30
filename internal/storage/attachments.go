package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

const attachmentsDirName = ".attachments"

// getAttachmentsDir returns the path to the attachments directory for a project.
func getAttachmentsDir(project *models.Project) string {
	return filepath.Join(project.Path, attachmentsDirName)
}

// taskAttachmentsDirName returns the on-disk directory name for a task's
// attachments: the lowercase hex SHA-256 of the task's stable UUID. Hashing
// yields a fixed 64-character [0-9a-f] name that is always a safe single path
// component, so a crafted or malformed task ID can never shape the directory
// layout or escape the project's .attachments tree. Short IDs are avoided
// because they can be reused after archive; the UUID is stable.
func taskAttachmentsDirName(taskID string) string {
	sum := sha256.Sum256([]byte(taskID))
	return hex.EncodeToString(sum[:])
}

// GetTaskAttachmentsDir returns the lexical path to a task attachment
// directory. Callers that open or mutate the path must use
// ResolveTaskAttachmentsDir so symlink containment is verified.
func GetTaskAttachmentsDir(project *models.Project, task *models.Task) string {
	return filepath.Join(getAttachmentsDir(project), taskAttachmentsDirName(task.ID))
}

// GetAttachmentPath returns the lexical path to an attachment file. It remains
// for trusted test/setup callers; code that opens or mutates the path must use
// ResolveAttachmentPath so filename and symlink containment are verified.
func GetAttachmentPath(project *models.Project, task *models.Task, filename string) string {
	return filepath.Join(GetTaskAttachmentsDir(project, task), filename)
}

// ResolveTaskAttachmentsDir returns a contained attachment directory suitable
// for opening or mutation.
func ResolveTaskAttachmentsDir(project *models.Project, task *models.Task) (string, error) {
	return resolveTaskAttachmentsDir(project, task)
}

// ResolveAttachmentPath returns a contained attachment file path suitable for
// opening or mutation.
func ResolveAttachmentPath(project *models.Project, task *models.Task, filename string) (string, error) {
	return resolveAttachmentPath(project, task, filename)
}

// resolveTaskAttachmentsDir is the single resolver shared by every attachment
// read/write path (copy, remove, cleanup, move, existence, size). It addresses
// the task's directory by sha256(task.ID) and re-verifies, via SafeJoin against
// the project root, that the result stays inside the project's .attachments
// tree even when a parent component is a symlink that escapes the project.
func resolveTaskAttachmentsDir(project *models.Project, task *models.Task) (string, error) {
	if err := validateAttachmentTask(task); err != nil {
		return "", err
	}
	rel := filepath.Join(attachmentsDirName, taskAttachmentsDirName(task.ID))
	dir, err := SafeJoin(project.Path, rel)
	if err != nil {
		return "", fmt.Errorf("invalid attachments directory for task %q: %w", task.ID, err)
	}
	return dir, nil
}

// resolveAttachmentPath resolves the contained path to a single attachment
// file, validating the filename and re-verifying containment after the final
// join so a crafted filename cannot escape the task's attachment directory.
func resolveAttachmentPath(project *models.Project, task *models.Task, filename string) (string, error) {
	if err := validateAttachmentFilename(filename); err != nil {
		return "", err
	}
	if err := validateAttachmentTask(task); err != nil {
		return "", err
	}
	rel := filepath.Join(attachmentsDirName, taskAttachmentsDirName(task.ID), filename)
	path, err := SafeJoin(project.Path, rel)
	if err != nil {
		return "", fmt.Errorf("invalid attachment path %q: %w", filename, err)
	}
	return path, nil
}

func validateAttachmentTask(task *models.Task) error {
	if task == nil || task.ID == "" {
		return fmt.Errorf("task ID cannot be empty for attachments")
	}
	// The task ID is hashed (see taskAttachmentsDirName) before it becomes a
	// directory name, so a malformed or crafted ID can no longer influence the
	// on-disk path. Containment is enforced by the hash plus a SafeJoin check
	// in the resolver; no path-component validation of the raw ID is required.
	return nil
}

func validateAttachmentFilename(filename string) error {
	if err := SafeChildName(filename); err != nil {
		return fmt.Errorf("invalid attachment filename %q: %w", filename, err)
	}
	return nil
}

// LegacyAttachmentDir describes a pre-hash attachment directory found during
// migration: a directory under .attachments whose name is a raw task ID rather
// than a sha256 address.
type LegacyAttachmentDir struct {
	// TaskID is the raw directory name, which for legacy layouts is the task's
	// UUID (or short ID). Callers derive the new hashed home from it.
	TaskID string
	// Path is the absolute path of the legacy directory.
	Path string
}

// isHashedAttachmentDirName reports whether name is a sha256 attachment address
// (a 64-character lowercase hex string). Any other directory name under
// .attachments is treated as a legacy raw-ID directory.
func isHashedAttachmentDirName(name string) bool {
	if len(name) != sha256.Size*2 {
		return false
	}
	for _, c := range name {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// DiscoverLegacyAttachmentDirs returns the raw-ID attachment directories under a
// project's .attachments tree that predate sha256 addressing. It exists solely
// for the eventual live migration (see the program plan); no operational
// read/write path calls it. A missing .attachments directory yields no entries
// and no error.
func DiscoverLegacyAttachmentDirs(project *models.Project) ([]LegacyAttachmentDir, error) {
	base := getAttachmentsDir(project)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read attachments directory: %w", err)
	}
	var legacy []LegacyAttachmentDir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isHashedAttachmentDirName(name) {
			continue
		}
		legacy = append(legacy, LegacyAttachmentDir{
			TaskID: name,
			Path:   filepath.Join(base, name),
		})
	}
	return legacy, nil
}

// EnsureTaskAttachmentsDir creates the attachments directory for a task if it doesn't exist.
func EnsureTaskAttachmentsDir(project *models.Project, task *models.Task) error {
	dir, err := resolveTaskAttachmentsDir(project, task)
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, DirPermissions)
}

// CopyAttachment copies a file to the task's attachments directory.
// Returns the base filename (without path) that was used.
func CopyAttachment(project *models.Project, task *models.Task, sourcePath string) (filename string, err error) {
	if err = EnsureTaskAttachmentsDir(project, task); err != nil {
		return "", fmt.Errorf("failed to create attachments directory: %w", err)
	}

	// Get the base filename and resolve its contained destination.
	filename = filepath.Base(sourcePath)
	destPath, err := resolveAttachmentPath(project, task, filename)
	if err != nil {
		return "", err
	}

	// Check if file already exists
	if _, statErr := os.Stat(destPath); statErr == nil {
		return "", fmt.Errorf("attachment '%s' already exists", filename)
	}

	// Open source file
	var src *os.File
	src, err = os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = src.Close() }()

	// Create destination file
	var dst *os.File
	dst, err = os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close destination file: %w", cerr)
		}
		// Clean up partially-written file on any error so that a retry
		// is not blocked by the "already exists" check above.
		if err != nil {
			_ = os.Remove(destPath)
		}
	}()

	// Copy contents
	if _, err = io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	return filename, nil
}

// RemoveAttachmentFile removes an attachment file from disk.
// This does not modify the task's Attachments slice.
func RemoveAttachmentFile(project *models.Project, task *models.Task, filename string) error {
	attachmentPath, err := resolveAttachmentPath(project, task, filename)
	if err != nil {
		return err
	}
	if err := os.Remove(attachmentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove attachment file: %w", err)
	}
	return nil
}

// MoveTaskAttachments moves a task's attachment directory from one project to
// another. Used by `tt task move-to`: the task UUID is preserved across the
// move, so the destination project must take ownership of the on-disk files
// that the task's metadata still references. No-op when the source has no
// attachment directory (URL-only or code-ref-only tasks).
//
// Guards against an empty task ID for the same reason as CleanupTaskAttachments.
func MoveTaskAttachments(src, dest *models.Project, task *models.Task) error {
	if task == nil || task.ID == "" {
		return nil
	}
	srcDir, err := resolveTaskAttachmentsDir(src, task)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(srcDir); os.IsNotExist(statErr) {
		return nil
	} else if statErr != nil {
		return fmt.Errorf("failed to stat source attachments directory: %w", statErr)
	}
	destDir, err := resolveTaskAttachmentsDir(dest, task)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destDir), DirPermissions); err != nil {
		return fmt.Errorf("failed to create destination attachments parent: %w", err)
	}
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("destination task attachments directory already exists: %s", destDir)
	}
	if err := os.Rename(srcDir, destDir); err != nil {
		return fmt.Errorf("failed to move task attachments: %w", err)
	}
	return nil
}

// CleanupTaskAttachments removes all attachments for a task and the task's attachment directory.
// This is called when a task is deleted.
//
// Guards against an empty task ID: without this check the joined path would
// collapse to the project's parent .attachments directory and os.RemoveAll
// would wipe attachments for every other task in the project.
func CleanupTaskAttachments(project *models.Project, task *models.Task) error {
	if task == nil || task.ID == "" {
		return nil
	}
	dir, err := resolveTaskAttachmentsDir(project, task)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove attachments directory: %w", err)
	}
	return nil
}

// listTaskAttachmentFiles returns the absolute paths of the regular files directly
// inside dir. Attachment directories are flat by construction — CopyAttachment
// writes files directly under the task directory — so a nested directory is
// unexpected and reported as an error rather than silently skipped, since the
// transaction journal's delete primitive is a non-recursive removal. exists is
// false when the directory itself is absent, letting callers treat a delete or move
// of a task with no attachments as a no-op.
func listTaskAttachmentFiles(dir string) (files []string, exists bool, err error) {
	entries, readErr := os.ReadDir(dir)
	if os.IsNotExist(readErr) {
		return nil, false, nil
	}
	if readErr != nil {
		return nil, false, fmt.Errorf("failed to read task attachments directory: %w", readErr)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, true, fmt.Errorf("unexpected nested directory in attachments: %s", filepath.Join(dir, entry.Name()))
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	return files, true, nil
}

// AttachmentExists checks if an attachment file exists on disk.
func AttachmentExists(project *models.Project, task *models.Task, filename string) bool {
	path, err := resolveAttachmentPath(project, task, filename)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// GetAttachmentSize returns the size of an attachment file in bytes.
// Returns 0 if the file doesn't exist or is a URL.
func GetAttachmentSize(project *models.Project, task *models.Task, attachment *models.Attachment) int64 {
	if attachment == nil || attachment.IsURL {
		return 0
	}
	path, err := resolveAttachmentPath(project, task, attachment.Filename)
	if err != nil {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// FormatFileSize returns a human-readable file size string.
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ParseCodeReference detects if input is a code reference in file:line format.
// Returns the resolved path, line number, and whether it's a valid code reference.
// A valid code reference has a file path followed by :N where N is a line number.
// The file is validated to exist (after expanding ~ and resolving the path).
// If the file doesn't exist, returns the original input with line 0 and isCodeRef=false.
func ParseCodeReference(input string) (path string, line int, isCodeRef bool) {
	// Find the last colon followed by digits
	lastColonIdx := -1
	for i := len(input) - 1; i >= 0; i-- {
		if input[i] == ':' {
			// Check if everything after the colon is digits
			suffix := input[i+1:]
			if len(suffix) > 0 {
				allDigits := true
				for _, c := range suffix {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits {
					lastColonIdx = i
					break
				}
			}
		}
	}

	if lastColonIdx == -1 {
		return input, 0, false
	}

	// Extract path and line number
	pathPart := input[:lastColonIdx]
	linePart := input[lastColonIdx+1:]

	lineNum, err := strconv.Atoi(linePart)
	if err != nil || lineNum < 1 {
		return input, 0, false
	}

	// Expand ~ to home directory
	expandedPath := pathPart
	if strings.HasPrefix(expandedPath, "~/") {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return input, 0, false
		}
		expandedPath = filepath.Join(home, expandedPath[2:])
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(expandedPath)
	if err != nil {
		return input, 0, false
	}

	// Check if file exists - this is soft validation (we still allow the reference)
	if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
		// File doesn't exist, but it's still a valid code reference format
		return absPath, lineNum, true
	}

	return absPath, lineNum, true
}

// CodeReferenceFileExists checks if the file for a code reference exists.
func CodeReferenceFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
