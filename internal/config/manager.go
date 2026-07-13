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
	// fromDefaultPath records whether this manager resolved its config path
	// from the active default (no explicit --config override). Later
	// first-run migration is gated on this so an explicit config path never
	// triggers default-path migration.
	fromDefaultPath bool
}

// NewManager creates a new configuration manager. An empty customPath resolves
// the config file through the central default-path resolver (iCloud Drive when
// available, otherwise ~/.tt); a non-empty customPath is used verbatim and
// disables default-path migration.
func NewManager(customPath string) *Manager {
	if customPath != "" {
		return &Manager{configPath: customPath}
	}
	return &Manager{
		configPath:      DefaultConfigPath(),
		fromDefaultPath: true,
	}
}

// CreatedFromDefaultPath reports whether this manager resolved its config path
// from the active default rather than an explicit --config override. First-run
// migration must only run when this is true.
func (m *Manager) CreatedFromDefaultPath() bool {
	return m.fromDefaultPath
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

// Load loads the configuration. Persistence is the five decomposed domain files
// (workspace/planning/settings/automation.json under <data>/.tt/, plus the
// machine-local machine.json); the legacy monolithic .tt.json is read only as a
// migration fallback when no domain files exist yet.
//
// Automatic first-run migration into iCloud has been removed. New installs are
// local-first (see ActiveDefaultDataDir); an existing iCloud workspace is
// selected only when its config already exists, and relocating a workspace is
// now the explicit `tt workspace move` flow (config.MoveWorkspace).
func (m *Manager) Load() (*Config, error) {
	dataDir := m.workspaceDataDir()
	store := NewDomainFileStore(dataDir)
	if m.domainFilesExist(store) {
		cfg, err := m.recomposeFromDomains(store, dataDir)
		if err != nil {
			return nil, err
		}
		return m.finalize(cfg)
	}
	return m.loadLegacy()
}

// loadLegacy reads the pre-domain monolithic .tt.json config. It is retained
// only for migration: an install that predates the split stores is read here and
// re-persisted into the domain files on the next Save (finalize triggers a Save
// for version/filter/palette migrations). A missing file yields a fresh,
// defaulted config.
func (m *Manager) loadLegacy() (*Config, error) {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		cfg := NewConfig()
		// Root a fresh workspace at the data directory that owns this manager's
		// config path — the same directory a domain reload derives (and where Save
		// writes the domain files). For the default manager this equals
		// ActiveDefaultDataDir; for an explicit --config/--data-dir workspace it
		// keeps project paths, domain files, and the reload's derived DataDir
		// consistent instead of diverging to the home-relative default.
		cfg.DataDir = m.workspaceDataDir()
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

	return m.finalize(&config)
}

// finalize applies the shared post-load normalization: field defaults, range
// clamps, project validation/dedup, stale-reference pruning, and the version/
// filter/palette migrations. Both the domain-file load path and the legacy
// monolithic fallback call it so a config loaded either way is normalized
// identically. Migrations that must persist call m.Save, which now writes the
// domain files.
func (m *Manager) finalize(config *Config) (*Config, error) { //nolint:gocyclo
	// Ensure defaults for missing fields
	if config.DataDir == "" {
		config.DataDir = ActiveDefaultDataDir()
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
	clampIntPtrRange(config.ResumeCardLimit, "ResumeCardLimit", 1, ResumeCardLimitUpperBound)
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
	config.normalizeGlobalPinnedTags()
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
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to save config after filter migration: %w", err)
		}
	}

	// Migrate legacy flat palette/bg/cursor fields into ThemeDark/ThemeLight.
	if config.MigratePaletteToThemes() {
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to save config after palette theme migration: %w", err)
		}
	}

	// Prune stale data that accumulates over time.
	pruned := config.PruneExpiredAlertDismissals()
	pruned = config.PruneFocusDismissedIDs() || pruned
	if pruned {
		if err := m.Save(config); err != nil {
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
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to save config after version migration: %w", err)
		}
	}
	if config.Version == 1 {
		// v1 → v2: palette data already migrated into ThemeDark/ThemeLight by
		// MigratePaletteToThemes() above. Just bump the version and save.
		config.Version = 2
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to save config after v1→v2 version bump: %w", err)
		}
	}
	if config.Version == 2 {
		// v2 -> v3: notes_editor was renamed to external_editor by the raw
		// key migration before unmarshalling.
		config.Version = 3
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to save config after v2->v3 version bump: %w", err)
		}
	}

	return config, nil
}

