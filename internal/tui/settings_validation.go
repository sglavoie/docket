package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func (m *SettingsModel) validateEditor() error {
	editor := strings.TrimSpace(m.editorInput.Value())

	if editor == "" {
		return fmt.Errorf("editor is required")
	}

	// Check if it's an absolute path
	if strings.HasPrefix(editor, "/") {
		info, err := os.Stat(editor)
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", editor)
		}
		if err != nil {
			return fmt.Errorf("cannot access file: %s", editor)
		}
		if info.IsDir() {
			return fmt.Errorf("path is a directory, not an executable: %s", editor)
		}
		// Check if executable
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("file is not executable: %s", editor)
		}
		return nil
	}

	// Otherwise, check if command exists in PATH
	_, err := exec.LookPath(editor)
	if err != nil {
		return fmt.Errorf("command not found in PATH: %s", editor)
	}

	return nil
}

func (m *SettingsModel) validateContext() error {
	context := strings.TrimSpace(m.contextInput.Value())
	if context == "" {
		return fmt.Errorf("default context is required")
	}
	return nil
}

func (m *SettingsModel) validateUndoDepth() error {
	value := strings.TrimSpace(m.undoDepthInput.Value())
	if value == "" {
		return fmt.Errorf("undo history depth is required")
	}

	depth, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a number")
	}

	if depth < 0 {
		return fmt.Errorf("must be 0 or greater")
	}

	if depth > 1000 {
		return fmt.Errorf("maximum is 1000")
	}

	return nil
}

func (m *SettingsModel) validateReviewSettings() error {
	// Validate Due Soon Days
	dueSoonStr := strings.TrimSpace(m.dueSoonInput.Value())
	if dueSoonStr == "" {
		return fmt.Errorf("due soon days is required")
	}
	dueSoon, err := strconv.Atoi(dueSoonStr)
	if err != nil {
		return fmt.Errorf("due soon days must be a number")
	}
	if dueSoon < 0 {
		return fmt.Errorf("due soon days must be 0 or greater")
	}

	// Validate Future Days
	futureStr := strings.TrimSpace(m.futureInput.Value())
	if futureStr == "" {
		return fmt.Errorf("future days is required")
	}
	futureDays, err := strconv.Atoi(futureStr)
	if err != nil {
		return fmt.Errorf("future days must be a number")
	}
	if futureDays < 0 {
		return fmt.Errorf("future days must be 0 or greater")
	}

	// Future must be > Due Soon when both are non-zero
	if dueSoon > 0 && futureDays > 0 && futureDays <= dueSoon {
		return fmt.Errorf("future must be greater than due soon")
	}

	// Validate Old Days
	oldDaysStr := strings.TrimSpace(m.oldDaysInput.Value())
	if oldDaysStr == "" {
		return fmt.Errorf("old days is required")
	}
	oldDays, err := strconv.Atoi(oldDaysStr)
	if err != nil {
		return fmt.Errorf("old days must be a number")
	}
	if oldDays < 0 {
		return fmt.Errorf("old days must be 0 or greater")
	}

	// Validate Completed Hours
	completedHoursStr := strings.TrimSpace(m.completedHoursInput.Value())
	if completedHoursStr == "" {
		return fmt.Errorf("completed hours is required")
	}
	completedHours, err := strconv.Atoi(completedHoursStr)
	if err != nil {
		return fmt.Errorf("completed hours must be a number")
	}
	if completedHours < 0 {
		return fmt.Errorf("completed hours must be 0 or greater")
	}

	return nil
}

