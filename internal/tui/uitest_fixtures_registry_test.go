package tui

import (
	"sort"
)

// This file builds the reliable data surfaces the LLM-driven review harness
// drives against. Two shapes are supported:
//
//   - Built-in fixtures construct a fresh
//     isolated temp data dir, write projects through storage, and launch the
//     driver from that copied config.
//   - Copied-config workspaces load a real user config, copy its data dir into
//     temp storage, assign the copy a fresh workspace identity, and launch the
//     driver from there — never mutating the source config, machine state, or
//     task files.
//
// Fixture builders live beside the driver in internal/tui so committed
// scenarios exercise production model construction without importing unrelated
// test helpers.

// Built-in fixture names accepted by NewFixtureDriver and the committed
// test-only scenario harness.
const (
	FixtureEmpty        = "empty"
	FixtureFresh        = "fresh"
	FixtureRich         = "rich"
	FixtureBusy         = "busy"
	FixtureWorkflow     = "workflow"
	FixtureNarrow       = "narrow"
	FixtureMultiProject = "multi-project"
	FixtureInboxTriage  = "inbox-triage"
	FixtureRelease      = "release"
	FixtureParked       = "parked"
	FixtureProjectMix   = "project-mix"
	FixtureSignals      = "signals"
	FixtureStale        = "stale"
	FixtureChecklist    = "checklist"
	// FixtureUXAcceptance is the one fixture where the axes every other fixture
	// isolates are simultaneously true, so a row, a Peek document, a selection,
	// and a narrow terminal can be reviewed against the same workspace.
	FixtureUXAcceptance = "ux-acceptance"
)

// Checklist fixture identities. Scenarios assert on these titles and item texts,
// so the builder and the committed JSON agree through these constants.
const (
	checklistFixtureProjectName = "Checklist Delivery"
	checklistFixtureFoldTitle   = "Draft the launch runbook"
	checklistFixtureParentTitle = "Coordinate the rollout window"
	checklistFixturePlainTitle  = "Retire the legacy runbook"
	checklistFixtureFirstItem   = "Outline the rollback steps"
	checklistFixtureSecondItem  = "Name the on-call owner"
	checklistFixtureThirdItem   = "Attach the comms draft"
)

// Signals fixture identities. Scenarios assert on the project slug and on task
// titles, so the builder and the committed JSON agree through these constants.
const (
	signalsFixtureProjectName = "Signal Delivery Coordination Track"
	signalsFixtureOtherName   = "Signal Beta"
)

// Inbox-triage fixture identities. The advance scenarios assert on these exact
// titles, and no title may be a substring of another so a `contains` check can
// only be answered by the row it names.
const (
	inboxTriageAlphaName         = "Triage Alpha"
	inboxTriageBetaName          = "Triage Beta"
	inboxTriageAlphaLeadTitle    = "Alpha intake ready for triage"
	inboxTriageAlphaFinalTitle   = "Alpha intake closing the group"
	inboxTriageBetaParentTitle   = "Beta intake parent awaiting triage"
	inboxTriageBetaChildTitle    = "Beta intake child promoted on departure"
	inboxTriageBetaTrailingTitle = "Beta intake trailing remainder"

	inboxTriageAlphaLeadDetail  = "Detail line belonging to the departing intake."
	inboxTriageAlphaFinalDetail = "Detail line belonging to the successor intake."
)

// Parked fixture identities. Scenarios assert on these strings, and
// models.NewProject stores a name verbatim, so the constants are the single
// source both the builder and the committed JSON agree on.
const (
	parkedFixtureWorkspaceName = "Active Delivery"
	parkedFixtureParkedName    = "Parked Work"
)

// Project-mix fixture identities. The Select Project task filter asserts on
// presence and absence of these exact names, so no name may be a substring of
// another.
const (
	projectMixActiveName   = "Mix Active Delivery"
	projectMixEmptyName    = "Mix Quiet Planning"
	projectMixFinishedName = "Mix Finished Cleanup"
	projectMixParkedName   = "Mix Parked Migration"
	projectMixArchivedName = "Mix Retired Pilot"

	// Each project also carries a one-line description and a descriptive tag
	// whose words appear nowhere else in the fixture — not in another project's
	// metadata and not in any task — so a Select Project search by description or
	// tag can only be answered by the metadata aliases, and a search by task
	// wording can only be answered by a leak.
	projectMixActiveDescription   = "Coordinate the storefront launch calendar."
	projectMixEmptyDescription    = "Sketch the charter before anyone commits."
	projectMixFinishedDescription = "Retire the legacy warehouse scripts."
	projectMixParkedDescription   = "Resume the database cutover when staffing returns."
	projectMixArchivedDescription = "Shelved the kiosk trial after the field study."
)

// builtinFixtures maps each fixture name to the builder that populates its
// workspace. The workspace is already created (empty temp data dir + config)
// when the builder runs; the builder only adds projects and sets config state.
var builtinFixtures = map[string]func(*uitestWorkspace) error{
	"check-ins":         buildCheckInFixture,
	FixtureEmpty:        buildEmptyFixture,
	FixtureFresh:        nil,
	FixtureRich:         buildRichFixture,
	FixtureBusy:         buildBusyFixture,
	FixtureWorkflow:     buildWorkflowFixture,
	FixtureNarrow:       buildNarrowFixture,
	FixtureMultiProject: buildMultiProjectFixture,
	FixtureInboxTriage:  buildInboxTriageFixture,
	FixtureRelease:      buildReleaseFixture,
	FixtureParked:       buildParkedFixture,
	FixtureProjectMix:   buildProjectMixFixture,
	FixtureSignals:      buildSignalsFixture,
	FixtureStale:        buildStaleFixture,
	FixtureChecklist:    buildChecklistFixture,
	FixtureUXAcceptance: buildUXAcceptanceFixture,
}

// BuiltinFixtureNames returns the sorted list of built-in fixture names.
func BuiltinFixtureNames() []string {
	names := make([]string, 0, len(builtinFixtures))
	for name := range builtinFixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