// workspaceDataDir returns the workspace root (data directory) that owns this
// manager's config, derived from the config path. It is where the synced domain
// files live (under <data>/.tt/) and the DataDir a domain-loaded config carries
// so project paths derive correctly.
func (m *Manager) workspaceDataDir() string {
	return WorkspaceRootFromConfigPath(m.configPath)
}

// domainStore builds a DomainFileStore rooted at this manager's workspace data
// directory.
func (m *Manager) domainStore() *DomainFileStore {
	return NewDomainFileStore(m.workspaceDataDir())
}

// allDomainKinds is the fixed order of the five decomposed configuration domains
// the manager persists and restores together.
var allDomainKinds = []domainKind{domainCatalog, domainPlanning, domainSettings, domainAutomation, domainMachine}

// domainFilesExist reports whether any of the five domain artifacts already
// exist on disk. When true, Load reads the domain files; when false, Load falls
// back to the legacy monolithic .tt.json for one-time migration.
func (m *Manager) domainFilesExist(store *DomainFileStore) bool {
	for _, kind := range allDomainKinds {
		if _, err := os.Stat(store.pathFor(kind)); err == nil {
			return true
		}
	}
	return false
}

// domainFilePaths returns the on-disk paths of all five domain artifacts for
// backup/restore, in the fixed domain order.
func (m *Manager) domainFilePaths() []string {
	store := m.domainStore()
	paths := make([]string, 0, len(allDomainKinds))
	for _, kind := range allDomainKinds {
		paths = append(paths, store.pathFor(kind))
	}
	return paths
}

// recomposeFromDomains loads the five domain files and rebuilds the temporary
// in-memory Config shape existing callers still consume. DataDir is set to the
// workspace root before ApplyTo so cataloged project paths derive correctly; the
// retired custom-stage fields have no domain owner and stay at their NewConfig
// defaults.
func (m *Manager) recomposeFromDomains(store *DomainFileStore, dataDir string) (*Config, error) {
	catalog, err := store.LoadCatalog()
	if err != nil {
		return nil, err
	}
	planning, err := store.LoadPlanning()
	if err != nil {
		return nil, err
	}
	settings, err := store.LoadSettings()
	if err != nil {
		return nil, err
	}
	automation, err := store.LoadAutomation()
	if err != nil {
		return nil, err
	}
	machine, err := store.LoadMachine()
	if err != nil {
		return nil, err
	}

	cfg := NewConfig()
	cfg.DataDir = dataDir
	cfg.Projects = nil
	(&Domains{
		Catalog:    catalog,
		Planning:   planning,
		Settings:   settings,
		Automation: automation,
		Machine:    machine,
	}).ApplyTo(cfg)
	return cfg, nil
}

// Save persists the configuration by decomposing it into the five domain files
// through the DomainFileStore. It never writes the legacy monolithic .tt.json.
// Each domain write is locked, atomic, backed up with a .bak sidecar, and
// stamped with the workspace ID (adopted from an existing catalog or minted).
func (m *Manager) Save(config *Config) error {
	if config == nil {
		return fmt.Errorf("config is required")
	}
	store := m.domainStore()
	// Adopt an existing on-disk workspace ID (or mint one) so every domain
	// artifact this save writes agrees on the workspace identity.
	if _, err := store.LoadCatalog(); err != nil {
		return err
	}
	workspaceID := store.EnsureWorkspaceID()

	d := Decompose(config)
	d.Catalog.WorkspaceID = workspaceID
	if err := store.SaveCatalog(d.Catalog); err != nil {
		return err
	}
	if err := store.SavePlanning(d.Planning); err != nil {
		return err
	}
	if err := store.SaveSettings(d.Settings); err != nil {
		return err
	}
	if err := store.SaveAutomation(d.Automation); err != nil {
		return err
	}
	if err := store.SaveMachine(d.Machine); err != nil {
		return err
	}
	return nil
}

// DomainArtifact is one synced domain file's on-disk path and its
// envelope-encoded bytes, ready for a transactional writer in the cmd/tui layer
// to wrap in a storage.ArtifactChange (which config must not import). It is the
// unit produced by SyncedDomainArtifacts.
type DomainArtifact struct {
	Path string
	Data []byte
}