func (m *SettingsModel) validateRecentActivityLimit() error {
	value := strings.TrimSpace(m.recentActivityInput.Value())
	if value == "" {
		return fmt.Errorf("recent activity limit is required")
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if limit < 0 {
		return fmt.Errorf("must be 0 or greater")
	}
	return nil
}

func (m *SettingsModel) validateAttachmentEditor() error {
	editor := strings.TrimSpace(m.attachmentEditorInput.Value())
	if editor == "" {
		return nil // Empty is valid (uses OS default)
	}
	return m.validateEditorCommand(editor)
}

func (m *SettingsModel) validateCodeEditor() error {
	editor := strings.TrimSpace(m.codeEditorInput.Value())
	if editor == "" {
		return nil // Empty is valid (defaults to "code")
	}
	return m.validateEditorCommand(editor)
}

func (m *SettingsModel) validateMaxPinnedBarItems() error {
	value := strings.TrimSpace(m.maxPinnedInput.Value())
	if value == "" {
		return fmt.Errorf("bar limit is required")
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if n < 0 || n > 20 {
		return fmt.Errorf("must be 0-20")
	}
	return nil
}

func (m *SettingsModel) validateKanbanSettings() error {
	colsStr := strings.TrimSpace(m.kanbanMaxColumnsInput.Value())
	if colsStr == "" {
		return fmt.Errorf("max columns is required")
	}
	cols, err := strconv.Atoi(colsStr)
	if err != nil {
		return fmt.Errorf("max columns must be a number")
	}
	if cols < 1 || cols > 20 {
		return fmt.Errorf("max columns must be 1-20")
	}

	rowsStr := strings.TrimSpace(m.kanbanMaxSwimlaneInput.Value())
	if rowsStr == "" {
		return fmt.Errorf("max swimlane rows is required")
	}
	rows, err := strconv.Atoi(rowsStr)
	if err != nil {
		return fmt.Errorf("max swimlane rows must be a number")
	}
	if rows < 1 || rows > 20 {
		return fmt.Errorf("max swimlane rows must be 1-20")
	}

	return nil
}

func (m *SettingsModel) validateAlertSettings() error {
	// Daily review hour (0-23)
	hourStr := strings.TrimSpace(m.alertDailyReviewHourInput.Value())
	if hourStr != "" {
		hour, err := strconv.Atoi(hourStr)
		if err != nil {
			return fmt.Errorf("review hour must be a number")
		}
		if hour < 0 || hour > 23 {
			return fmt.Errorf("review hour must be 0-23")
		}
	}

	// Due soon days (positive)
	if v := strings.TrimSpace(m.alertDueSoonDaysInput.Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("due soon days must be a number")
		}
		if n < 1 {
			return fmt.Errorf("due soon days must be at least 1")
		}
	}

	// Stale days (positive)
	if v := strings.TrimSpace(m.alertStaleDaysInput.Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("stale days must be a number")
		}
		if n < 1 {
			return fmt.Errorf("stale days must be at least 1")
		}
	}

	// Inbox threshold (positive)
	if v := strings.TrimSpace(m.alertInboxThresholdInput.Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("inbox threshold must be a number")
		}
		if n < 1 {
			return fmt.Errorf("inbox threshold must be at least 1")
		}
	}

	// Blocked days (positive)
	if v := strings.TrimSpace(m.alertBlockedDaysInput.Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("blocked days must be a number")
		}
		if n < 1 {
			return fmt.Errorf("blocked days must be at least 1")
		}
	}

	// Project stale days (positive)
	if v := strings.TrimSpace(m.alertProjectStaleDaysInput.Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("project stale days must be a number")
		}
		if n < 1 {
			return fmt.Errorf("project stale days must be at least 1")
		}
	}

	return nil
}

func (m *SettingsModel) validateEditorCommand(editor string) error {
	// Check if it's an absolute path
	if strings.HasPrefix(editor, "/") {
		info, err := os.Stat(editor)
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", editor)
		}
		if err != nil {
			return fmt.Errorf("cannot access file: %s", editor)
		}
		if info.IsDir() {
			return fmt.Errorf("path is a directory: %s", editor)
		}
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("file is not executable: %s", editor)
		}
		return nil
	}

	// Otherwise, check if command exists in PATH
	_, err := exec.LookPath(editor)
	if err != nil {
		return fmt.Errorf("command not found: %s", editor)
	}
	return nil
}
