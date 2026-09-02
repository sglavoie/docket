package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// Autocomplete mode constants
const (
	acModeNone        = iota // No autocomplete active
	acModeTag                // Completing a #tag
	acModeParent             // Completing a >parent
	acModeChild              // Completing a <child
	acModePriority           // Completing a !priority
	acModeDate               // Completing a @date
	acModeDestination        // Completing a $destination
	acModeProject            // Completing a %project
	acModePin                // Completing a *pin
	acModeReview             // Completing a =review
	acModeContext            // Completing a ##context
	acModeDefer              // Completing a @@defer
	acModeAttachment         // Completing an &link
	acModeComment            // Completing a ;comment;
	// acModeRemoveTag completes a -#tag. It is reached only from the search
	// surface: capture does not carry the field at all, and quick edit peels the
	// two-character trigger itself so it can reuse the tag mode's candidate list.
	acModeRemoveTag
	// acModeDependency completes a ~task and acModeRemoveDependency a -~task.
	// Only the edit surface reaches the removal mode: capture has no stored set to
	// subtract from, and it peels the two-character trigger itself.
	acModeDependency
	acModeRemoveDependency
	// The remaining negations are search-only, so none of them is ever reached
	// while capturing or editing. Each still needs a mode of its own because the
	// field-to-mode projection has to stay one-to-one for a trigger to be resolved
	// back to the definition it came from; the suggestions themselves come from the
	// positive twin's vocabulary.
	acModeRemovePriority
	acModeRemoveContext
	acModeRemoveDue
	acModeRemoveDefer
	acModeRemoveDestination
	acModeRemovePin
	acModeRemoveReview
	acModeRemoveProject
	// acModeDerived completes a ?predicate and acModeRemoveDerived a -?predicate.
	// Both are search-only: a derived value is computed from the task rather than
	// stored on it, so there is nothing for capture or edit to write.
	acModeDerived
	acModeRemoveDerived
)

// acVisibleCount is the number of suggestions visible at once (scrollable window size)
const acVisibleCount = 7

// capturedChildRef is one resolved <child reference. The identifying fields are
// copied out of the task the reference named rather than kept as a pointer, so a
// parse result can be previewed and mapped into a request without holding a
// handle on a view-owned task.
type capturedChildRef struct {
	id       string
	shortID  int
	title    string
	terminal bool
}

// quickCaptureResult holds parsed metadata from quick capture input.
type quickCaptureResult struct {
	title             string
	priority          int
	prioritySet       bool
	tags              []string
	context           string
	dueDate           *time.Time
	deferUntil        *time.Time
	deferSet          bool
	parentID          *string
	children          []capturedChildRef
	destination       app.CaptureDestination
	destinationSet    bool
	pinned            bool
	review            bool
	dependencies      []capturedDependencyRef
	attachments       []string
	comments          []string
	targetProjectID   *string // Target project ID for cross-project capture
	targetProjectName string  // Target project name (for preview display)
}

// childIDs projects the resolved children into the stable IDs a CaptureRequest
// carries, in reference order.
func (r *quickCaptureResult) childIDs() []string {
	if len(r.children) == 0 {
		return nil
	}
	ids := make([]string, len(r.children))
	for i, child := range r.children {
		ids[i] = child.id
	}
	return ids
}

// dependencyLocators projects the resolved references into the locators a
// CaptureRequest carries, in reference order.
func (r *quickCaptureResult) dependencyLocators() []app.TaskLocator {
	return dependencyLocators(r.dependencies)
}

// quickCaptureInput encapsulates the compact Task Composer's autocomplete,
// preview, and parsing infrastructure.
type quickCaptureInput struct {
	input textarea.Model
	err   error

	// Autocomplete state
	ac           AutocompleteState
	tagCache     []string
	contextCache []string
	projectCache []*models.Project
	triggers     []ACTrigger // Active triggers (may exclude %project)

	// Live preview
	preview    *quickCaptureResult
	previewErr string

	// Context needed for parsing/preview
	project           *models.Project
	allProjects       []*models.Project
	skipProjectSyntax bool // When true, %project syntax is disabled

	// Layout
	width int // Available width for preview wrapping

	// Syntax help state, shared by both capture hosts so the expanded/collapsed
	// flag and scroll offset live in one place. helpExpanded is the logical
	// toggle; syntaxVP holds the scrollable rendered guide and its offset, which
	// is preserved across autocomplete activation and clamped on resize.
	helpExpanded bool
	syntaxVP     viewport.Model
	syntaxReady  bool
}

