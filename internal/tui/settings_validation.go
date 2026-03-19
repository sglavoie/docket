package tui

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// validateIntInRange parses a required integer input and checks it is within [min, max].
func validateIntInRange(input, fieldName string, min, max int) (int, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, fmt.Errorf("%s is required", fieldName)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", fieldName)
	}
	if n < min || n > max {
		if max == math.MaxInt {
			return 0, fmt.Errorf("%s must be %d or greater", fieldName, min)
		}
		return 0, fmt.Errorf("%s must be %d-%d", fieldName, min, max)
	}
	return n, nil
}

// validateIntOptionalInRange parses an optional integer input.
// Returns (0, false, nil) if empty. Returns (n, true, nil) if valid.
func validateIntOptionalInRange(input, fieldName string, min, max int) (int, bool, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a number", fieldName)
	}
	if n < min || n > max {
		if max == math.MaxInt {
			return 0, false, fmt.Errorf("%s must be %d or greater", fieldName, min)
		}
		return 0, false, fmt.Errorf("%s must be %d-%d", fieldName, min, max)
	}
	return n, true, nil
}

// validateAll runs all settings validations and returns the first error, if any.
func (m *SettingsModel) validateAll() error {
	validators := []func() error{
		m.validateEditor,
		m.validateContext,
		m.validateUndoDepth,
		m.validateBackgroundColor,
		m.validateCursorColor,
		m.validateCursorTextColor,
		m.validateReviewSettings,
		m.validateRecentActivityLimit,
		m.validateMaxPinnedBarItems,
		m.validateMaxRecentTaskVisits,
		m.validateAttachmentEditor,
		m.validateCodeEditor,
		m.validateAlertSettings,
	}
	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}
	return nil
}

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
	_, err := validateIntInRange(m.undoDepthInput.Value(), "undo history depth", 0, 1000)
	return err
}

func (m *SettingsModel) validateReviewSettings() error {
	dueSoon, err := validateIntInRange(m.dueSoonInput.Value(), "due soon days", 0, math.MaxInt)
	if err != nil {
		return err
	}

	futureDays, err := validateIntInRange(m.futureInput.Value(), "future days", 0, math.MaxInt)
	if err != nil {
		return err
	}

	// Future must be > Due Soon when both are non-zero
	if dueSoon > 0 && futureDays > 0 && futureDays <= dueSoon {
		return fmt.Errorf("future must be greater than due soon")
	}

	if _, err := validateIntInRange(m.oldDaysInput.Value(), "old days", 0, math.MaxInt); err != nil {
		return err
	}

	if _, err := validateIntInRange(m.completedHoursInput.Value(), "completed hours", 0, math.MaxInt); err != nil {
		return err
	}

	return nil
}

func (m *SettingsModel) validateRecentActivityLimit() error {
	_, err := validateIntInRange(m.recentActivityInput.Value(), "recent activity limit", 0, math.MaxInt)
	return err
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

func (m *SettingsModel) validateMaxRecentTaskVisits() error {
	_, err := validateIntInRange(m.maxRecentTaskVisitsInput.Value(), "task limit", 1, 50)
	return err
}

func (m *SettingsModel) validateMaxPinnedBarItems() error {
	_, err := validateIntInRange(m.maxPinnedInput.Value(), "bar limit", 0, 20)
	return err
}

func (m *SettingsModel) validateAlertSettings() error {
	if _, _, err := validateIntOptionalInRange(m.alertDailyReviewHourInput.Value(), "review hour", 0, 23); err != nil {
		return err
	}

	if _, _, err := validateIntOptionalInRange(m.alertDueSoonDaysInput.Value(), "due soon days", 1, math.MaxInt); err != nil {
		return err
	}

	if _, _, err := validateIntOptionalInRange(m.alertStaleDaysInput.Value(), "stale days", 1, math.MaxInt); err != nil {
		return err
	}

	if _, _, err := validateIntOptionalInRange(m.alertInboxThresholdInput.Value(), "inbox threshold", 1, math.MaxInt); err != nil {
		return err
	}

	if _, _, err := validateIntOptionalInRange(m.alertBlockedDaysInput.Value(), "blocked days", 1, math.MaxInt); err != nil {
		return err
	}

	if _, _, err := validateIntOptionalInRange(m.alertProjectStaleDaysInput.Value(), "project stale days", 1, math.MaxInt); err != nil {
		return err
	}

	return nil
}

// commitStageEdit validates and applies an in-progress stage add or rename.
// Returns an error if the value is a duplicate of an existing stage.
func (m *SettingsModel) commitStageEdit() error {
	val := strings.TrimSpace(m.stageInput.Value())
	if val != "" {
		if m.stageAddMode {
			if m.hasDuplicateStage(val, -1) {
				return fmt.Errorf("stage '%s' already exists", val)
			}
			m.stages = append(m.stages, val)
			m.stageCursor = len(m.stages) - 1
		} else {
			if m.hasDuplicateStage(val, m.stageCursor) {
				return fmt.Errorf("stage '%s' already exists", val)
			}
			m.stages[m.stageCursor] = val
		}
	}
	m.stageEditMode = false
	m.stageAddMode = false
	m.stageInput.Blur()
	return nil
}

// hasDuplicateStage checks if a stage name already exists, excluding the given index.
// Pass excludeIdx=-1 to check all stages (for add mode).
func (m *SettingsModel) hasDuplicateStage(name string, excludeIdx int) bool {
	lower := strings.ToLower(name)
	for i, s := range m.stages {
		if i == excludeIdx {
			continue
		}
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return false
}

// hasUnsavedStages reports whether the current stages differ from the saved snapshot.
func (m *SettingsModel) hasUnsavedStages() bool {
	if len(m.stages) != len(m.savedStages) {
		return true
	}
	for i := range m.stages {
		if m.stages[i] != m.savedStages[i] {
			return true
		}
	}
	return false
}

// hexColorRegex matches #rgb or #rrggbb hex color strings.
var hexColorRegex = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func (m *SettingsModel) validateBackgroundColor() error {
	val := strings.TrimSpace(m.bgColorInput.Value())
	if val == "" {
		return nil // empty = no background
	}
	if !hexColorRegex.MatchString(val) {
		return fmt.Errorf("must be #rgb or #rrggbb hex color")
	}
	return nil
}

func (m *SettingsModel) validateCursorColor() error {
	val := strings.TrimSpace(m.cursorColorInput.Value())
	if val == "" {
		return nil // empty = default
	}
	if !hexColorRegex.MatchString(val) {
		return fmt.Errorf("must be #rgb or #rrggbb hex color")
	}
	return nil
}

func (m *SettingsModel) validateCursorTextColor() error {
	val := strings.TrimSpace(m.cursorTextColorInput.Value())
	if val == "" {
		return nil // empty = default
	}
	if !hexColorRegex.MatchString(val) {
		return fmt.Errorf("must be #rgb or #rrggbb hex color")
	}
	return nil
}

// normalizeHexColor expands 3-digit hex (#rgb → #rrggbb) and lowercases.
// Returns empty string unchanged.
func normalizeHexColor(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	val = strings.ToLower(val)
	if len(val) == 4 { // #rgb → #rrggbb
		return string([]byte{'#', val[1], val[1], val[2], val[2], val[3], val[3]})
	}
	return val
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
