package config

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// This file is the mechanically verified proof that the Config -> domain
// decomposition (Decompose / ApplyTo in domain_store.go) covers EVERY field of
// the monolithic Config facade with an explicit owner and a lossless round trip
// — the precondition for retiring the facade in the runtime-flip child (33.7.8).
//
// configFieldOwner is the migration map. TestConfigFieldOwnershipExhaustive
// fails if any Config field is missing (or any listed field disappears), so a
// future field cannot be silently dropped. TestDecomposeApplyToRoundTripsEveryField
// then proves every "owned" field survives Decompose->ApplyTo unchanged, while
// every "retired" field is intentionally dropped and the "container" field is
// left to the runtime.

const (
	ownerCatalog    = "catalog"
	ownerPlanning   = "planning"
	ownerSettings   = "settings"
	ownerAutomation = "automation"
	ownerMachine    = "machine"
	// ownerContainer marks the workspace-location container (DataDir): resolved
	// at runtime from flags/pointer, never persisted inside a domain file.
	ownerContainer = "container"
	// ownerRetired marks fields whose concept has been retired (custom stages).
	// They are captured by no domain and dropped on a domain round trip; their
	// legacy values are surfaced in the migration audit report, not carried
	// forward. See context.md ("Custom stage configuration is recorded in that
	// report and then removed").
	ownerRetired = "retired"
)

