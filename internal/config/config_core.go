package config

import (
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// File permission constants (re-exported from util for backward compatibility).
const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

// DefaultUndoHistoryDepth is the default number of undo snapshots to keep.
const DefaultUndoHistoryDepth = 50

// CurrentConfigVersion is the latest config schema version supported by this build.
const CurrentConfigVersion = 2

// NewConfig creates a new configuration with default values.
func NewConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultUndoDepth := DefaultUndoHistoryDepth
	return &Config{
		Version:           CurrentConfigVersion,
		DataDir:           filepath.Join(homeDir, ".tt"),
		CurrentProject:    "",
		Editor:            "code",
		DefaultContext:    "personal",
		FzfEnabled:        true,
		GlowEnabled:       true,
		UndoHistoryDepth:  &defaultUndoDepth,
		Projects:          []*models.Project{},
		SectionColors:     make(map[string]string),
		ProjectViewStates: make(map[string]ProjectViewState),
		KanbanSettings: KanbanSettings{
			DefaultMode:        "section",
			DefaultStartColumn: "inbox",
		},
	}
}

// GetEditor returns the configured editor.
func (c *Config) GetEditor() string {
	return c.Editor
}

// EnsureDataDir creates the data directory and its standard subdirectories if they don't exist.
func (c *Config) EnsureDataDir() error {
	if err := os.MkdirAll(c.DataDir, DirPermissions); err != nil {
		return err
	}
	if err := os.MkdirAll(c.GetInternalDir(), DirPermissions); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(c.DataDir, "projects"), DirPermissions)
}

// GetProjectPath returns the full path for a project's data directory.
func (c *Config) GetProjectPath(slug string) string {
	return filepath.Join(c.DataDir, "projects", slug)
}

// GetInternalDir returns the path to the internal .tt directory within the data directory.
func (c *Config) GetInternalDir() string {
	return filepath.Join(c.DataDir, ".tt")
}

// GetAttachmentEditor returns the configured editor for general attachments.
// Returns empty string if not set (use OS default).
func (c *Config) GetAttachmentEditor() string {
	return c.AttachmentEditor
}

// GetFzfTimeout returns the fzf invocation timeout in seconds.
// Defaults to 30 if not set; values <= 0 are clamped to 1.
func (c *Config) GetFzfTimeout() int {
	if c.FzfTimeout == nil {
		return 30
	}
	if *c.FzfTimeout <= 0 {
		return 1
	}
	return *c.FzfTimeout
}

// ZebraStriping returns true when alternating row backgrounds are enabled.
// Defaults to false.
func (c *Config) ZebraStriping() bool {
	return c.TUIZebraStriping != nil && *c.TUIZebraStriping
}

// GetPopupTimeoutSeconds returns the auto-dismiss timeout for prefix popups in seconds.
// Returns 0 when nil or when the stored value is negative (disabled — popup stays until manual dismiss).
func (c *Config) GetPopupTimeoutSeconds() int {
	if c.PopupTimeoutSeconds == nil {
		return 0
	}
	if *c.PopupTimeoutSeconds < 0 {
		return 0
	}
	return *c.PopupTimeoutSeconds
}

// GetCodeEditor returns the configured editor for code references.
// Defaults to "code" (VS Code) if not set.
func (c *Config) GetCodeEditor() string {
	if c.CodeEditor != "" {
		return c.CodeEditor
	}
	if editor := c.GetEditor(); editor != "" {
		return editor
	}
	return "code"
}

// GetUseEmoji returns whether emoji icons are enabled. Defaults to false (Unicode symbols).
func (c *Config) GetUseEmoji() bool {
	return c.UseEmoji != nil && *c.UseEmoji
}

// IsDescriptionColumnVisible reports whether the description-indicator column
// is enabled in OutlinerDefaultColumns. Key mirrors tui.ColDescription.
func (c *Config) IsDescriptionColumnVisible() bool {
	if c == nil {
		return true
	}
	if v, ok := c.OutlinerDefaultColumns["description"]; ok {
		return v
	}
	return true
}

// GetNotesEditor returns the configured editor for opening notes with Ctrl+O.
// Defaults to "code" (VS Code) if not set.
func (c *Config) GetNotesEditor() string {
	if c.NotesEditor != "" {
		return c.NotesEditor
	}
	if editor := c.GetEditor(); editor != "" {
		return editor
	}
	return "code"
}
