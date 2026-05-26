package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// presetMode is the state of the palette preset overlay on the Palette tab.
type presetMode int

const (
	presetModeNone         presetMode = iota
	presetModeSave                    // name-input prompt
	presetModeLoad                    // list picker
	presetModeResetConfirm            // y/n confirmation
)

// presetErrMsg carries an error from an async preset operation.
type presetErrMsg struct{ err string }

// presetSavedMsg signals a successful preset save.
type presetSavedMsg struct{ name string }

// presetLoadedMsg signals a successful preset load.
type presetLoadedMsg struct{ name string }

// presetResetMsg signals a successful slot reset.
type presetResetMsg struct{}

// presetDeletedMsg signals a successful preset file deletion.
type presetDeletedMsg struct {
	name      string
	listEmpty bool
}

// presetFlashClearMsg clears the transient palette-tab footer flash.
type presetFlashClearMsg struct{}

// themeFromInputs reads the current active palette inputs and returns a config.Theme.
func (m *SettingsModel) themeFromInputs() config.Theme {
	return config.Theme{
		BackgroundColor: normalizeHexColor(m.bgColorInput.Value()),
		CursorColor:     normalizeHexColor(m.cursorColorInput.Value()),
		CursorTextColor: normalizeHexColor(m.cursorTextColorInput.Value()),
		Palette: config.PaletteColors{
			NormalText:     normalizeHexColor(m.normalTextInput.Value()),
			Muted:          normalizeHexColor(m.mutedInput.Value()),
			Primary:        normalizeHexColor(m.primaryInput.Value()),
			Secondary:      normalizeHexColor(m.secondaryInput.Value()),
			Accent:         normalizeHexColor(m.accentInput.Value()),
			Success:        normalizeHexColor(m.successInput.Value()),
			Warning:        normalizeHexColor(m.warningInput.Value()),
			Error:          normalizeHexColor(m.errorColorInput.Value()),
			Border:         normalizeHexColor(m.borderInput.Value()),
			Priority1Title: normalizeHexColor(m.priority1TitleInput.Value()),
			Priority2Title: normalizeHexColor(m.priority2TitleInput.Value()),
			Priority3Title: normalizeHexColor(m.priority3TitleInput.Value()),
			Priority4Title: normalizeHexColor(m.priority4TitleInput.Value()),
			Priority5Title: normalizeHexColor(m.priority5TitleInput.Value()),
			Dimmed:         normalizeHexColor(m.activeHiddenDimmed[0]),
			DimmedBg:       normalizeHexColor(m.activeHiddenDimmed[1]),
			DimmedFg:       normalizeHexColor(m.activeHiddenDimmed[2]),
		},
	}
}

// populatePaletteInputs fills all active palette inputs from a config.Theme.
// Call ApplyTheme after this to refresh the live preview.
func (m *SettingsModel) populatePaletteInputs(t config.Theme) {
	m.bgColorInput.SetValue(t.BackgroundColor)
	m.cursorColorInput.SetValue(t.CursorColor)
	m.cursorTextColorInput.SetValue(t.CursorTextColor)
	m.normalTextInput.SetValue(t.Palette.NormalText)
	m.mutedInput.SetValue(t.Palette.Muted)
	m.primaryInput.SetValue(t.Palette.Primary)
	m.secondaryInput.SetValue(t.Palette.Secondary)
	m.accentInput.SetValue(t.Palette.Accent)
	m.successInput.SetValue(t.Palette.Success)
	m.warningInput.SetValue(t.Palette.Warning)
	m.errorColorInput.SetValue(t.Palette.Error)
	m.borderInput.SetValue(t.Palette.Border)
	m.priority1TitleInput.SetValue(t.Palette.Priority1Title)
	m.priority2TitleInput.SetValue(t.Palette.Priority2Title)
	m.priority3TitleInput.SetValue(t.Palette.Priority3Title)
	m.priority4TitleInput.SetValue(t.Palette.Priority4Title)
	m.priority5TitleInput.SetValue(t.Palette.Priority5Title)
	// Preserve hidden palette fields (no UI input widgets for these).
	m.activeHiddenDimmed = themeHiddenDimmed(t)
}