// configFieldOwner declares the owning domain for every Config field. Keep it in
// lockstep with the Config struct: TestConfigFieldOwnershipExhaustive enforces
// bidirectional completeness.
var configFieldOwner = map[string]string{
	// Workspace catalog (workspace.json).
	"Version":              ownerCatalog,
	"Projects":             ownerCatalog,
	"ActionInboxProjectID": ownerCatalog,

	// Runtime container (not domain-owned).
	"DataDir": ownerContainer,

	// Retired custom-stage vocabulary (dropped).
	"Stages":                   ownerRetired,
	"CrossOutlinerFilterStage": ownerRetired,

	// Planning (planning.json).
	"FocusQueue":         ownerPlanning,
	"DailyPlan":          ownerPlanning,
	"FocusDismissedIDs":  ownerPlanning,
	"AlertDismissals":    ownerPlanning,
	"SavedViews":         ownerPlanning,
	"SavedFilters":       ownerPlanning,
	"SmartLists":         ownerPlanning,
	"LastDailyReviewAt":  ownerPlanning,
	"LastWeeklyReviewAt": ownerPlanning,
	"TodayClearedAt":     ownerPlanning,
	"StreakCount":        ownerPlanning,
	"StreakLastDate":     ownerPlanning,

	// Automation (automation.json).
	"WorkflowRules":           ownerAutomation,
	"DefaultRulesInitialized": ownerAutomation,
	"WorkflowEvalStates":      ownerAutomation,
	"WorkflowEvalInterval":    ownerAutomation,
	"TaskTemplates":           ownerAutomation,
	"DefaultTemplatesInit":    ownerAutomation,
	"ScoringProfiles":         ownerAutomation,
	"ProjectScoringProfile":   ownerAutomation,

	// Settings (settings.json) — portable behavior, appearance, alerts, privacy.
	"DefaultContext":            ownerSettings,
	"FzfEnabled":                ownerSettings,
	"GlowEnabled":               ownerSettings,
	"FzfTimeout":                ownerSettings,
	"UndoHistoryDepth":          ownerSettings,
	"PopupTimeoutSeconds":       ownerSettings,
	"TimeFormat":                ownerSettings,
	"WeekEndDay":                ownerSettings,
	"UseEmoji":                  ownerSettings,
	"LinkedTaskJumpToNew":       ownerSettings,
	"TutorDismissed":            ownerSettings,
	"DefaultProjectView":        ownerSettings,
	"FocusBoardDefaultMode":     ownerSettings,
	"GlobalPinnedTags":          ownerSettings,
	"SectionColors":             ownerSettings,
	"MaxActivityEntries":        ownerSettings,
	"RecentActivityLimit":       ownerSettings,
	"RecentActivityShowAll":     ownerSettings,
	"MaxPinnedBarItems":         ownerSettings,
	"MaxRecentTaskVisits":       ownerSettings,
	"ResumeCardLimit":           ownerSettings,
	"ReviewDueSoonDays":         ownerSettings,
	"ReviewFutureDays":          ownerSettings,
	"ReviewOldDays":             ownerSettings,
	"ReviewCompletedHours":      ownerSettings,
	"WeeklyStaleProjectDays":    ownerSettings,
	"WeeklyStaleTaskDays":       ownerSettings,
	"StaleTaskDays":             ownerSettings,
	"PriorityEscalationDays":    ownerSettings,
	"WeeklyInboxThreshold":      ownerSettings,
	"WeeklyBlockedThreshold":    ownerSettings,
	"WeeklyInProgressThreshold": ownerSettings,
	"SectionLimitActive":        ownerSettings,
	"SectionLimitNext":          ownerSettings,
	"SectionLimitInbox":         ownerSettings,
	"SectionLimitStandby":       ownerSettings,
	"SectionLimitWaiting":       ownerSettings,
	"SectionLimitSomeday":       ownerSettings,
	"SectionLimitReview":        ownerSettings,
	"ActiveTheme":               ownerSettings,
	"AutoTheme":                 ownerSettings,
	"ThemeDark":                 ownerSettings,
	"ThemeDarkName":             ownerSettings,
	"ThemeDarkRev":              ownerSettings,
	"ThemeLight":                ownerSettings,
	"ThemeLightName":            ownerSettings,
	"ThemeLightRev":             ownerSettings,
	"Palette":                   ownerSettings,
	"KanbanSettings":            ownerSettings,
	"TUIBackgroundColor":        ownerSettings,
	"TUICursorColor":            ownerSettings,
	"TUICursorTextColor":        ownerSettings,
	"TUIZebraStriping":          ownerSettings,
	"AlertDailyReviewEnabled":   ownerSettings,
	"AlertWeeklyReviewEnabled":  ownerSettings,
	"AlertOverdueEnabled":       ownerSettings,
	"AlertDueSoonEnabled":       ownerSettings,
	"AlertStaleEnabled":         ownerSettings,
	"AlertInboxEnabled":         ownerSettings,
	"AlertBlockedEnabled":       ownerSettings,
	"AlertProjectHealthEnabled": ownerSettings,
	"AlertDueSoonDays":          ownerSettings,
	"AlertStaleDays":            ownerSettings,
	"AlertInboxThreshold":       ownerSettings,
	"AlertBlockedDays":          ownerSettings,
	"AlertProjectStaleDays":     ownerSettings,
	"AlertDailyReviewHour":      ownerSettings,
	"AlertWeeklyReviewDay":      ownerSettings,

	// Machine (machine.json) — editors, current navigation, histories, layout,
	// and active (non-saved) filter/sort selections.
	"CurrentProject":                 ownerMachine,
	"Editor":                         ownerMachine,
	"AttachmentEditor":               ownerMachine,
	"CodeEditor":                     ownerMachine,
	"ExternalEditor":                 ownerMachine,
	"CycleProjects":                  ownerMachine,
	"ContextCycleProjects":           ownerMachine,
	"CustomContexts":                 ownerMachine,
	"RecentTaskVisits":               ownerMachine,
	"StartView":                      ownerMachine,
	"FilterContext":                  ownerMachine,
	"FilterCustomContext":            ownerMachine,
	"ScoreFilterCustom":              ownerMachine,
	"RecentPaletteItems":             ownerMachine,
	"LastPaletteCategory":            ownerMachine,
	"PaletteSearchHistory":           ownerMachine,
	"GlobalSearchHistory":            ownerMachine,
	"CommandHistory":                 ownerMachine,
	"PinnedCommands":                 ownerMachine,
	"SavedSearches":                  ownerMachine,
	"ProjectViewStates":              ownerMachine,
	"ShowInfoPanel":                  ownerMachine,
	"InfoPanelMode":                  ownerMachine,
	"DashboardBottomLayout":          ownerMachine,
	"DashboardHorizontalSplitPct":    ownerMachine,
	"DashboardVerticalSplitPct":      ownerMachine,
	"OutlinerCollapsedSections":      ownerMachine,
	"TreeDefaultColumns":             ownerMachine,
	"OutlinerDefaultColumns":         ownerMachine,
	"TreePriorityMode":               ownerMachine,
	"OutlinerPriorityMode":           ownerMachine,
	"AllTasksTabOrder":               ownerMachine,
	"TaskFormCollapsed":              ownerMachine,
	"CrossOutlinerSidebarEnabled":    ownerMachine,
	"CrossOutlinerCycleOnly":         ownerMachine,
	"CrossOutlinerCollapsedSections": ownerMachine,
	"CrossOutlinerFoldState":         ownerMachine,
	"CrossOutlinerColumnToggles":     ownerMachine,
	"CrossOutlinerFilterType":        ownerMachine,
	"CrossOutlinerFilterStatus":      ownerMachine,
	"CrossOutlinerFilterPriority":    ownerMachine,
	"CrossOutlinerFilterDueDate":     ownerMachine,
	"CrossOutlinerFilterActionable":  ownerMachine,
	"CrossOutlinerFilterHideDimmed":  ownerMachine,
	"CrossOutlinerFilterGitHubIssue": ownerMachine,
	"CrossOutlinerFilterParked":      ownerMachine,
	"CrossOutlinerFilterScoreMin":    ownerMachine,
	"CrossOutlinerFilterTags":        ownerMachine,
	"CrossOutlinerFilterSections":    ownerMachine,
	"CrossOutlinerFilterAttrs":       ownerMachine,
	"CrossOutlinerPinnedTagTab":      ownerMachine,
	"CrossOutlinerSortField":         ownerMachine,
	"CrossOutlinerSortAscending":     ownerMachine,
	"CrossOutlinerFilterBarEnabled":  ownerMachine,
}

