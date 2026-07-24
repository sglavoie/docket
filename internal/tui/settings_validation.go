package tui

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/external"
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
		m.validateExternalEditor,
		m.validateContext,
		m.validateUndoDepth,
		m.validateBackgroundColor,
		m.validateCursorColor,
		m.validateCursorTextColor,
		m.validatePaletteColors,
		m.validatePriorityTitleColors,
		m.validateReviewSettings,
		m.validateRecentActivityLimit,
		m.validateMaxPinnedBarItems,
		m.validateMaxRecentTaskVisits,
		m.validateSectionLimits,
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
	return m.validateEditorCommand(editor)
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

	if _, err := validateIntInRange(m.staleTaskDaysInput.Value(), "stale badge days", 0, math.MaxInt); err != nil {
		return err
	}

	return nil
}

func (m *SettingsModel) validateRecentActivityLimit() error {
	_, err := validateIntInRange(m.recentActivityInput.Value(), "recent activity limit", 0, math.MaxInt)
	return err
}

func (m *SettingsModel) validateSectionLimits() error {
	inputs := []struct {
		input textinput.Model
		name  string
	}{
		{m.sectionLimitActiveInput, "Active limit"},
		{m.sectionLimitNextInput, "Next limit"},
		{m.sectionLimitInboxInput, "Inbox limit"},
		{m.sectionLimitStandbyInput, "Standby limit"},
		{m.sectionLimitWaitingInput, "Waiting limit"},
		{m.sectionLimitSomedayInput, "Someday limit"},
		{m.sectionLimitReviewInput, "Review limit"},
	}
	for _, inp := range inputs {
		if _, err := validateIntInRange(inp.input.Value(), inp.name, 0, 999); err != nil {
			return err
		}
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

func (m *SettingsModel) validateExternalEditor() error {
	editor := strings.TrimSpace(m.externalEditorInput.Value())
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

// hexColorRegex matches hex color strings with or without leading #.
// Accepts: #rgb, #rrggbb, rgb, rrggbb.
var hexColorRegex = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func (m *SettingsModel) validateBackgroundColor() error {
	val := strings.TrimSpace(m.bgColorInput.Value())
	if val == "" {
		return nil // empty = no background
	}
	if !hexColorRegex.MatchString(val) {
		return fmt.Errorf("must be hex color (e.g. aabbcc or #abc)")
	}
	return nil
}

func (m *SettingsModel) validateCursorColor() error {
	val := strings.TrimSpace(m.cursorColorInput.Value())
	if val == "" {
		return nil // empty = default
	}
	if !hexColorRegex.MatchString(val) {
		return fmt.Errorf("must be hex color (e.g. aabbcc or #abc)")
	}
	return nil
}

func (m *SettingsModel) validatePaletteColors() error {
	inputs := []struct {
		input textinput.Model
		name  string
	}{
		{m.normalTextInput, "normal text"},
		{m.mutedInput, "muted"},
		{m.primaryInput, "primary"},
		{m.secondaryInput, "secondary"},
		{m.accentInput, "accent"},
		{m.successInput, "success"},
		{m.warningInput, "warning"},
		{m.errorColorInput, "error"},
		{m.borderInput, "border"},
	}
	for _, inp := range inputs {
		val := strings.TrimSpace(inp.input.Value())
		if val == "" {
			continue
		}
		if !hexColorRegex.MatchString(val) {
			return fmt.Errorf("%s: must be hex color (e.g. aabbcc or #abc)", inp.name)
		}
	}
	return nil
}

func (m *SettingsModel) validatePriorityTitleColors() error {
	inputs := []struct {
		input textinput.Model
		name  string
	}{
		{m.priority1TitleInput, "priority 1 title"},
		{m.priority2TitleInput, "priority 2 title"},
		{m.priority3TitleInput, "priority 3 title"},
		{m.priority4TitleInput, "priority 4 title"},
		{m.priority5TitleInput, "priority 5 title"},
	}
	for _, inp := range inputs {
		val := strings.TrimSpace(inp.input.Value())
		if val == "" {
			continue
		}
		if !hexColorRegex.MatchString(val) {
			return fmt.Errorf("%s: must be hex color (e.g. aabbcc or #abc)", inp.name)
		}
	}
	return nil
}

func (m *SettingsModel) validateCursorTextColor() error {
	val := strings.TrimSpace(m.cursorTextColorInput.Value())
	if val == "" {
		return nil // empty = default
	}
	if !hexColorRegex.MatchString(val) {
		return fmt.Errorf("must be hex color (e.g. aabbcc or #abc)")
	}
	return nil
}

// normalizeHexColor ensures a leading # and expands 3-digit hex to 6-digit.
// Accepts rgb, rrggbb, #rgb, #rrggbb. Returns empty string unchanged.
func normalizeHexColor(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	// Prepend # if missing
	if val[0] != '#' {
		val = "#" + val
	}
	val = strings.ToLower(val)
	if len(val) == 4 { // #rgb → #rrggbb
		return string([]byte{'#', val[1], val[1], val[2], val[2], val[3], val[3]})
	}
	return val
}

func (m *SettingsModel) validateEditorCommand(editor string) error {
	exe, _ := external.SplitEditorString(editor)
	if exe == "" {
		return fmt.Errorf("editor is required")
	}
	// Check if it's an absolute path
	if strings.HasPrefix(exe, "/") {
		info, err := os.Stat(exe)
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", exe)
		}
		if err != nil {
			return fmt.Errorf("cannot access file: %s", exe)
		}
		if info.IsDir() {
			return fmt.Errorf("path is a directory: %s", exe)
		}
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("file is not executable: %s", exe)
		}
		return nil
	}

	// Otherwise, check if command exists in PATH
	_, err := exec.LookPath(exe)
	if err != nil {
		return fmt.Errorf("command not found: %s", exe)
	}
	return nil
}