// openPresetSave enters save mode and focuses the name-input.
func (m *SettingsModel) openPresetSave() {
	m.themePresetMode = presetModeSave
	m.themePresetErr = ""
	m.themePresetInput.SetValue("")
	m.themePresetInput.Focus()
}

// openPresetLoad enters load mode after fetching the theme list.
// Sets themePresetErr and stays in presetModeNone if no presets exist.
func (m *SettingsModel) openPresetLoad() {
	themes, err := m.configManager.ListThemes()
	if err != nil {
		m.themePresetErr = "could not list presets: " + err.Error()
		return
	}
	if len(themes) == 0 {
		m.themePresetErr = "no saved presets found"
		return
	}
	m.themePresetMode = presetModeLoad
	m.themePresetList = themes
	m.themePresetCursor = 0
	m.themePresetErr = ""
}

// cancelPresetMode exits any preset overlay.
func (m *SettingsModel) cancelPresetMode() {
	m.themePresetMode = presetModeNone
	m.themePresetErr = ""
	m.themePresetInput.Blur()
}

// execPresetSave writes the current palette as a named ThemeFile on disk.
// Also records the name as the active slot's preset so it survives the session.
func (m *SettingsModel) execPresetSave(name string) tea.Cmd {
	tf := config.NewThemeFile(name, m.activeTheme, AppVersion, m.themeFromInputs())
	m.activePresetName = name
	if m.activeTheme == "light" {
		m.config.ThemeLightName = name
	} else {
		m.config.ThemeDarkName = name
	}
	cfg := m.config
	mgr := m.configManager
	return func() tea.Msg {
		if err := mgr.WriteTheme(tf); err != nil {
			return presetErrMsg{err: err.Error()}
		}
		if err := mgr.Save(cfg); err != nil {
			return presetErrMsg{err: err.Error()}
		}
		return presetSavedMsg{name: name}
	}
}

// execPresetLoad loads the currently selected theme into the active slot.
func (m *SettingsModel) execPresetLoad() tea.Cmd {
	if m.themePresetCursor >= len(m.themePresetList) {
		return nil
	}
	tf := m.themePresetList[m.themePresetCursor]
	m.populatePaletteInputs(tf.Theme)
	ApplyTheme(m.buildTentativeThemeConfig())
	m.activePresetName = tf.Name
	if m.activeTheme == "light" {
		m.config.ThemeLight = tf.Theme
		m.config.ThemeLightName = tf.Name
	} else {
		m.config.ThemeDark = tf.Theme
		m.config.ThemeDarkName = tf.Name
	}
	cfg := m.config
	mgr := m.configManager
	name := tf.Name
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return presetErrMsg{err: err.Error()}
		}
		return presetLoadedMsg{name: name}
	}
}

// execPresetDelete removes the currently highlighted preset file from disk.
func (m *SettingsModel) execPresetDelete() tea.Cmd {
	if m.themePresetCursor >= len(m.themePresetList) {
		return nil
	}
	name := m.themePresetList[m.themePresetCursor].Name
	mgr := m.configManager
	return func() tea.Msg {
		if err := mgr.DeleteTheme(name); err != nil {
			return presetErrMsg{err: err.Error()}
		}
		themes, _ := mgr.ListThemes()
		return presetDeletedMsg{name: name, listEmpty: len(themes) == 0}
	}
}

// execPresetReset clears the active slot to blank (runtime defaults apply at render time).
func (m *SettingsModel) execPresetReset() tea.Cmd {
	m.populatePaletteInputs(config.Theme{})
	ApplyTheme(m.buildTentativeThemeConfig())
	m.activePresetName = ""
	if m.activeTheme == "light" {
		m.config.ThemeLight = config.Theme{}
		m.config.ThemeLightName = ""
	} else {
		m.config.ThemeDark = config.Theme{}
		m.config.ThemeDarkName = ""
	}
	cfg := m.config
	mgr := m.configManager
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return presetErrMsg{err: err.Error()}
		}
		return presetResetMsg{}
	}
}

// handlePresetKey dispatches key events while a preset overlay is active.
// Must be called with the full KeyPressMsg so the save-mode textinput can be updated.
func (m *SettingsModel) handlePresetKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch m.themePresetMode {
	case presetModeNone:
		return false, nil
	case presetModeSave:
		return m.handlePresetSaveKey(msg)
	case presetModeLoad:
		return m.handlePresetLoadKey(msg.String())
	case presetModeResetConfirm:
		return m.handlePresetResetKey(msg.String())
	}
	return false, nil
}

