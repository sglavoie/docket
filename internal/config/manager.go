package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// clampIntPtrMin clamps a non-nil *int to a minimum of 0, logging a warning on change.
func clampIntPtrMin(p *int, name string) {
	if p != nil && *p < 0 {
		logging.Warn("%s %d is negative, clamping to 0", name, *p)
		*p = 0
	}
}

// clampIntPtrRange clamps a non-nil *int to [min, max], logging a warning on change.
func clampIntPtrRange(p *int, name string, min, max int) {
	if p == nil {
		return
	}
	if *p < min {
		logging.Warn("%s %d is below minimum, clamping to %d", name, *p, min)
		*p = min
	} else if *p > max {
		logging.Warn("%s %d exceeds maximum, clamping to %d", name, *p, max)
		*p = max
	}
}

// Manager handles configuration file operations.
type Manager struct {
	configPath string
}

// NewManager creates a new configuration manager.
func NewManager(customPath string) *Manager {
	var configPath string

	if customPath != "" {
		configPath = customPath
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not determine home directory, using .tt.json in current directory\n")
			configPath = ".tt.json"
		} else {
			configPath = filepath.Join(homeDir, ".tt", ".tt.json")
		}
	}

	return &Manager{
		configPath: configPath,
	}
}

// recoverFromBackup attempts to restore a Config from the backup file when the
// primary config is corrupted. On success the recovered data is persisted back
// to configPath so subsequent loads don't hit the corruption again.
func (m *Manager) recoverFromBackup(primaryErr error) (Config, error) {
	backupPath := m.configPath + ".bak"
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse config file (no backup available): %w", primaryErr)
	}
	backupData = migrateLegacyConfigKeys(backupData)

	var cfg Config
	if json.Unmarshal(backupData, &cfg) != nil {
		return Config{}, fmt.Errorf("failed to parse config file and backup is also corrupted: config error: %w; backup path: %s", primaryErr, backupPath)
	}

	logging.Warn("config file was corrupted, restored from backup %s", backupPath)
	if writeErr := util.AtomicWriteFile(m.configPath, backupData, FilePermissions); writeErr != nil {
		return Config{}, fmt.Errorf("recovered config from backup but failed to persist it: %w", writeErr)
	}
	return cfg, nil
}

func migrateLegacyConfigKeys(data []byte) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return data
	}

	notesEditor, hasNotesEditor := raw["notes_editor"]
	if !hasNotesEditor {
		return data
	}
	if _, hasExternalEditor := raw["external_editor"]; !hasExternalEditor {
		raw["external_editor"] = notesEditor
	}
	delete(raw, "notes_editor")

	migrated, err := json.Marshal(raw)
	if err != nil {
		return data
	}
	return migrated
}

