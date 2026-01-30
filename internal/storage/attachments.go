package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
)

const attachmentsDirName = ".attachments"

// GetAttachmentsDir returns the path to the attachments directory for a project.
func GetAttachmentsDir(project *models.Project) string {
	return filepath.Join(project.Path, attachmentsDirName)
}

// GetTaskAttachmentsDir returns the path to the attachments directory for a specific task.
// Uses the task's UUID as the directory name for stability (short IDs can be reused after archive).
func GetTaskAttachmentsDir(project *models.Project, task *models.Task) string {
	return filepath.Join(GetAttachmentsDir(project), task.ID)
}

// GetAttachmentPath returns the full path to a specific attachment file.
func GetAttachmentPath(project *models.Project, task *models.Task, filename string) string {
	return filepath.Join(GetTaskAttachmentsDir(project, task), filename)
}

// EnsureTaskAttachmentsDir creates the attachments directory for a task if it doesn't exist.
func EnsureTaskAttachmentsDir(project *models.Project, task *models.Task) error {
	dir := GetTaskAttachmentsDir(project, task)
	return os.MkdirAll(dir, DirPermissions)
}

// CopyAttachment copies a file to the task's attachments directory.
// Returns the base filename (without path) that was used.
func CopyAttachment(project *models.Project, task *models.Task, sourcePath string) (filename string, err error) {
	if err = EnsureTaskAttachmentsDir(project, task); err != nil {
		return "", fmt.Errorf("failed to create attachments directory: %w", err)
	}

	// Get the base filename
	filename = filepath.Base(sourcePath)
	destPath := GetAttachmentPath(project, task, filename)

	// Check if file already exists
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
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
	attachmentPath := GetAttachmentPath(project, task, filename)
	if err := os.Remove(attachmentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove attachment file: %w", err)
	}
	return nil
}

// CleanupTaskAttachments removes all attachments for a task and the task's attachment directory.
// This is called when a task is deleted.
func CleanupTaskAttachments(project *models.Project, task *models.Task) error {
	dir := GetTaskAttachmentsDir(project, task)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove attachments directory: %w", err)
	}
	return nil
}

// AttachmentExists checks if an attachment file exists on disk.
func AttachmentExists(project *models.Project, task *models.Task, filename string) bool {
	path := GetAttachmentPath(project, task, filename)
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetAttachmentSize returns the size of an attachment file in bytes.
// Returns 0 if the file doesn't exist or is a URL.
func GetAttachmentSize(project *models.Project, task *models.Task, attachment *models.Attachment) int64 {
	if attachment.IsURL {
		return 0
	}
	path := GetAttachmentPath(project, task, attachment.Filename)
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