func (m *SettingsModel) handlePresetSaveKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelPresetMode()
		return true, nil
	case "enter":
		name := strings.TrimSpace(m.themePresetInput.Value())
		if name == "" {
			m.themePresetErr = "name cannot be empty"
			return true, nil
		}
		cmd := m.execPresetSave(name)
		m.cancelPresetMode()
		return true, cmd
	}
	var cmd tea.Cmd
	m.themePresetInput, cmd = m.themePresetInput.Update(msg)
	return true, cmd
}

func (m *SettingsModel) handlePresetLoadKey(key string) (bool, tea.Cmd) {
	if m.themePresetConfirmDelete.IsActive() {
		confirmed, handled := m.themePresetConfirmDelete.HandleKey(key)
		if handled {
			if confirmed {
				return true, m.execPresetDelete()
			}
			return true, nil
		}
	}

	switch key {
	case "esc":
		m.cancelPresetMode()
		return true, nil
	case "j", "down":
		if m.themePresetCursor < len(m.themePresetList)-1 {
			m.themePresetCursor++
		}
		return true, nil
	case "k", "up":
		if m.themePresetCursor > 0 {
			m.themePresetCursor--
		}
		return true, nil
	case "enter":
		cmd := m.execPresetLoad()
		m.cancelPresetMode()
		return true, cmd
	case "D":
		if len(m.themePresetList) > 0 {
			name := m.themePresetList[m.themePresetCursor].Name
			m.themePresetConfirmDelete.Show("delete", fmt.Sprintf("Delete preset '%s'?", name))
		}
		return true, nil
	}
	return true, nil
}

func (m *SettingsModel) handlePresetResetKey(key string) (bool, tea.Cmd) {
	switch key {
	case "y", "Y":
		cmd := m.execPresetReset()
		m.cancelPresetMode()
		return true, cmd
	case "n", "N", "esc":
		m.cancelPresetMode()
		return true, nil
	}
	return true, nil
}

// renderPresetOverlay returns the rendered overlay string for the active preset mode,
// or empty string when no overlay is active.
func (m *SettingsModel) renderPresetOverlay() string {
	switch m.themePresetMode {
	case presetModeNone:
		return ""
	case presetModeSave:
		return m.renderPresetSaveOverlay()
	case presetModeLoad:
		return m.renderPresetLoadOverlay()
	case presetModeResetConfirm:
		return m.renderPresetResetOverlay()
	}
	return ""
}

func (m *SettingsModel) renderPresetSaveOverlay() string {
	var b strings.Builder
	b.WriteString(SubtitleStyle.Render("Save Palette Preset"))
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Name: "))
	m.themePresetInput.SetStyles(themedTextInputStyles())
	b.WriteString(m.themePresetInput.View())
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Enter to save · Esc to cancel"))
	if m.themePresetErr != "" {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.themePresetErr))
	}
	return OverlayBoxStyle.Render(b.String())
}

func (m *SettingsModel) renderPresetLoadOverlay() string {
	if m.themePresetConfirmDelete.IsActive() {
		return m.themePresetConfirmDelete.RenderOverlay(m.width)
	}

	var b strings.Builder
	b.WriteString(SubtitleStyle.Render("Load Palette Preset"))
	b.WriteString("\n\n")
	for i, tf := range m.themePresetList {
		line := fmt.Sprintf("%-20s  %-5s  %s", tf.Name, tf.Variant, tf.CreatedAt.Format("2006-01-02"))
		if i == m.themePresetCursor {
			b.WriteString(SelectedStyle.Render(IconCursorRight + " " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("j/k Navigate · Enter load · D delete · Esc cancel"))
	if m.themePresetErr != "" {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.themePresetErr))
	}
	return OverlayBoxStyle.Render(b.String())
}

func (m *SettingsModel) renderPresetResetOverlay() string {
	var b strings.Builder
	b.WriteString(SubtitleStyle.Render("Reset Palette Slot"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Clear all %s palette colors back to defaults?\n", m.activeTheme)
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("y/Y Confirm · n/N/Esc Cancel"))
	return OverlayBoxStyle.Render(b.String())
}