// resolveEditorDefault picks the best available editor via the env/PATH fallback
// chain: EDITOR env → VISUAL env → "code" → "vi" → "nano" → "vi" (last resort).
// It is called for fresh configs (no config file) and when the config Editor
// field is empty.
func resolveEditorDefault() string {
	if ed := os.Getenv("EDITOR"); ed != "" {
		return ed
	}
	if vis := os.Getenv("VISUAL"); vis != "" {
		return vis
	}
	for _, candidate := range []string{"code", "vi", "nano"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return "vi" // last resort even if not found
}

// Load loads the configuration from file.
func (m *Manager) Load() (*Config, error) { //nolint:gocyclo
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		cfg := NewConfig()
		// Respect $EDITOR/$VISUAL and PATH availability instead of always
		// defaulting to "code" which may not be installed.
		cfg.Editor = resolveEditorDefault()
		cfg.MigratePaletteToThemes()
		cfg.Version = CurrentConfigVersion
		return cfg, nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	data = migrateLegacyConfigKeys(data)

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		recovered, recoverErr := m.recoverFromBackup(err)
		if recoverErr != nil {
			return nil, recoverErr
		}
		config = recovered
	}
	if config.Version > CurrentConfigVersion {
		return nil, fmt.Errorf("config version %d not supported, please upgrade tt", config.Version)
	}

	// Ensure defaults for missing fields
	if config.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine home directory for data dir: %w", err)
		}
		config.DataDir = filepath.Join(homeDir, ".tt")
	}
	if config.Editor == "" {
		config.Editor = resolveEditorDefault()
	}
	// Validate only the executable portion so that editor strings with flags
	// (e.g. "code --wait") or quoted paths with spaces do not trigger a bogus
	// "not found" warning.
	if editorTokens := util.ShellTokenize(config.Editor); len(editorTokens) > 0 {
		if _, err := exec.LookPath(editorTokens[0]); err != nil {
			fmt.Fprintf(os.Stderr, "warning: editor '%s' not found in PATH; editing commands may fail\n", editorTokens[0])
		}
	}
	if config.DefaultContext == "" {
		config.DefaultContext = "personal"
	}
	if config.Projects == nil {
		config.Projects = []*models.Project{}
	}
	if config.SectionColors == nil {
		config.SectionColors = make(map[string]string)
	}
	if config.ProjectViewStates == nil {
		config.ProjectViewStates = make(map[string]ProjectViewState)
	}
	if config.UndoHistoryDepth == nil {
		// Default for existing configs that don't have this field
		defaultUndoDepth := DefaultUndoHistoryDepth
		config.UndoHistoryDepth = &defaultUndoDepth
	}
	// Clamp UndoHistoryDepth to valid range
	clampIntPtrRange(config.UndoHistoryDepth, "UndoHistoryDepth", 0, 1000)

	// Clamp optional *int fields to >= 0
	clampIntPtrMin(config.WorkflowEvalInterval, "WorkflowEvalInterval")
	clampIntPtrMin(config.ReviewDueSoonDays, "ReviewDueSoonDays")
	clampIntPtrMin(config.ReviewOldDays, "ReviewOldDays")
	clampIntPtrMin(config.ReviewFutureDays, "ReviewFutureDays")
	clampIntPtrMin(config.ReviewCompletedHours, "ReviewCompletedHours")
	clampIntPtrMin(config.MaxPinnedBarItems, "MaxPinnedBarItems")
	clampIntPtrRange(config.MaxRecentTaskVisits, "MaxRecentTaskVisits", 1, 50)
	clampIntPtrMin(config.RecentActivityLimit, "RecentActivityLimit")
	clampIntPtrMin(config.MaxActivityEntries, "MaxActivityEntries")
	clampIntPtrMin(config.WeeklyStaleProjectDays, "WeeklyStaleProjectDays")
	clampIntPtrMin(config.WeeklyStaleTaskDays, "WeeklyStaleTaskDays")
	clampIntPtrMin(config.WeeklyInboxThreshold, "WeeklyInboxThreshold")
	clampIntPtrMin(config.WeeklyBlockedThreshold, "WeeklyBlockedThreshold")
	clampIntPtrMin(config.WeeklyInProgressThreshold, "WeeklyInProgressThreshold")
	clampIntPtrMin(config.StaleTaskDays, "StaleTaskDays")
	clampIntPtrMin(config.PriorityEscalationDays, "PriorityEscalationDays")
	clampIntPtrMin(config.AlertDueSoonDays, "AlertDueSoonDays")
	clampIntPtrMin(config.AlertStaleDays, "AlertStaleDays")
	clampIntPtrMin(config.AlertInboxThreshold, "AlertInboxThreshold")
	clampIntPtrMin(config.AlertBlockedDays, "AlertBlockedDays")
	clampIntPtrMin(config.AlertProjectStaleDays, "AlertProjectStaleDays")
	clampIntPtrMin(config.SectionLimitActive, "SectionLimitActive")
	clampIntPtrMin(config.SectionLimitNext, "SectionLimitNext")
	clampIntPtrMin(config.SectionLimitInbox, "SectionLimitInbox")
	clampIntPtrMin(config.SectionLimitStandby, "SectionLimitStandby")
	clampIntPtrMin(config.SectionLimitWaiting, "SectionLimitWaiting")
	clampIntPtrMin(config.SectionLimitSomeday, "SectionLimitSomeday")
	clampIntPtrMin(config.SectionLimitReview, "SectionLimitReview")

	// Clamp timeouts and other range-limited *int fields so an out-of-range
	// stored value doesn't keep round-tripping through Save. Getters already
	// clamp on read but the bogus stored value would otherwise persist
	// indefinitely.
	if config.FzfTimeout != nil && *config.FzfTimeout <= 0 {
		logging.Warn("FzfTimeout %d is <= 0, clamping to 1", *config.FzfTimeout)
		one := 1
		config.FzfTimeout = &one
	}
	if config.PopupTimeoutSeconds != nil && *config.PopupTimeoutSeconds < 0 {
		logging.Warn("PopupTimeoutSeconds %d is negative, clamping to 0", *config.PopupTimeoutSeconds)
		zero := 0
		config.PopupTimeoutSeconds = &zero
	}
	if config.AlertDailyReviewHour != nil {
		v := *config.AlertDailyReviewHour
		if v < 0 || v > 23 {
			logging.Warn("AlertDailyReviewHour %d out of range, clamping to default %d", v, DefaultAlertDailyReviewHour)
			def := DefaultAlertDailyReviewHour
			config.AlertDailyReviewHour = &def
		}
	}
	if config.AlertWeeklyReviewDay != nil {
		v := *config.AlertWeeklyReviewDay
		if v < 0 || v > 6 {
			logging.Warn("AlertWeeklyReviewDay %d out of range, clamping to default %d", v, DefaultAlertWeeklyReviewDay)
			def := DefaultAlertWeeklyReviewDay
			config.AlertWeeklyReviewDay = &def
		}
	}
	if config.DashboardHorizontalSplitPct != nil {
		v := *config.DashboardHorizontalSplitPct
		if v < MinDashboardSplitPct || v > MaxDashboardSplitPct {
			logging.Warn("DashboardHorizontalSplitPct %d out of range, clamping", v)
			c := clampSplitPct(v)
			config.DashboardHorizontalSplitPct = &c
		}
	}
	if config.DashboardVerticalSplitPct != nil {
		v := *config.DashboardVerticalSplitPct
		if v < MinDashboardSplitPct || v > MaxDashboardSplitPct {
			logging.Warn("DashboardVerticalSplitPct %d out of range, clamping", v)
			c := clampSplitPct(v)
			config.DashboardVerticalSplitPct = &c
		}
	}

	// Clamp enum-style stored values back into their valid range so an
	// out-of-range value (e.g. from a hand-edited config or a downgrade from
	// a newer build) doesn't keep round-tripping unchanged through Save.
	// GetDashboardBottomLayout/GetInfoPanelMode already clamp on read, but
	// without this they'd silently re-persist the bogus value.
	if config.DashboardBottomLayout != nil {
		v := *config.DashboardBottomLayout
		if v < 0 || v >= int(dashboardLayoutCount) {
			logging.Warn("DashboardBottomLayout %d out of range, clamping to 0", v)
			zero := int(DashboardLayoutHorizontal)
			config.DashboardBottomLayout = &zero
		}
	}
	if config.InfoPanelMode != nil {
		v := *config.InfoPanelMode
		if v < 0 || v >= int(infoPanelModeCount) {
			logging.Warn("InfoPanelMode %d out of range, clamping to %d", v, int(InfoPanelInfoCards))
			def := int(InfoPanelInfoCards)
			config.InfoPanelMode = &def
		}
	}

	if config.Stages == nil {
		config.Stages = []string{"Brainstorm", "Design", "Implement", "Test", "Review", "Deploy"}
	}
	if config.CustomContexts == nil {
		config.CustomContexts = make(map[string][]string)
	}
	if config.ProjectScoringProfile == nil {
		config.ProjectScoringProfile = make(map[string]string)
	}
	if config.SavedFilters == nil {
		config.SavedFilters = []SavedFilter{}
	}
	normalizeKanbanSettings(&config.KanbanSettings)

	// Filter out projects with missing required fields. A JSON `null` entry in
	// the projects array deserializes to a nil pointer; treat those as invalid
	// rather than panicking when accessing fields.
	validProjects := make([]*models.Project, 0, len(config.Projects))
	for _, p := range config.Projects {
		if p == nil {
			fmt.Fprintf(os.Stderr, "warning: dropping nil project entry from config\n")
			continue
		}
		if p.ID == "" || p.Name == "" {
			fmt.Fprintf(os.Stderr, "warning: dropping project with missing ID/name from config (ID=%q Name=%q)\n", p.ID, p.Name)
			continue
		}
		validProjects = append(validProjects, p)
	}
	if len(validProjects) != len(config.Projects) {
		config.Projects = validProjects
	}

	// Drop duplicate-ID and duplicate non-empty-slug projects, keeping the
	// first occurrence. Both are hard invariant violations because GetProject
	// resolves by first match; leaving later duplicates in place would make the
	// surviving config ambiguous relative to create/edit paths, which require
	// slug uniqueness. Skip empty slugs so projects missing the field don't
	// produce a spurious "duplicate" warning for every pair of empty-slug
	// entries.
	seenIDs := make(map[string]bool)
	seenSlugs := make(map[string]bool)
	deduped := make([]*models.Project, 0, len(validProjects))
	for _, p := range validProjects {
		if seenIDs[p.ID] {
			fmt.Fprintf(os.Stderr, "warning: dropping duplicate project ID %q from config\n", p.ID)
			continue
		}
		if p.Slug != "" && seenSlugs[p.Slug] {
			fmt.Fprintf(os.Stderr, "warning: dropping duplicate project slug %q from config\n", p.Slug)
			continue
		}
		seenIDs[p.ID] = true
		if p.Slug != "" {
			seenSlugs[p.Slug] = true
		}
		deduped = append(deduped, p)
	}
	if len(deduped) != len(validProjects) {
		config.Projects = deduped
	}

	// Clear a dangling CurrentProject pointer (e.g. the referenced project was
	// dropped above, or the file was hand-edited). Otherwise the stale value
	// keeps shipping through Save round-trips even though it resolves to nil.
	if config.CurrentProject != "" {
		stillExists := false
		for _, p := range config.Projects {
			if p.ID == config.CurrentProject || p.Slug == config.CurrentProject {
				stillExists = true
				break
			}
		}
		if !stillExists {
			fmt.Fprintf(os.Stderr, "warning: clearing dangling current_project %q (no matching project)\n", config.CurrentProject)
			config.CurrentProject = ""
		}
	}

	// Drop persisted references to project IDs that no longer exist (e.g. a
	// project was removed via a hand edit). Without this, stale entries
	// resurrect on every Save round-trip.
	config.PruneOrphanedProjectRefs()

	// Backfill new fields on builtin scoring profiles from presets
	presets := scoring.PresetProfiles()
	for i := range config.ScoringProfiles {
		p := &config.ScoringProfiles[i]
		if p.Builtin {
			if factory, ok := presets[p.ID]; ok {
				preset := factory()
				if p.UsageHint == "" {
					p.UsageHint = preset.UsageHint
				}
			}
		}
	}

	// Migrate legacy savedViews and smartLists to unified SavedFilters.
	if config.MigrateLegacyFilters() {
		// Auto-save so legacy fields are cleared on disk.
		if err := m.Save(&config); err != nil {
			return nil, fmt.Errorf("failed to save config after filter migration: %w", err)
		}
	}

	// Migrate legacy flat palette/bg/cursor fields into ThemeDark/ThemeLight.
	if config.MigratePaletteToThemes() {
		if err := m.Save(&config); err != nil {
			return nil, fmt.Errorf("failed to save config after palette theme migration: %w", err)
		}
	}

	// Prune stale data that accumulates over time.
	pruned := config.PruneExpiredAlertDismissals()
	pruned = config.PruneFocusDismissedIDs() || pruned
	if pruned {
		if err := m.Save(&config); err != nil {
			logging.Warn("failed to save config after pruning stale data: %v", err)
		}
	}

	// Config version migration and validation.
	// Strategy: "defaults-on-load" — missing fields are backfilled with defaults
	// above, so older configs gain new fields transparently without a migration.
	// Explicit migration blocks below are only needed for breaking changes that
	// require data transformation (e.g., renaming a field, restructuring data).
	// Each version bump gets a dedicated migration block below.
	// When adding version 2, add a `if config.Version == 1 { ... }` block
	// that transforms the data, then set config.Version = 2 and save.
	// Bump CurrentConfigVersion in config_core.go to match.
	if config.Version == 0 {
		config.Version = 1
		if err := m.Save(&config); err != nil {
			return nil, fmt.Errorf("failed to save config after version migration: %w", err)
		}
	}
	if config.Version == 1 {
		// v1 → v2: palette data already migrated into ThemeDark/ThemeLight by
		// MigratePaletteToThemes() above. Just bump the version and save.
		config.Version = 2
		if err := m.Save(&config); err != nil {
			return nil, fmt.Errorf("failed to save config after v1→v2 version bump: %w", err)
		}
	}
	if config.Version == 2 {
		// v2 -> v3: notes_editor was renamed to external_editor by the raw
		// key migration before unmarshalling.
		config.Version = 3
		if err := m.Save(&config); err != nil {
			return nil, fmt.Errorf("failed to save config after v2->v3 version bump: %w", err)
		}
	}

	return &config, nil
}