// newQuickCaptureInput creates a new quickCaptureInput with default textarea settings.
func newQuickCaptureInput(project *models.Project, allProjects []*models.Project, placeholder string, skipProject bool) quickCaptureInput {
	input := textarea.New()
	input.Placeholder = placeholder
	input.CharLimit = 200
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	input.Focus()
	applyThemedTextarea(&input)

	triggers := quickCaptureTriggers(skipProject)

	qci := quickCaptureInput{
		input:             input,
		project:           project,
		allProjects:       allProjects,
		skipProjectSyntax: skipProject,
		triggers:          triggers,
	}
	qci.initCaches()
	return qci
}

// quickCaptureTriggers projects the canonical capture definitions into the
// autocomplete engine's presentation-specific modes. The registry owns which
// triggers exist; this switch owns only the TUI behavior attached to each field.
func quickCaptureTriggers(skipProject bool) []ACTrigger {
	definitions := textgrammar.Definitions(textgrammar.SurfaceCapture)
	triggers := make([]ACTrigger, 0, len(definitions))
	for _, definition := range definitions {
		if skipProject && definition.Field == textgrammar.FieldProject {
			continue
		}
		mode := autocompleteModeForField(definition.Field)
		if mode == acModeNone {
			continue
		}
		triggers = append(triggers, ACTrigger{Text: definition.Trigger, Mode: mode})
	}
	return triggers
}

func autocompleteModeForField(field textgrammar.Field) int {
	switch field {
	case textgrammar.FieldTag:
		return acModeTag
	case textgrammar.FieldRemoveTag:
		return acModeRemoveTag
	case textgrammar.FieldContext:
		return acModeContext
	case textgrammar.FieldParent:
		return acModeParent
	case textgrammar.FieldChild:
		return acModeChild
	case textgrammar.FieldPriority:
		return acModePriority
	case textgrammar.FieldDue:
		return acModeDate
	case textgrammar.FieldDefer:
		return acModeDefer
	case textgrammar.FieldDestination:
		return acModeDestination
	case textgrammar.FieldProject:
		return acModeProject
	case textgrammar.FieldPinned:
		return acModePin
	case textgrammar.FieldReview:
		return acModeReview
	case textgrammar.FieldDependency:
		return acModeDependency
	case textgrammar.FieldRemoveDependency:
		return acModeRemoveDependency
	case textgrammar.FieldRemovePriority:
		return acModeRemovePriority
	case textgrammar.FieldRemoveContext:
		return acModeRemoveContext
	case textgrammar.FieldRemoveDue:
		return acModeRemoveDue
	case textgrammar.FieldRemoveDefer:
		return acModeRemoveDefer
	case textgrammar.FieldRemoveDestination:
		return acModeRemoveDestination
	case textgrammar.FieldRemovePinned:
		return acModeRemovePin
	case textgrammar.FieldRemoveReview:
		return acModeRemoveReview
	case textgrammar.FieldRemoveProject:
		return acModeRemoveProject
	case textgrammar.FieldDerived:
		return acModeDerived
	case textgrammar.FieldRemoveDerived:
		return acModeRemoveDerived
	case textgrammar.FieldAttachment:
		// An attachment value is a pasted URL or path, so there is nothing to
		// suggest. The mode exists because quickCaptureTriggers drops a field
		// returning acModeNone, and the registry projection must stay one-to-one;
		// refreshAutocomplete has no default arm, so the mode simply shows nothing.
		return acModeAttachment
	case textgrammar.FieldComment:
		// A comment is free prose, so there is nothing to suggest either; the mode
		// exists for the same one-to-one reason acModeAttachment does.
		return acModeComment
	default:
		return acModeNone
	}
}