// SyncedDomainArtifacts renders the four synced configuration domains
// (workspace, planning, settings, automation) of cfg into their canonical
// envelope-encoded on-disk artifacts under <data>/.tt/, byte-compatible with
// what Save writes. It is the transactional counterpart of Save: a config
// mutation that must ride the same atomic commit as an accompanying task write
// is encoded here and committed as ArtifactChange entries, so no config mutation
// is lost on a domain-file reload.
//
// machine.json is deliberately excluded: it is machine-local, lives outside the
// workspace data dir and the commit service's approved root, and its references
// are repaired separately after a commit.
func (m *Manager) SyncedDomainArtifacts(cfg *Config) ([]DomainArtifact, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	dataDir := m.workspaceDataDir()
	store := m.domainStore()
	// Adopt the on-disk workspace ID (or mint one) so the transactional artifacts
	// carry the same identity Save stamps.
	if _, err := store.LoadCatalog(); err != nil {
		return nil, err
	}
	workspaceID := store.EnsureWorkspaceID()

	d := Decompose(cfg)
	d.Catalog.WorkspaceID = workspaceID
	specs := []struct {
		kind    domainKind
		path    string
		payload any
	}{
		{domainCatalog, WorkspaceCatalogPath(dataDir), d.Catalog},
		{domainPlanning, PlanningStatePath(dataDir), d.Planning},
		{domainSettings, SettingsPath(dataDir), d.Settings},
		{domainAutomation, AutomationPath(dataDir), d.Automation},
	}
	arts := make([]DomainArtifact, 0, len(specs))
	for _, s := range specs {
		data, err := encodeDomainEnvelope(s.kind, workspaceID, s.payload)
		if err != nil {
			return nil, err
		}
		arts = append(arts, DomainArtifact{Path: s.path, Data: data})
	}
	return arts, nil
}

// GetConfigPath returns the current config file path.
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// Exists reports whether persisted configuration exists — either the decomposed
// domain files or a legacy monolithic .tt.json awaiting migration. LoadOrCreate
// uses it to decide whether an initial Save is needed.
func (m *Manager) Exists() bool {
	if m.domainFilesExist(m.domainStore()) {
		return true
	}
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

// GetBackupPath returns the representative backup path for the config. Backups
// now span all five domain files; this returns the workspace catalog's .bak
// sidecar as the canonical representative (used by future-version load guards
// that assert no backup was created).
func (m *Manager) GetBackupPath() string {
	return WorkspaceCatalogPath(m.workspaceDataDir()) + ".bak"
}

// ConfigBackupExists reports whether any domain artifact has a .bak sidecar.
func (m *Manager) ConfigBackupExists() bool {
	for _, path := range m.domainFilePaths() {
		if _, err := os.Stat(path + ".bak"); err == nil {
			return true
		}
	}
	return false
}

// BackupConfig snapshots every existing domain artifact to its .bak sidecar,
// mirroring the per-file backup the DomainFileStore also writes on each save.
func (m *Manager) BackupConfig() error {
	for _, path := range m.domainFilePaths() {
		content, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to read %s for backup: %w", filepath.Base(path), err)
		}
		if err := util.AtomicWriteFile(path+".bak", content, FilePermissions); err != nil {
			return fmt.Errorf("failed to write backup for %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// RestoreConfigFromBackup swaps each domain artifact with its .bak sidecar. The
// swap keeps the operation reversible: running restore again undoes it. Domains
// without a backup are left untouched.
func (m *Manager) RestoreConfigFromBackup() error {
	if !m.ConfigBackupExists() {
		return fmt.Errorf("no config backup available")
	}
	for _, path := range m.domainFilePaths() {
		backupPath := path + ".bak"
		backupContent, err := os.ReadFile(backupPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to read backup for %s: %w", filepath.Base(path), err)
		}

		currentContent, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			currentContent = []byte{}
		} else if err != nil {
			return fmt.Errorf("failed to read current %s: %w", filepath.Base(path), err)
		}

		if err := util.AtomicWriteFile(path, backupContent, FilePermissions); err != nil {
			return fmt.Errorf("failed to restore %s: %w", filepath.Base(path), err)
		}
		if err := util.AtomicWriteFile(backupPath, currentContent, FilePermissions); err != nil {
			return fmt.Errorf("failed to update backup for %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
