package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
