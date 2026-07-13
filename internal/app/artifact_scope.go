package app

// ArtifactScope classifies where a workspace artifact lives in the durable-versus-
// machine-local split defined in the program's storage design: durable synced
// state travels with the data directory across machines and is committed through
// the transactional workspace, while machine-local state stays on one machine and
// is never part of a durable ChangeSet.
type ArtifactScope int

const (
	// ScopeUnknown is the zero value for an unclassified artifact.
	ScopeUnknown ArtifactScope = iota
	// ScopeDurable marks synced state persisted with the workspace (task markdown,
	// notes, blockers, project metadata and tags, templates, scoring profiles,
	// workflow rules, saved filters/views, review completion, and the planning
	// collections). It is the only scope the application layer commits through the
	// transactional Repository.
	ScopeDurable
	// ScopeMachineLocal marks state that stays on one machine and must never be
	// committed to the shared workspace (editors, integrations, current navigation,
	// histories, recent visits, and layout). It belongs in machine.json, not in a
	// ChangeSet.
	ScopeMachineLocal
)

// ArtifactKind enumerates the workspace artifacts the application layer mediates,
// so callers classify each one through a single exhaustive switch instead of
// scattering string comparisons across adapters.
type ArtifactKind string

const (
	// Durable synced artifacts.
	ArtifactTasks            ArtifactKind = "tasks"
	ArtifactNotes            ArtifactKind = "notes"
	ArtifactBlockers         ArtifactKind = "blockers"
	ArtifactProjectMetadata  ArtifactKind = "project_metadata"
	ArtifactProjectTags      ArtifactKind = "project_tags"
	ArtifactTemplates        ArtifactKind = "templates"
	ArtifactScoringProfiles  ArtifactKind = "scoring_profiles"
	ArtifactWorkflowRules    ArtifactKind = "workflow_rules"
	ArtifactSavedFilters     ArtifactKind = "saved_filters"
	ArtifactReviewCompletion ArtifactKind = "review_completion"
	ArtifactFocusQueue       ArtifactKind = "focus_queue"
	ArtifactDailyPlan        ArtifactKind = "daily_plan"

	// Machine-local UI artifacts.
	ArtifactEditors      ArtifactKind = "editors"
	ArtifactIntegrations ArtifactKind = "integrations"
	ArtifactNavigation   ArtifactKind = "navigation"
	ArtifactHistories    ArtifactKind = "histories"
	ArtifactLayout       ArtifactKind = "layout"
	ArtifactRecentVisits ArtifactKind = "recent_visits"
)

// ClassifyArtifact reports whether a workspace artifact is durable synced state or
// machine-local UI state. The application layer commits only durable artifacts
// through the transactional Repository; routing a machine-local artifact there is
// a programming error the mutation path rejects.
func ClassifyArtifact(kind ArtifactKind) ArtifactScope {
	switch kind {
	case ArtifactTasks, ArtifactNotes, ArtifactBlockers, ArtifactProjectMetadata,
		ArtifactProjectTags, ArtifactTemplates, ArtifactScoringProfiles,
		ArtifactWorkflowRules, ArtifactSavedFilters, ArtifactReviewCompletion,
		ArtifactFocusQueue, ArtifactDailyPlan:
		return ScopeDurable
	case ArtifactEditors, ArtifactIntegrations, ArtifactNavigation,
		ArtifactHistories, ArtifactLayout, ArtifactRecentVisits:
		return ScopeMachineLocal
	default:
		return ScopeUnknown
	}
}

// IsDurable reports whether an artifact is durable synced state.
func (k ArtifactKind) IsDurable() bool { return ClassifyArtifact(k) == ScopeDurable }

// IsMachineLocal reports whether an artifact is machine-local UI state.
func (k ArtifactKind) IsMachineLocal() bool { return ClassifyArtifact(k) == ScopeMachineLocal }
