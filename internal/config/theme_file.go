package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const themeFileSchema = "tt.theme/v1"

// ThemeFile is the portable on-disk representation of a named theme.
type ThemeFile struct {
	Schema    string    `json:"schema"`
	Name      string    `json:"name"`
	Variant   string    `json:"variant"`
	CreatedAt time.Time `json:"created_at"`
	TTVersion string    `json:"tt_version"`
	Theme     Theme     `json:"theme"`
}

var themeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`)

// NewThemeFile constructs a ThemeFile with the correct schema, timestamp, and
// version string — use this when creating a file for Manager.WriteTheme.
func NewThemeFile(name, variant, version string, t Theme) ThemeFile {
	return ThemeFile{
		Schema:    themeFileSchema,
		Name:      name,
		Variant:   variant,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		TTVersion: version,
		Theme:     t,
	}
}

// MarshalThemeFile serializes a theme into the portable ThemeFile JSON format.
func MarshalThemeFile(name, variant, version string, t Theme) ([]byte, error) {
	tf := ThemeFile{
		Schema:    themeFileSchema,
		Name:      name,
		Variant:   variant,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		TTVersion: version,
		Theme:     t,
	}
	return json.MarshalIndent(tf, "", "  ")
}

// UnmarshalThemeFile deserializes a ThemeFile, validating schema and variant.
func UnmarshalThemeFile(data []byte) (ThemeFile, error) {
	var tf ThemeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return ThemeFile{}, fmt.Errorf("invalid theme file: %w", err)
	}
	if tf.Schema != themeFileSchema {
		return ThemeFile{}, fmt.Errorf("unsupported schema %q (expected %q)", tf.Schema, themeFileSchema)
	}
	if tf.Variant != "dark" && tf.Variant != "light" {
		return ThemeFile{}, fmt.Errorf("invalid variant %q (must be \"dark\" or \"light\")", tf.Variant)
	}
	return tf, nil
}

// ThemesDir returns the directory where theme files are stored (~/.tt/themes/).
func (m *Manager) ThemesDir() string {
	return filepath.Join(filepath.Dir(m.configPath), "themes")
}

// validateThemeName returns an error if name contains path-unsafe characters.
func validateThemeName(name string) error {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") || !themeNameRegex.MatchString(name) {
		return fmt.Errorf("invalid theme name %q: only [a-zA-Z0-9_.-] are allowed, no path separators or ..", name)
	}
	return nil
}

// ThemePath returns the full path for a named theme file.
// Returns an error if the name is invalid.
func (m *Manager) ThemePath(name string) (string, error) {
	if err := validateThemeName(name); err != nil {
		return "", err
	}
	return filepath.Join(m.ThemesDir(), name+".json"), nil
}

// ListThemes returns all valid theme files in the themes directory.
func (m *Manager) ListThemes() ([]ThemeFile, error) {
	dir := m.ThemesDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read themes dir: %w", err)
	}
	var themes []ThemeFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		tf, err := UnmarshalThemeFile(data)
		if err != nil {
			continue
		}
		themes = append(themes, tf)
	}
	return themes, nil
}

// ReadTheme reads a named theme file from the themes directory.
func (m *Manager) ReadTheme(name string) (ThemeFile, error) {
	path, err := m.ThemePath(name)
	if err != nil {
		return ThemeFile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ThemeFile{}, fmt.Errorf("read theme %q: %w", name, err)
	}
	return UnmarshalThemeFile(data)
}

// WriteTheme writes a ThemeFile to the themes directory, creating it if needed.
func (m *Manager) WriteTheme(tf ThemeFile) error {
	dir := m.ThemesDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create themes dir: %w", err)
	}
	path, err := m.ThemePath(tf.Name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal theme: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// DeleteTheme removes the named theme file from ThemesDir.
// Returns a sentinel-wrapped error if the file does not exist so
// callers can distinguish "already gone" from other I/O failures.
func (m *Manager) DeleteTheme(name string) error {
	path, err := m.ThemePath(name) // validates name (no traversal)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("theme %q not found: %w", name, err)
		}
		return fmt.Errorf("delete theme %q: %w", name, err)
	}
	return nil
}