// Save saves the configuration to file.
// A backup of the existing config is created before writing, enabling
// recovery if the new file is corrupted.
func (m *Manager) Save(config *Config) error {
	if config == nil {
		return fmt.Errorf("config is required")
	}
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	lockPath := m.configPath + ".lock"
	return util.WithFileLock(lockPath, true, func() error {
		// Backup current config before overwriting (best-effort).
		// On failure, try an alternate path before giving up.
		if err := m.BackupConfig(); err != nil {
			altPath := m.configPath + ".bak2"
			if altErr := m.backupConfigToPath(altPath); altErr != nil {
				fmt.Fprintf(os.Stderr, "warning: config backup failed (primary: %v; alt %s: %v)\n", err, altPath, altErr)
			}
		}

		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := util.AtomicWriteFile(m.configPath, data, FilePermissions); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		return nil
	})
}

// GetConfigPath returns the current config file path.
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// Exists checks if the config file exists.
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.configPath)
	return err == nil
}

// LoadOrCreate loads existing config or creates a new one.
func (m *Manager) LoadOrCreate() (*Config, error) {
	config, err := m.Load()
	if err != nil {
		return nil, err
	}

	if !m.Exists() {
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to create initial config file: %w", err)
		}
	}

	return config, nil
}

// GetBackupPath returns the path to the config backup file.
func (m *Manager) GetBackupPath() string {
	return m.configPath + ".bak"
}

