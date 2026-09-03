package operation

import (
	"strings"

	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// Capability is a declarative feature exposed by the active route or overlay.
// Routes and overlays construct CommandContext values from their active mode.
type Capability string

const (
	CapabilityCommandDiscovery Capability = "command-discovery"
	// CapabilitySimplifiedInput exposes the staged final key grammar without
	// making its operations visible to the legacy command adapter.
	CapabilitySimplifiedInput Capability = "input:simplified"
	// CapabilityScopeHub limits scope/context lifecycle operations to the
	// routed Scope Hub that supplies their frozen targets.
	CapabilityScopeHub Capability = "surface:scope-hub"
	// CapabilityProjectHub limits global project creation to the routed project
	// selection and administration surface.
	CapabilityProjectHub Capability = "surface:project-hub"
	// CapabilityNotesManager limits the Notes grammar's operations to the
	// mounted manager that owns the rename, picker, and confirmation states
	// they delegate to.
	CapabilityNotesManager Capability = "surface:notes-manager"
	// CapabilityAttachmentsPanel limits the Attachments grammar's operations to
	// the mounted panel whose cursor names the item they act on.
	CapabilityAttachmentsPanel Capability = "surface:attachments-panel"
	// CapabilityCommentsPanel limits the Comments grammar's operations to the
	// mounted panel whose cursor names the comment they act on.
	CapabilityCommentsPanel Capability = "surface:comments-panel"
	// CapabilityLensPicker limits the named-lens lifecycle operations to the
	// mounted lens picker whose cursor names the saved lens they act on. Create
	// carries it too, because the picker is also the only surface that offers
	// saving the current view.
	CapabilityLensPicker   Capability = "surface:lens-picker"
	CapabilityAppScope     Capability = "scope:app"
	CapabilityProjectScope Capability = "scope:project"
	CapabilityTaskScope    Capability = "scope:task"
)

// capabilityFamilies are the prefixes reserved for capabilities generated per
// route, overlay, presentation, scope, surface, or input strategy.
var capabilityFamilies = []string{
	"presentation:",
	"scope:",
	"route:",
	"overlay:",
	"surface:",
	"input:",
}

// validCapability reports whether a capability is a declared constant or a
// member of a reserved dynamic family. It catches typos at registration.
func validCapability(capability Capability) bool {
	switch capability { //nolint:exhaustive // Test-only harness capabilities are registered separately.
	case CapabilityCommandDiscovery,
		CapabilitySimplifiedInput,
		CapabilityScopeHub,
		CapabilityProjectHub,
		CapabilityNotesManager,
		CapabilityAttachmentsPanel,
		CapabilityCommentsPanel,
		CapabilityLensPicker,
		CapabilityAppScope,
		CapabilityProjectScope,
		CapabilityTaskScope:
		return true
	}
	for _, family := range capabilityFamilies {
		if strings.HasPrefix(string(capability), family) &&
			len(capability) > len(family) {
			return true
		}
	}
	return false
}

// CapabilitySet is the active route/overlay capability set.
type CapabilitySet map[Capability]struct{}

// NewCapabilitySet builds a capability set from the supplied values.
func NewCapabilitySet(capabilities ...Capability) CapabilitySet {
	set := make(CapabilitySet, len(capabilities))
	for _, capability := range capabilities {
		set[capability] = struct{}{}
	}
	return set
}

// Has reports whether capability is present.
func (s CapabilitySet) Has(capability Capability) bool {
	_, ok := s[capability]
	return ok
}

// HasAll reports whether every required capability is present.
func (s CapabilitySet) HasAll(required CapabilitySet) bool {
	for capability := range required {
		if !s.Has(capability) {
			return false
		}
	}
	return true
}

// Add inserts capabilities into the set.
func (s CapabilitySet) Add(capabilities ...Capability) {
	for _, capability := range capabilities {
		s[capability] = struct{}{}
	}
}

// SelectionContext preserves project-qualified task identity independently of
// cursor or row position.
type SelectionContext struct {
	Tasks []taskref.TaskIdentity
	// VisibleCount and HiddenCount classify Tasks against the originating
	// projection at the moment the command context is frozen.
	VisibleCount int
	HiddenCount  int
	// Entities mirrors Tasks with the lifecycle and project mutability facts
	// needed to decide whether an operation applies to every selected target.
	// It stays pointer-free so a frozen action sheet cannot be retargeted by a
	// later row rebuild.
	Entities []EntityContext
	// EntityIdentities is the typed selection for surfaces whose rows are not
	// tasks. Hosts that only select tasks may leave it empty; selected-entity
	// resolution then derives it from Tasks.
	EntityIdentities []EntityIdentity
	// Visual distinguishes an explicit visual selection (including one selected
	// task) from the ordinary cursor fallback used by selected-target operations.
	Visual bool
}

// Count returns the number of selected tasks.
func (s SelectionContext) Count() int { return len(s.Tasks) }

// EntityContext describes the entity a command would act on. It carries
// identity and the archive guard only: an applicability rule that needs live
// task fields reads them from the resolved targets, so a frozen context can
// never disagree with the workspace snapshot the handler mutates.
type EntityContext struct {
	ProjectID string
	TaskID    string
	Archived  bool
	// HasChecklistCursor reports that the cursor rests on a checklist row owned
	// by the task TaskID names. The owner's task and project stay filled beside
	// it so the archived-project guard still fires, which is exactly why the
	// checklist cursor needs a flag of its own: nothing else distinguishes it
	// from the owner row whose pointer it carries.
	HasChecklistCursor bool
	// ChecklistIndex is the item's position inside the owner's checklist. It is
	// meaningful only while HasChecklistCursor holds.
	ChecklistIndex int
	// HasSectionHeaderCursor reports that the cursor rests on a grouping section
	// header, which names neither a task nor a project. Nothing else
	// distinguishes it from an empty entity in a single-project scope.
	HasSectionHeaderCursor bool
	// HasProjectHeaderCursor reports that the cursor rests on a project group
	// header. ProjectID alone cannot say so: a single-project scope fills it from
	// the scope on every row, section headers included, so an operation that acts
	// on the project as a whole reads this flag instead.
	HasProjectHeaderCursor bool
	// HasNoteCursor reports that the cursor rests on a row of the search
	// projection's notes tier. A note is not an entity the operation grammar
	// resolves — it carries no project or task ID — so an empty entity is the
	// only thing it would otherwise look like, and Open Note could not tell
	// itself apart from Open Project.
	HasNoteCursor bool
}

// CommandContext is the complete input to capability-based command
// availability. Existing commands continue to use Views and Available; new or
// migrated commands may additionally declare RequiredCapabilities and
// ContextAvailable.
type CommandContext struct {
	View         presentation.PresentationID
	Capabilities CapabilitySet
	Selection    SelectionContext
	Entity       EntityContext
	// CurrentEntity is the typed identity of the focused row. Surfaces whose
	// rows are not tasks or projects set it directly; task and project surfaces
	// may leave it empty and let resolution derive it from Entity.
	CurrentEntity EntityIdentity
	// ScopeProjectIDs is the stable project set admitted by the active scope.
	// It lets operation execution resolve scoped-project targets without
	// depending on renderer models or palette-specific messages.
	ScopeProjectIDs []string
}