// TestConfigFieldOwnershipExhaustive proves configFieldOwner and the Config
// struct are in exact bijection: every field has a declared owner, and no owner
// entry names a nonexistent field. Adding a Config field without wiring it here
// (and into Decompose/ApplyTo) fails the build's coverage gate.
func TestConfigFieldOwnershipExhaustive(t *testing.T) {
	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if _, ok := configFieldOwner[name]; !ok {
			t.Errorf("Config field %q has no declared domain owner; add it to configFieldOwner and wire Decompose/ApplyTo (or mark it retired)", name)
		}
	}
	for name := range configFieldOwner {
		if _, ok := typ.FieldByName(name); !ok {
			t.Errorf("configFieldOwner lists %q, which is not a Config field", name)
		}
	}
}

// coverageSentinelTime is a fixed non-zero timestamp used to populate time.Time
// fields (Date.Now-free so the test is deterministic).
var coverageSentinelTime = time.Date(2026, time.July, 12, 9, 30, 0, 0, time.UTC)

// fillNonZero recursively sets v to a deterministic non-zero value so that, after
// a Decompose->ApplyTo round trip, a dropped (unwired) field is observable: the
// output keeps the zero value while the input carried a non-zero one. The depth
// guard keeps potentially self-referential nested types (workflow/scoring/
// template structs) from recursing forever; leaf values beyond the cap stay
// zero, which is harmless because reference-type fields are copied by reference
// and thus round-trip regardless of their contents.
func fillNonZero(v reflect.Value, depth int) {
	if !v.CanSet() {
		return
	}
	if v.Type() == reflect.TypeOf(time.Time{}) {
		v.Set(reflect.ValueOf(coverageSentinelTime))
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Ptr:
		elem := reflect.New(v.Type().Elem())
		if depth < 6 {
			fillNonZero(elem.Elem(), depth+1)
		}
		v.Set(elem)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		if depth < 6 {
			fillNonZero(s.Index(0), depth+1)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		val := reflect.New(v.Type().Elem()).Elem()
		if depth < 6 {
			fillNonZero(key, depth+1)
			fillNonZero(val, depth+1)
		}
		if key.Kind() == reflect.String && key.Len() == 0 {
			key.SetString("k")
		}
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.Struct:
		if depth >= 6 {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			fillNonZero(v.Field(i), depth+1)
		}
	}
}

// fullyPopulatedConfig returns a Config whose every field (except the specially
// handled Projects/DataDir) is set to a non-zero sentinel.
func fullyPopulatedConfig(dataDir string, proj *models.Project) *Config {
	cfg := &Config{}
	rv := reflect.ValueOf(cfg).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		if name == "Projects" || name == "DataDir" {
			continue
		}
		fillNonZero(rv.Field(i), 0)
	}
	cfg.DataDir = dataDir
	cfg.Projects = []*models.Project{proj}
	// Version must stay positive so ApplyTo copies it back verbatim (the <=0
	// bump is exercised separately in TestDecomposeBumpsNonPositiveSchemaVersion).
	cfg.Version = 3
	return cfg
}