// ConfigBackupExists checks if a config backup file exists.
func (m *Manager) ConfigBackupExists() bool {
	_, err := os.Stat(m.GetBackupPath())
	return err == nil
}

// backupConfigToPath writes the current config file to an arbitrary backup path.
func (m *Manager) backupConfigToPath(path string) error {
	content, err := os.ReadFile(m.configPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read config file for backup: %w", err)
	}
	if err := util.AtomicWriteFile(path, content, FilePermissions); err != nil {
		return fmt.Errorf("failed to write config backup to %s: %w", path, err)
	}
	return nil
}

// BackupConfig creates a backup of the current config file.
func (m *Manager) BackupConfig() error {
	return m.backupConfigToPath(m.GetBackupPath())
}

// RestoreConfigFromBackup swaps the current config file with the backup.
// This makes the operation reversible - running restore again undoes the restore.
func (m *Manager) RestoreConfigFromBackup() error {
	backupPath := m.GetBackupPath()

	// Check if backup exists
	if !m.ConfigBackupExists() {
		return fmt.Errorf("no config backup available")
	}

	// Read backup content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read config backup file: %w", err)
	}

	// Read current content (may not exist)
	currentContent, err := os.ReadFile(m.configPath)
	if os.IsNotExist(err) {
		currentContent = []byte{}
	} else if err != nil {
		return fmt.Errorf("failed to read current config file: %w", err)
	}

	// Write backup content to main file
	if err := util.AtomicWriteFile(m.configPath, backupContent, FilePermissions); err != nil {
		return fmt.Errorf("failed to restore config file: %w", err)
	}

	// Write current content to backup (for reversibility)
	if err := util.AtomicWriteFile(backupPath, currentContent, FilePermissions); err != nil {
		return fmt.Errorf("failed to update config backup file: %w", err)
	}

	return nil
}
