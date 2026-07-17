package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/workflow"
)

// Configuration is split into five explicit persistence domains:
//
//   - WorkspaceCatalog -> <data>/.tt/workspace.json
//   - PlanningState    -> <data>/.tt/planning.json
//   - Settings         -> <data>/.tt/settings.json
//   - AutomationConfig -> <data>/.tt/automation.json
//   - MachineState     -> ~/.tt/runtime/<workspace-id>/machine.json
//
// Each aggregate owns its defaults, validation, project-reference cleanup, and
// persistence lifecycle. RuntimeState composes the five loaded domains for
// long-lived consumers, while mutations commit only the owning domain (or an
// explicit checked multi-domain transaction).

// CatalogProject is the persisted, portable description of a project inside the
// WorkspaceCatalog. It deliberately omits the absolute on-disk Path that the
// legacy models.Project carries: a project's location is derived from the data
// directory and its safe slug (see CatalogPath), never stored, so a workspace
// can be moved between machines or storage backends without rewriting paths.
type CatalogProject struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	StatusIcon       string     `json:"status_icon,omitempty"`
	StatusIconSource string     `json:"status_icon_source,omitempty"`
	Context          string     `json:"context"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Archived         bool       `json:"archived"`
	ArchivedAt       *time.Time `json:"archived_at,omitempty"`
	Pinned           bool       `json:"pinned"`
	PinnedAt         *time.Time `json:"pinned_at,omitempty"`
	Sandbox          bool       `json:"sandbox,omitempty"`
	Internal         bool       `json:"internal,omitempty"`
	ColorSlot        int        `json:"color_slot,omitempty"`
}

const (
	// ProjectColorSlotCount is the fixed number of durable project identity
	// colors. Slot zero represents a legacy or otherwise unassigned project.
	ProjectColorSlotCount = 48

	// SystemInboxName and SystemInboxSlug are the user-facing identity of the
	// workspace-owned capture project. The catalog reference, rather than the
	// display name or slug, is the durable identity used by callers.
	SystemInboxName = "Inbox"
	SystemInboxSlug = "inbox"

	// LegacyActionInboxSlug identifies the hidden predecessor migrated by the
	// TUI when it first opens a workspace created by an older release.
	LegacyActionInboxSlug = "action-inbox"
)

// CatalogPath derives a project's data directory from the workspace data
// directory and the project slug. Absolute paths are never persisted; this is
// the single derivation point for a catalog project's location.
func (p CatalogProject) CatalogPath(dataDir string) string {
	return filepath.Join(dataDir, "projects", p.Slug)
}

// catalogProjectFromModel snapshots a models.Project into its portable catalog
// form, dropping the absolute Path and all computed (unpersisted) fields.
func catalogProjectFromModel(p *models.Project) CatalogProject {
	if p == nil {
		return CatalogProject{}
	}
	return CatalogProject{
		ID:               p.ID,
		Slug:             p.Slug,
		Name:             p.Name,
		Description:      p.Description,
		StatusIcon:       p.StatusIcon,
		StatusIconSource: p.StatusIconSource,
		Context:          p.Context,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		Archived:         p.Archived,
		ArchivedAt:       p.ArchivedAt,
		Pinned:           p.Pinned,
		PinnedAt:         p.PinnedAt,
		Sandbox:          p.Sandbox,
		Internal:         p.Internal,
		ColorSlot:        p.ColorSlot,
	}
}

// ToModel rehydrates a catalog project into a models.Project, deriving Path
// from the supplied data directory. Computed fields (Tasks, Sections, etc.)
// are left zero for storage to populate on load.
func (p CatalogProject) ToModel(dataDir string) *models.Project {
	return &models.Project{
		ID:               p.ID,
		Slug:             p.Slug,
		Name:             p.Name,
		Description:      p.Description,
		StatusIcon:       p.StatusIcon,
		StatusIconSource: p.StatusIconSource,
		Context:          p.Context,
		Path:             p.CatalogPath(dataDir),
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		Archived:         p.Archived,
		ArchivedAt:       p.ArchivedAt,
		Pinned:           p.Pinned,
		PinnedAt:         p.PinnedAt,
		Sandbox:          p.Sandbox,
		Internal:         p.Internal,
		ColorSlot:        p.ColorSlot,
	}
}

// WorkspaceCatalog owns the workspace identity and the project catalog. It maps
// to <data>/.tt/workspace.json.
type WorkspaceCatalog struct {
	SchemaVersion        int              `json:"schema_version"`
	WorkspaceID          string           `json:"workspace_id,omitempty"`
	Projects             []CatalogProject `json:"projects"`
	SystemInboxProjectID string           `json:"system_inbox_project_id,omitempty"`
}

// IsSystemInboxProject reports whether id is the catalog-owned Inbox. Names and
// slugs are deliberately ignored so identity stays stable across presentation
// changes and legacy migrations.
func (w *WorkspaceCatalog) IsSystemInboxProject(id string) bool {
	return w != nil && id != "" && w.SystemInboxProjectID == id
}

// NewWorkspaceCatalog returns a catalog seeded with defaults.
func NewWorkspaceCatalog() *WorkspaceCatalog {
	return &WorkspaceCatalog{
		SchemaVersion: CurrentConfigVersion,
		Projects:      []CatalogProject{},
	}
}

// Validate reports structural problems with the catalog: a project must have a
// non-empty ID and canonical safe slug, IDs and slugs must be unique, and the
// system inbox reference must point at a cataloged project when set.
func (w *WorkspaceCatalog) Validate() error {
	return w.validate(false)
}

// validateForLoad permits invalid project color slots so callers can repair
// them with EnsureProjectColorSlots after the catalog's structural fields have
// been validated. Save paths continue to use Validate and reject invalid slots.
func (w *WorkspaceCatalog) validateForLoad() error {
	return w.validate(true)
}

func (w *WorkspaceCatalog) validate(allowRepairableColorSlots bool) error {
	if w.SchemaVersion <= 0 {
		return fmt.Errorf("workspace catalog: schema version must be positive, got %d", w.SchemaVersion)
	}
	seenID := make(map[string]bool, len(w.Projects))
	seenSlug := make(map[string]bool, len(w.Projects))
	for i, p := range w.Projects {
		if p.ID == "" {
			return fmt.Errorf("workspace catalog: project %d has empty id", i)
		}
		if err := models.ValidateProjectSlug(p.Slug, p.Sandbox); err != nil {
			return fmt.Errorf("workspace catalog: project %q has invalid slug: %w", p.ID, err)
		}
		if seenID[p.ID] {
			return fmt.Errorf("workspace catalog: duplicate project id %q", p.ID)
		}
		if seenSlug[p.Slug] {
			return fmt.Errorf("workspace catalog: duplicate project slug %q", p.Slug)
		}
		if !allowRepairableColorSlots && (p.ColorSlot < 0 || p.ColorSlot > ProjectColorSlotCount) {
			return fmt.Errorf("workspace catalog: project %q has invalid color slot %d", p.ID, p.ColorSlot)
		}
		seenID[p.ID] = true
		seenSlug[p.Slug] = true
	}
	if w.SystemInboxProjectID != "" && !seenID[w.SystemInboxProjectID] {
		return fmt.Errorf("workspace catalog: system inbox references unknown project %q", w.SystemInboxProjectID)
	}
	return nil
}

// EnsureProjectColorSlots assigns durable color slots to every eligible
// project that is missing one or carries an out-of-range value. Valid existing
// assignments are never changed, including duplicates. Assignment order is
// deterministic and independent of the catalog slice order.
func (w *WorkspaceCatalog) EnsureProjectColorSlots() bool {
	if w == nil {
		return false
	}

	usage := projectColorSlotUsage(w.Projects, -1)
	pending := make([]int, 0)
	changed := false
	for i := range w.Projects {
		project := &w.Projects[i]
		if projectColorEligible(*project) {
			if !validProjectColorSlot(project.ColorSlot) {
				pending = append(pending, i)
			}
			continue
		}
		if project.ColorSlot < 0 || project.ColorSlot > ProjectColorSlotCount {
			project.ColorSlot = 0
			changed = true
		}
	}

	sort.SliceStable(pending, func(i, j int) bool {
		left := w.Projects[pending[i]]
		right := w.Projects[pending[j]]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
	for _, index := range pending {
		slot := leastUsedProjectColorSlot(usage)
		w.Projects[index].ColorSlot = slot
		usage[slot]++
		changed = true
	}
	return changed
}

func projectColorEligible(project CatalogProject) bool {
	return !project.Sandbox && !project.Internal
}

func validProjectColorSlot(slot int) bool {
	return slot >= 1 && slot <= ProjectColorSlotCount
}

func projectColorSlotUsage(projects []CatalogProject, skipIndex int) [ProjectColorSlotCount + 1]int {
	var usage [ProjectColorSlotCount + 1]int
	for i, project := range projects {
		if i == skipIndex || !projectColorEligible(project) || !validProjectColorSlot(project.ColorSlot) {
			continue
		}
		usage[project.ColorSlot]++
	}
	return usage
}

func leastUsedProjectColorSlot(usage [ProjectColorSlotCount + 1]int) int {
	selected := 1
	for slot := 2; slot <= ProjectColorSlotCount; slot++ {
		if usage[slot] < usage[selected] {
			selected = slot
		}
	}
	return selected
}

func allProjectColorSlotsUsed(usage [ProjectColorSlotCount + 1]int) bool {
	for slot := 1; slot <= ProjectColorSlotCount; slot++ {
		if usage[slot] == 0 {
			return false
		}
	}
	return true
}

// RemoveProjectReferences drops the cataloged project and clears the system
// inbox reference when it targeted the removed project. It returns true when
// anything changed.
func (w *WorkspaceCatalog) RemoveProjectReferences(id, slug string) bool {
	changed := false
	kept := w.Projects[:0]
	for _, p := range w.Projects {
		if projectMatches(p.ID, p.Slug, id, slug) {
			changed = true
			continue
		}
		kept = append(kept, p)
	}
	w.Projects = kept
	if refMatches(w.SystemInboxProjectID, id, slug) {
		w.SystemInboxProjectID = ""
		changed = true
	}
	return changed
}

// UpsertProject adds a project to the catalog or, when one with the same ID
// already exists, replaces its snapshot in place. The project is stored in its
// portable catalog form (absolute Path and computed fields dropped), so a TUI
// that mutates catalog-owned flags on a rehydrated models.Project (Archive,
// Pin, rename) can write the change back through a checked catalog commit.
func (w *WorkspaceCatalog) UpsertProject(p *models.Project) {
	if w == nil || p == nil {
		return
	}
	w.EnsureProjectColorSlots()
	for i := range w.Projects {
		if w.Projects[i].ID == p.ID {
			storedSlot := w.Projects[i].ColorSlot
			if !validProjectColorSlot(storedSlot) && projectColorEligible(catalogProjectFromModel(p)) {
				usage := projectColorSlotUsage(w.Projects, i)
				storedSlot = leastUsedProjectColorSlot(usage)
			}
			if !validProjectColorSlot(storedSlot) {
				storedSlot = 0
			}
			p.ColorSlot = storedSlot
			cp := catalogProjectFromModel(p)
			w.Projects[i] = cp
			return
		}
	}

	cp := catalogProjectFromModel(p)
	if projectColorEligible(cp) {
		usage := projectColorSlotUsage(w.Projects, -1)
		if !validProjectColorSlot(cp.ColorSlot) || (usage[cp.ColorSlot] > 0 && !allProjectColorSlotsUsed(usage)) {
			cp.ColorSlot = leastUsedProjectColorSlot(usage)
		}
	} else if !validProjectColorSlot(cp.ColorSlot) {
		cp.ColorSlot = 0
	}
	p.ColorSlot = cp.ColorSlot
	w.Projects = append(w.Projects, cp)
}

// PlanningState owns the planning collections: focus queue, daily plan, review
// completion, dismissals, and saved task views/filters. It maps to
// <data>/.tt/planning.json.
type PlanningState struct {
	FocusQueue         []FocusQueueItem `json:"focus_queue,omitempty"`
	DailyPlan          []DailyPlanItem  `json:"daily_plan,omitempty"`
	FocusDismissedIDs  []string         `json:"focus_dismissed_ids,omitempty"`
	AlertDismissals    []AlertDismissal `json:"alert_dismissals,omitempty"`
	SavedViews         []savedView      `json:"saved_views,omitempty"`
	SavedFilters       []SavedFilter    `json:"saved_filters,omitempty"`
	SmartLists         []smartList      `json:"smart_lists,omitempty"`
	LastDailyReviewAt  *time.Time       `json:"last_daily_review_at,omitempty"`
	LastWeeklyReviewAt *time.Time       `json:"last_weekly_review_at,omitempty"`
	TodayClearedAt     string           `json:"today_cleared_at,omitempty"`
	StreakCount        int              `json:"streak_count,omitempty"`
	StreakLastDate     string           `json:"streak_last_date,omitempty"`
}

// NewPlanningState returns an empty planning state.
func NewPlanningState() *PlanningState {
	return &PlanningState{}
}

// Validate reports problems with the planning collections: focus queue and
// daily plan items must reference a task, and saved views/filters must be named.
func (p *PlanningState) Validate() error {
	for i, item := range p.FocusQueue {
		if item.TaskID == "" {
			return fmt.Errorf("planning: focus queue item %d has empty task id", i)
		}
	}
	for i, item := range p.DailyPlan {
		if item.TaskID == "" {
			return fmt.Errorf("planning: daily plan item %d has empty task id", i)
		}
	}
	for i, v := range p.SavedViews {
		if v.Name == "" {
			return fmt.Errorf("planning: saved view %d has empty name", i)
		}
	}
	for i, f := range p.SavedFilters {
		if f.Name == "" {
			return fmt.Errorf("planning: saved filter %d has empty name", i)
		}
	}
	return nil
}

// RemoveProjectReferences drops focus queue and daily plan entries that point at
// the removed project. It returns true when anything changed.
func (p *PlanningState) RemoveProjectReferences(id, slug string) bool {
	var fq, dp bool
	p.FocusQueue, fq = filterProjectRefSlice(p.FocusQueue, func(item FocusQueueItem) string {
		return item.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	p.DailyPlan, dp = filterProjectRefSlice(p.DailyPlan, func(item DailyPlanItem) string {
		return item.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	return fq || dp
}

// AIConsentSettings records the user's remembered consent to send task context
// to the AI assistant. Consent is opt-in and must be explicit (context.md:
// "AI receives task context only after explicit remembered consent").
type AIConsentSettings struct {
	Granted   bool                         `json:"granted"`
	GrantedAt *time.Time                   `json:"granted_at,omitempty"`
	Providers map[string]AIProviderConsent `json:"providers,omitempty"`
}

// CurrentAIConsentScopeVersion is the disclosure scope that covers all active
// project data and permits Codex to inspect the workspace read-only. Bumping it
// deliberately requires each provider to receive fresh consent.
const CurrentAIConsentScopeVersion = 1

// AIProviderConsent records one provider-specific remembered consent grant.
type AIProviderConsent struct {
	Granted      bool       `json:"granted"`
	GrantedAt    *time.Time `json:"granted_at,omitempty"`
	ScopeVersion int        `json:"scope_version,omitempty"`
}

// GrantedFor reports whether the selected harness has granted consent for the
// current disclosure scope. Legacy and unversioned grants are intentionally
// insufficient because they did not disclose active-workspace context or
// Codex's read-only workspace inspection capability.
func (s AIConsentSettings) GrantedFor(name aiharness.Name) bool {
	if !name.Valid() {
		return false
	}
	grant := s.Providers[string(name)]
	return grant.Granted && grant.ScopeVersion == CurrentAIConsentScopeVersion
}

// GrantFor remembers task-context consent for one harness at grantedAt.
// Claude also updates the legacy fields so older tt versions retain that grant.
func (s *AIConsentSettings) GrantFor(name aiharness.Name, grantedAt time.Time) error {
	if s == nil {
		return fmt.Errorf("AI consent settings are required")
	}
	if !name.Valid() {
		return fmt.Errorf("grant AI consent: %w %q", aiharness.ErrUnknownHarness, name)
	}
	grantedAt = grantedAt.UTC()
	if s.Providers == nil {
		s.Providers = make(map[string]AIProviderConsent)
	}
	timestamp := grantedAt
	s.Providers[string(name)] = AIProviderConsent{
		Granted:      true,
		GrantedAt:    &timestamp,
		ScopeVersion: CurrentAIConsentScopeVersion,
	}
	if name == aiharness.Claude {
		legacyTimestamp := grantedAt
		s.Granted = true
		s.GrantedAt = &legacyTimestamp
	}
	return nil
}

// Validate rejects provider-map keys that are not part of the supported
// allowlist. It does not require grants for every provider.
func (s AIConsentSettings) Validate() error {
	for rawName := range s.Providers {
		if _, err := aiharness.Parse(rawName); err != nil {
			return fmt.Errorf("settings: ai_consent provider: %w", err)
		}
	}
	return nil
}

// OnboardingProgress records the five durable first-run milestones. Each flag
// is monotonic: once observed it remains complete even if the corresponding
// task or project is later removed.
type OnboardingProgress struct {
	ProjectCreated bool `json:"project_created,omitempty"`
	TaskCaptured   bool `json:"task_captured,omitempty"`
	InboxTriaged   bool `json:"inbox_triaged,omitempty"`
	TaskPlanned    bool `json:"task_planned,omitempty"`
	TaskCompleted  bool `json:"task_completed,omitempty"`
}

// CompletedCount returns the number of finished onboarding milestones.
func (p OnboardingProgress) CompletedCount() int {
	count := 0
	for _, done := range []bool{p.ProjectCreated, p.TaskCaptured, p.InboxTriaged, p.TaskPlanned, p.TaskCompleted} {
		if done {
			count++
		}
	}
	return count
}

// Complete reports whether every onboarding milestone has been observed.
func (p OnboardingProgress) Complete() bool { return p.CompletedCount() == 5 }

// Settings owns portable behavior, appearance, alert, and privacy/AI-consent
// preferences. It maps to <data>/.tt/settings.json.
type Settings struct {
	// Behavior.
	DefaultContext        string              `json:"default_context,omitempty"`
	FzfEnabled            bool                `json:"fzf_enabled"`
	GlowEnabled           bool                `json:"glow_enabled"`
	FzfTimeout            *int                `json:"fzf_timeout,omitempty"`
	UndoHistoryDepth      *int                `json:"undo_history_depth,omitempty"`
	PopupTimeoutSeconds   *int                `json:"popup_timeout_seconds,omitempty"`
	TimeFormat            string              `json:"time_format,omitempty"`
	WeekEndDay            string              `json:"week_end_day,omitempty"`
	UseEmoji              *bool               `json:"use_emoji,omitempty"`
	LinkedTaskJumpToNew   *bool               `json:"linked_task_jump_to_new,omitempty"`
	TutorDismissed        *bool               `json:"tutor_dismissed,omitempty"`
	Onboarding            *OnboardingProgress `json:"onboarding,omitempty"`
	DefaultProjectView    string              `json:"default_project_view,omitempty"`
	FocusBoardDefaultMode string              `json:"focus_board_default_mode,omitempty"`
	GlobalPinnedTags      []string            `json:"global_pinned_tags,omitempty"`
	SectionColors         map[string]string   `json:"section_colors,omitempty"`

	// Behavior — activity, list, and pin limits.
	MaxActivityEntries    *int  `json:"max_activity_entries,omitempty"`
	RecentActivityLimit   *int  `json:"recent_activity_limit,omitempty"`
	RecentActivityShowAll *bool `json:"recent_activity_show_all,omitempty"`
	MaxPinnedBarItems     *int  `json:"max_pinned_bar_items,omitempty"`
	MaxRecentTaskVisits   *int  `json:"max_recent_task_visits,omitempty"`
	ResumeCardLimit       *int  `json:"resume_card_limit,omitempty"`

	// Behavior — review windows.
	ReviewDueSoonDays    *int `json:"review_due_soon_days,omitempty"`
	ReviewFutureDays     *int `json:"review_future_days,omitempty"`
	ReviewOldDays        *int `json:"review_old_days,omitempty"`
	ReviewCompletedHours *int `json:"review_completed_hours,omitempty"`

	// Behavior — weekly/stale thresholds.
	WeeklyStaleProjectDays    *int `json:"weekly_stale_project_days,omitempty"`
	WeeklyStaleTaskDays       *int `json:"weekly_stale_task_days,omitempty"`
	StaleTaskDays             *int `json:"stale_task_days,omitempty"`
	PriorityEscalationDays    *int `json:"priority_escalation_days,omitempty"`
	WeeklyInboxThreshold      *int `json:"weekly_inbox_threshold,omitempty"`
	WeeklyBlockedThreshold    *int `json:"weekly_blocked_threshold,omitempty"`
	WeeklyInProgressThreshold *int `json:"weekly_in_progress_threshold,omitempty"`

	// Behavior — GTD section WIP limits.
	SectionLimitActive  *int `json:"section_limit_active,omitempty"`
	SectionLimitNext    *int `json:"section_limit_next,omitempty"`
	SectionLimitInbox   *int `json:"section_limit_inbox,omitempty"`
	SectionLimitStandby *int `json:"section_limit_standby,omitempty"`
	SectionLimitWaiting *int `json:"section_limit_waiting,omitempty"`
	SectionLimitSomeday *int `json:"section_limit_someday,omitempty"`
	SectionLimitReview  *int `json:"section_limit_review,omitempty"`

	// Appearance.
	ActiveTheme        string         `json:"active_theme,omitempty"`
	AutoTheme          *bool          `json:"auto_theme,omitempty"`
	ThemeDark          Theme          `json:"theme_dark,omitzero"`
	ThemeDarkName      string         `json:"theme_dark_name,omitempty"`
	ThemeDarkRev       int            `json:"theme_dark_rev,omitempty"`
	ThemeLight         Theme          `json:"theme_light,omitzero"`
	ThemeLightName     string         `json:"theme_light_name,omitempty"`
	ThemeLightRev      int            `json:"theme_light_rev,omitempty"`
	Palette            PaletteColors  `json:"palette,omitzero"`
	KanbanSettings     KanbanSettings `json:"kanban_settings,omitzero"`
	TUIBackgroundColor string         `json:"tui_background_color,omitempty"`
	TUICursorColor     string         `json:"tui_cursor_color,omitempty"`
	TUICursorTextColor string         `json:"tui_cursor_text_color,omitempty"`
	TUIZebraStriping   *bool          `json:"tui_zebra_striping,omitempty"`

	// Alerts.
	AlertDailyReviewEnabled   *bool `json:"alert_daily_review_enabled,omitempty"`
	AlertWeeklyReviewEnabled  *bool `json:"alert_weekly_review_enabled,omitempty"`
	AlertOverdueEnabled       *bool `json:"alert_overdue_enabled,omitempty"`
	AlertDueSoonEnabled       *bool `json:"alert_due_soon_enabled,omitempty"`
	AlertStaleEnabled         *bool `json:"alert_stale_enabled,omitempty"`
	AlertInboxEnabled         *bool `json:"alert_inbox_enabled,omitempty"`
	AlertBlockedEnabled       *bool `json:"alert_blocked_enabled,omitempty"`
	AlertProjectHealthEnabled *bool `json:"alert_project_health_enabled,omitempty"`
	AlertDueSoonDays          *int  `json:"alert_due_soon_days,omitempty"`
	AlertStaleDays            *int  `json:"alert_stale_days,omitempty"`
	AlertInboxThreshold       *int  `json:"alert_inbox_threshold,omitempty"`
	AlertBlockedDays          *int  `json:"alert_blocked_days,omitempty"`
	AlertProjectStaleDays     *int  `json:"alert_project_stale_days,omitempty"`
	AlertDailyReviewHour      *int  `json:"alert_daily_review_hour,omitempty"`
	AlertWeeklyReviewDay      *int  `json:"alert_weekly_review_day,omitempty"`

	// Privacy / AI consent.
	AIConsent AIConsentSettings `json:"ai_consent,omitzero"`
}

// NewSettings returns settings seeded with the same portable defaults NewConfig
// applies.
func NewSettings() *Settings {
	depth := DefaultUndoHistoryDepth
	return &Settings{
		DefaultContext:   "personal",
		FzfEnabled:       true,
		GlowEnabled:      true,
		UndoHistoryDepth: &depth,
		KanbanSettings: KanbanSettings{
			DefaultMode:        defaultKanbanMode,
			DefaultStartColumn: defaultKanbanStartColumn,
		},
	}
}

// Validate reports out-of-range portable settings. Pointer fields that are nil
// fall back to defaults and are always valid.
func (s *Settings) Validate() error {
	if s.FzfTimeout != nil && *s.FzfTimeout < 0 {
		return fmt.Errorf("settings: fzf_timeout must be >= 0, got %d", *s.FzfTimeout)
	}
	if s.UndoHistoryDepth != nil && *s.UndoHistoryDepth < 0 {
		return fmt.Errorf("settings: undo_history_depth must be >= 0, got %d", *s.UndoHistoryDepth)
	}
	if err := s.AIConsent.Validate(); err != nil {
		return err
	}
	return nil
}

// AutomationConfig owns workflow rules, task templates, and scoring
// configuration. It maps to <data>/.tt/automation.json.
type AutomationConfig struct {
	WorkflowRules           []workflow.WorkflowRule      `json:"workflow_rules,omitempty"`
	DefaultRulesInitialized bool                         `json:"default_rules_initialized,omitempty"`
	WorkflowEvalStates      map[string]WorkflowEvalState `json:"workflow_eval_states,omitempty"`
	WorkflowEvalInterval    *int                         `json:"workflow_eval_interval,omitempty"`
	TaskTemplates           []template.TaskTemplate      `json:"task_templates,omitempty"`
	DefaultTemplatesInit    bool                         `json:"default_templates_init,omitempty"`
	ScoringProfiles         []scoring.ScoringProfile     `json:"scoring_profiles,omitempty"`
	ProjectScoringProfile   map[string]string            `json:"project_scoring_profile,omitempty"`
}

// NewAutomationConfig returns an empty automation config.
func NewAutomationConfig() *AutomationConfig {
	return &AutomationConfig{}
}

// Validate reports problems with automation entries: workflow rules and scoring
// profiles must be named.
func (a *AutomationConfig) Validate() error {
	for i, r := range a.WorkflowRules {
		if r.Name == "" {
			return fmt.Errorf("automation: workflow rule %d has empty name", i)
		}
	}
	for i, p := range a.ScoringProfiles {
		if p.Name == "" {
			return fmt.Errorf("automation: scoring profile %d has empty name", i)
		}
	}
	return nil
}

// RemoveProjectReferences drops per-project scoring assignments and workflow
// evaluation state for the removed project. It returns true when anything
// changed.
func (a *AutomationConfig) RemoveProjectReferences(id, slug string) bool {
	changed := mapHadProjectKey(a.ProjectScoringProfile, id, slug)
	changed = mapHadProjectKey(a.WorkflowEvalStates, id, slug) || changed
	deleteProjectKey(a.ProjectScoringProfile, id, slug)
	deleteProjectKey(a.WorkflowEvalStates, id, slug)
	return changed
}

// MachineState owns machine-specific preferences that should not follow a
// workspace between machines: editors, navigation, histories, and layout. It
// maps to ~/.tt/runtime/<workspace-id>/machine.json. The durable catalog UUID,
// rather than the current data-directory path, selects its machine-local state.
type MachineState struct {
	// Editors and integrations.
	Editor           string            `json:"editor,omitempty"`
	AttachmentEditor string            `json:"attachment_editor,omitempty"`
	CodeEditor       string            `json:"code_editor,omitempty"`
	ExternalEditor   string            `json:"external_editor,omitempty"`
	AIHarness        string            `json:"ai_harness,omitempty"`
	AIConfigDirs     map[string]string `json:"ai_config_dirs,omitempty"`

	// Current navigation and active (non-saved) filter selections.
	CurrentProject       string              `json:"current_project,omitempty"`
	CycleProjects        []string            `json:"cycle_projects,omitempty"`
	ContextCycleProjects map[string][]string `json:"context_cycle_projects,omitempty"`
	CustomContexts       map[string][]string `json:"custom_contexts,omitempty"`
	RecentTaskVisits     []RecentTaskVisit   `json:"recent_task_visits,omitempty"`
	StartView            string              `json:"start_view,omitempty"`
	FilterContext        string              `json:"filter_context,omitempty"`
	FilterCustomContext  string              `json:"filter_custom_context,omitempty"`
	ScoreFilterCustom    *float64            `json:"score_filter_custom,omitempty"`

	// Histories.
	RecentPaletteItems   []RecentPaletteItem `json:"recent_palette_items,omitempty"`
	LastPaletteCategory  string              `json:"last_palette_category,omitempty"`
	PaletteSearchHistory []string            `json:"palette_search_history,omitempty"`
	GlobalSearchHistory  []string            `json:"global_search_history,omitempty"`
	CommandHistory       []string            `json:"command_history,omitempty"`
	PinnedCommands       []string            `json:"pinned_commands,omitempty"`
	SavedSearches        map[string]string   `json:"saved_searches,omitempty"`

	// Layout — dashboard, tree, outliner, and cross-outliner view state.
	ProjectViewStates           map[string]ProjectViewState `json:"project_view_states,omitempty"`
	ShowInfoPanel               *bool                       `json:"show_info_panel,omitempty"`
	InfoPanelMode               *int                        `json:"info_panel_mode,omitempty"`
	DashboardBottomLayout       *int                        `json:"dashboard_bottom_layout,omitempty"`
	DashboardHorizontalSplitPct *int                        `json:"dashboard_horizontal_split_pct,omitempty"`
	DashboardVerticalSplitPct   *int                        `json:"dashboard_vertical_split_pct,omitempty"`
	OutlinerCollapsedSections   map[string]bool             `json:"outliner_collapsed_sections,omitempty"`
	TreeDefaultColumns          map[string]bool             `json:"tree_default_columns,omitempty"`
	OutlinerDefaultColumns      map[string]bool             `json:"outliner_default_columns,omitempty"`
	TreePriorityMode            string                      `json:"tree_priority_mode,omitempty"`
	OutlinerPriorityMode        string                      `json:"outliner_priority_mode,omitempty"`
	AllTasksTabOrder            []string                    `json:"all_tasks_tab_order,omitempty"`
	TaskFormCollapsed           *bool                       `json:"task_form_collapsed,omitempty"`

	// Layout — cross-outliner sidebar preferences and active filter/sort state.
	// (Saved views/filters are portable planning state; these are the machine's
	// current, in-flight cross-outliner selections.)
	CrossOutlinerSidebarEnabled    *bool             `json:"cross_outliner_sidebar_enabled,omitempty"`
	CrossOutlinerCycleOnly         *bool             `json:"cross_outliner_cycle_only,omitempty"`
	CrossOutlinerCollapsedSections map[string]bool   `json:"cross_outliner_collapsed_sections,omitempty"`
	CrossOutlinerFoldState         map[string]bool   `json:"cross_outliner_fold_state,omitempty"`
	CrossOutlinerColumnToggles     map[string]bool   `json:"cross_outliner_column_toggles,omitempty"`
	CrossOutlinerFilterType        string            `json:"cross_outliner_filter_type,omitempty"`
	CrossOutlinerFilterStatus      string            `json:"cross_outliner_filter_status,omitempty"`
	CrossOutlinerFilterPriority    int               `json:"cross_outliner_filter_priority,omitempty"`
	CrossOutlinerFilterDueDate     int               `json:"cross_outliner_filter_due_date,omitempty"`
	CrossOutlinerFilterActionable  bool              `json:"cross_outliner_filter_actionable,omitempty"`
	CrossOutlinerFilterHideDimmed  bool              `json:"cross_outliner_filter_hide_dimmed,omitempty"`
	CrossOutlinerFilterGitHubIssue bool              `json:"cross_outliner_filter_github_issue,omitempty"`
	CrossOutlinerFilterParked      bool              `json:"cross_outliner_filter_parked,omitempty"`
	CrossOutlinerFilterScoreMin    float64           `json:"cross_outliner_filter_score_min,omitempty"`
	CrossOutlinerFilterTags        []string          `json:"cross_outliner_filter_tags,omitempty"`
	CrossOutlinerFilterSections    []string          `json:"cross_outliner_filter_sections,omitempty"`
	CrossOutlinerFilterAttrs       map[string]string `json:"cross_outliner_filter_attrs,omitempty"`
	CrossOutlinerPinnedTagTab      string            `json:"cross_outliner_pinned_tag_tab,omitempty"`
	CrossOutlinerSortField         int               `json:"cross_outliner_sort_field,omitempty"`
	CrossOutlinerSortAscending     bool              `json:"cross_outliner_sort_ascending,omitempty"`
	CrossOutlinerFilterBarEnabled  bool              `json:"cross_outliner_filter_bar_enabled,omitempty"`
}

// NewMachineState returns machine state seeded with the same editor default
// NewConfig applies.
func NewMachineState() *MachineState {
	return &MachineState{
		Editor: "code",
	}
}

// Validate reports problems with machine-local preferences. Config-directory
// existence is intentionally checked only when an explicit override is used,
// so temporarily unavailable persisted directories remain loadable.
func (m *MachineState) Validate() error {
	if m.DashboardBottomLayout != nil && (*m.DashboardBottomLayout < 0 || *m.DashboardBottomLayout > 2) {
		return fmt.Errorf("machine: dashboard_bottom_layout must be 0..2, got %d", *m.DashboardBottomLayout)
	}
	if m.AIHarness != "" {
		if _, err := aiharness.Parse(m.AIHarness); err != nil {
			return fmt.Errorf("machine: ai_harness: %w", err)
		}
	}
	for rawName, path := range m.AIConfigDirs {
		if _, err := aiharness.Parse(rawName); err != nil {
			return fmt.Errorf("machine: ai_config_dirs: %w", err)
		}
		if path == "" {
			return fmt.Errorf("machine: ai_config_dirs[%q] must contain an explicit path", rawName)
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("machine: ai_config_dirs[%q] must contain an absolute path", rawName)
		}
	}
	return nil
}

// RemoveProjectReferences clears navigation, view-state, and recent-visit
// references to the removed project. It returns true when anything changed.
func (m *MachineState) RemoveProjectReferences(id, slug string) bool {
	changed := false
	if refMatches(m.CurrentProject, id, slug) {
		m.CurrentProject = ""
		changed = true
	}
	if mapHadProjectKey(m.ProjectViewStates, id, slug) {
		deleteProjectKey(m.ProjectViewStates, id, slug)
		changed = true
	}
	var visits bool
	m.RecentTaskVisits, visits = filterProjectRefSlice(m.RecentTaskVisits, func(v RecentTaskVisit) string {
		return v.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	changed = visits || changed

	var cycled bool
	m.CycleProjects, cycled = filterProjectIDs(m.CycleProjects, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	changed = cycled || changed
	changed = filterProjectIDMap(m.ContextCycleProjects, func(pid string) bool {
		return !refMatches(pid, id, slug)
	}) || changed
	changed = filterProjectIDMap(m.CustomContexts, func(pid string) bool {
		return !refMatches(pid, id, slug)
	}) || changed
	return changed
}

// refMatches reports whether a stored project reference targets the identified
// project, matching on either the ID or (when non-empty) the slug.
func refMatches(ref, id, slug string) bool {
	if ref == "" {
		return false
	}
	return ref == id || (slug != "" && ref == slug)
}

// projectMatches reports whether a catalog project (identified by its own id and
// slug) is the one being removed.
func projectMatches(projID, projSlug, id, slug string) bool {
	return refMatches(projID, id, slug) || refMatches(projSlug, id, slug)
}

// mapHadProjectKey reports whether a project-keyed map contains an entry for the
// removed project before deletion, so callers can report change accurately.
func mapHadProjectKey[T any](values map[string]T, id, slug string) bool {
	if _, ok := values[id]; ok {
		return true
	}
	if slug != "" {
		if _, ok := values[slug]; ok {
			return true
		}
	}
	return false
}