// TestDecomposeApplyToRoundTripsEveryField sets every Config field non-zero,
// decomposes into domains, recomposes onto a fresh Config, and asserts every
// owned field is byte-identical, every retired field is dropped, and the project
// catalog round-trips with a re-derived path.
func TestDecomposeApplyToRoundTripsEveryField(t *testing.T) {
	dataDir := "/data/root"
	wantPath := filepath.Join(dataDir, "projects", "proj-one")
	proj := &models.Project{ID: "p1", Slug: "proj-one", Name: "Proj One", Path: wantPath}

	full := fullyPopulatedConfig(dataDir, proj)

	d := Decompose(full)
	// Seed out with the same project so mergeCatalogProjects preserves computed
	// fields; DataDir supplies the derivation base.
	out := &Config{DataDir: dataDir, Projects: []*models.Project{proj}}
	d.ApplyTo(out)

	fullVal := reflect.ValueOf(*full)
	outVal := reflect.ValueOf(*out)
	rt := fullVal.Type()
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		owner := configFieldOwner[name]
		switch {
		case name == "Projects" || name == "DataDir":
			// Verified separately below.
			continue
		case owner == ownerRetired:
			if !outVal.Field(i).IsZero() {
				t.Errorf("retired field %q must be dropped on round trip, got %v", name, outVal.Field(i).Interface())
			}
		default:
			if !reflect.DeepEqual(fullVal.Field(i).Interface(), outVal.Field(i).Interface()) {
				t.Errorf("owned field %q (owner %s) did not round-trip:\n  in  = %v\n  out = %v",
					name, owner, fullVal.Field(i).Interface(), outVal.Field(i).Interface())
			}
		}
	}

	// Projects round-trip: identity preserved, path re-derived from the data dir.
	if len(out.Projects) != 1 {
		t.Fatalf("expected 1 project after apply, got %d", len(out.Projects))
	}
	if out.Projects[0].ID != "p1" || out.Projects[0].Slug != "proj-one" {
		t.Errorf("project identity not preserved: %+v", out.Projects[0])
	}
	if out.Projects[0].Path != wantPath {
		t.Errorf("project path = %q, want derived %q", out.Projects[0].Path, wantPath)
	}

	// DataDir is a runtime container: ApplyTo must not touch it.
	if out.DataDir != dataDir {
		t.Errorf("DataDir must be left to the runtime, got %q", out.DataDir)
	}
}

// TestDecomposeRetiresCustomStageConfiguration is the focused proof that the
// retired custom-stage fields are captured by no domain.
func TestDecomposeRetiresCustomStageConfiguration(t *testing.T) {
	cfg := &Config{
		Version:                  3,
		Stages:                   []string{"design", "build", "ship"},
		CrossOutlinerFilterStage: "build",
	}
	d := Decompose(cfg)

	out := &Config{}
	d.ApplyTo(out)

	if len(out.Stages) != 0 {
		t.Errorf("Stages must be retired (dropped) on round trip, got %+v", out.Stages)
	}
	if out.CrossOutlinerFilterStage != "" {
		t.Errorf("CrossOutlinerFilterStage must be retired, got %q", out.CrossOutlinerFilterStage)
	}
}

// TestDecomposeBumpsNonPositiveSchemaVersion documents the one migration
// behavior in Decompose: an unset/legacy schema version is normalized to the
// current version rather than persisted as zero.
func TestDecomposeBumpsNonPositiveSchemaVersion(t *testing.T) {
	for _, v := range []int{0, -1} {
		d := Decompose(&Config{Version: v})
		if d.Catalog.SchemaVersion != CurrentConfigVersion {
			t.Errorf("Decompose(Version=%d).Catalog.SchemaVersion = %d, want %d", v, d.Catalog.SchemaVersion, CurrentConfigVersion)
		}
	}
}

// TestDecomposeDefaultConfigRoundTrips proves the defaults path: a freshly
// constructed Config decomposes and recomposes with every owned field intact,
// so switching a load/save path to the domains (33.7.8) cannot silently reset a
// default. Retired fields are not part of NewConfig, so nothing is lost.
func TestDecomposeDefaultConfigRoundTrips(t *testing.T) {
	base := NewConfig()
	base.DataDir = "/data/root"

	d := Decompose(base)
	out := &Config{DataDir: base.DataDir, Projects: base.Projects}
	d.ApplyTo(out)

	fullVal := reflect.ValueOf(*base)
	outVal := reflect.ValueOf(*out)
	rt := fullVal.Type()
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		if name == "Projects" || name == "DataDir" || configFieldOwner[name] == ownerRetired {
			continue
		}
		if !reflect.DeepEqual(fullVal.Field(i).Interface(), outVal.Field(i).Interface()) {
			t.Errorf("default field %q did not round-trip:\n  in  = %v\n  out = %v",
				name, fullVal.Field(i).Interface(), outVal.Field(i).Interface())
		}
	}
}
