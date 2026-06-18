package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
)

func newExternalEditorSettingsModel(t *testing.T, externalEditor string) *SettingsModel {
	t.Helper()

	cfg := &config.Config{
		Editor:         "code",
		DefaultContext: "personal",
		ExternalEditor: externalEditor,
		ActiveTheme:    "dark",
		ThemeDark:      config.DefaultDarkTheme(),
		ThemeLight:     config.DefaultLightTheme(),
	}
	mgr := config.NewManager(filepath.Join(t.TempDir(), "config.json"))
	return NewSettingsModel(cfg, mgr)
}

func TestSettingsExternalEditorRegistryAndSearch(t *testing.T) {
	focusIndex := tabFieldStart[tabGeneral] + 1
	meta, ok := settingsFieldByIndex(focusIndex)
	if !ok {
		t.Fatalf("missing settings field metadata for focus index %d", focusIndex)
	}

	if meta.Label != "External editor" {
		t.Fatalf("Label = %q, want External editor", meta.Label)
	}
	if meta.Default != "code" {
		t.Fatalf("Default = %q, want code", meta.Default)
	}

	legacyConfigKey := strings.Join([]string{"notes", "editor"}, "_")
	searchText := strings.ToLower(meta.Label + " " + meta.Keywords)
	if strings.Contains(searchText, "notes") || strings.Contains(searchText, legacyConfigKey) {
		t.Fatalf("external editor metadata still contains legacy notes terminology: %q", searchText)
	}

	if !settingsSearchHasFocusIndex("external editor", focusIndex) {
		t.Fatal("search for external editor did not find the external editor field")
	}
	if !settingsSearchHasFocusIndex("ctrl o", focusIndex) {
		t.Fatal("search for ctrl o did not find the external editor field")
	}
	legacyQuery := strings.Join([]string{"notes", "editor"}, " ")
	if settingsSearchHasFocusIndex(legacyQuery, focusIndex) {
		t.Fatalf("search for legacy editor query %q still finds the external editor field", legacyQuery)
	}
}

func settingsSearchHasFocusIndex(query string, focusIndex int) bool {
	for _, result := range searchSettingsFields(query) {
		if result.FocusIndex == focusIndex {
			return true
		}
	}
	return false
}

func TestSettingsExternalEditorRenderedLabelAndDescription(t *testing.T) {
	m := newExternalEditorSettingsModel(t, "")
	m.focusIndex = tabFieldStart[tabGeneral] + 1

	rendered := ansi.Strip(m.renderGeneralTab())
	for _, want := range []string{
		"External editor:",
		"Ctrl+O external reference previews",
		"empty defaults to code",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered General tab missing %q:\n%s", want, rendered)
		}
	}

	legacyConfigKey := strings.Join([]string{"notes", "editor"}, "_")
	legacyLabel := strings.Join([]string{"Notes", "Editor"}, " ")
	for _, old := range []string{legacyLabel, legacyConfigKey, "Editor/code"} {
		if strings.Contains(rendered, old) {
			t.Fatalf("rendered General tab still contains %q:\n%s", old, rendered)
		}
	}
}

func TestSettingsExternalEditorInitAndSaveRoundTrip(t *testing.T) {
	m := newExternalEditorSettingsModel(t, "zed")
	if got := m.externalEditorInput.Value(); got != "zed" {
		t.Fatalf("externalEditorInput = %q, want zed", got)
	}

	m.externalEditorInput.SetValue("code --reuse-window")
	m.saveSettings()

	if got := m.config.ExternalEditor; got != "code --reuse-window" {
		t.Fatalf("Config.ExternalEditor = %q, want code --reuse-window", got)
	}
	if got := m.config.GetExternalEditor(); got != "code --reuse-window" {
		t.Fatalf("Config.GetExternalEditor() = %q, want code --reuse-window", got)
	}
}
