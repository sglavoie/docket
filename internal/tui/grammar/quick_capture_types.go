package grammar

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// Autocomplete mode constants
const (
	ACModeNone        = iota // No autocomplete active
	ACModeTag                // Completing a #tag
	acModeParent             // Completing a >parent
	ACModeChild              // Completing a <child
	acModePriority           // Completing a !priority
	acModeDate               // Completing a @date
	ACModeDestination        // Completing a $destination
	acModeProject            // Completing a %project
	acModePin                // Completing a *pin
	acModeReview             // Completing a =review
	ACModeContext            // Completing a ##context
	acModeDefer              // Completing a @@defer
	acModeAttachment         // Completing an &link
	acModeComment            // Completing a ;comment;
	// acModeRemoveTag completes a -#tag. It is reached only from the search
	// surface: capture does not carry the field at all, and quick edit peels the
	// two-character trigger itself so it can reuse the tag mode's candidate list.
	acModeRemoveTag
	// ACModeDependency completes a ~task and ACModeRemoveDependency a -~task.
	// Only the edit surface reaches the removal mode: capture has no stored set to
	// subtract from, and it peels the two-character trigger itself.
	ACModeDependency
	ACModeRemoveDependency
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
	acModeRemoveAttachment
	// acModeDerived completes a ?predicate and acModeRemoveDerived a -?predicate.
	// Both are search-only: a derived value is computed from the task rather than
	// stored on it, so there is nothing for capture or edit to write.
	acModeDerived
	acModeRemoveDerived
)

// acVisibleCount is the number of suggestions visible at once (scrollable window size)
const acVisibleCount = 7

// CaptureRouteSummary is what a host's route row resolved: the project and
// the destination every line agrees on, or Mixed where they disagree and the
// row says so instead of naming one.
type CaptureRouteSummary struct {
	ProjectID   string
	Destination app.CaptureDestination
	Mixed       bool
}

// QuickCaptureInput encapsulates the compact Task Composer's autocomplete,
// preview, and parsing infrastructure.
type QuickCaptureInput struct {
	Input textarea.Model
	err   error

	// Autocomplete state
	AC           AutocompleteState
	TagCache     []string
	contextCache []string
	projectCache []*models.Project
	Triggers     []ACTrigger // Active triggers (may exclude %project)

	// Live preview
	Preview            *QuickCaptureResult
	PreviewErr         string
	previewMinute      time.Time
	batchDuplicateHint string // settled with the preview, never parsed during layout

	// Context needed for parsing/preview
	Project           *models.Project
	DefaultParentID   *string // Composer's implicit parent, used for duplicate-hint ranking.
	allProjects       []*models.Project
	skipProjectSyntax bool // When true, %project syntax is disabled
	// HostRouteSummary is installed by a host that states the effective route
	// on a row of its own, and UpdatePreview settles HostRoute from it beside
	// the parse, so measuring the preview asks the host nothing. A preview
	// whose Route group that row already says in full stays silent about it.
	HostRouteSummary func() CaptureRouteSummary
	HostRoute        *CaptureRouteSummary

	// Layout
	Width int // Available width for preview wrapping

	// Syntax help state, shared by both capture hosts so the expanded/collapsed
	// flag and scroll offset live in one place. HelpExpanded is the logical
	// toggle; SyntaxVP holds the scrollable rendered guide and its offset, which
	// is preserved across autocomplete activation and clamped on resize.
	HelpExpanded       bool
	SyntaxVP           viewport.Model
	syntaxReady        bool
	syntaxAnchor       string // retained through resize until the reader scrolls
	syntaxAnchorOffset int
}

// NewQuickCaptureInput creates a new QuickCaptureInput with default textarea settings.
func NewQuickCaptureInput(project *models.Project, allProjects []*models.Project, placeholder string, skipProject bool) QuickCaptureInput {
	input := textarea.New()
	input.Placeholder = placeholder
	input.CharLimit = 200
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	input.Focus()
	theme.ApplyThemedTextarea(&input)

	triggers := quickCaptureTriggers(skipProject)

	qci := QuickCaptureInput{
		Input:             input,
		Project:           project,
		allProjects:       allProjects,
		skipProjectSyntax: skipProject,
		Triggers:          triggers,
	}
	qci.InitCaches()
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
		mode := AutocompleteModeForField(definition.Field)
		if mode == ACModeNone {
			continue
		}
		triggers = append(triggers, ACTrigger{Text: definition.Trigger, Mode: mode})
	}
	return triggers
}

func AutocompleteModeForField(field textgrammar.Field) int {
	switch field {
	case textgrammar.FieldTag:
		return ACModeTag
	case textgrammar.FieldRemoveTag:
		return acModeRemoveTag
	case textgrammar.FieldContext:
		return ACModeContext
	case textgrammar.FieldParent:
		return acModeParent
	case textgrammar.FieldChild:
		return ACModeChild
	case textgrammar.FieldPriority:
		return acModePriority
	case textgrammar.FieldDue:
		return acModeDate
	case textgrammar.FieldDefer:
		return acModeDefer
	case textgrammar.FieldDestination:
		return ACModeDestination
	case textgrammar.FieldProject:
		return acModeProject
	case textgrammar.FieldPinned:
		return acModePin
	case textgrammar.FieldReview:
		return acModeReview
	case textgrammar.FieldDependency:
		return ACModeDependency
	case textgrammar.FieldRemoveDependency:
		return ACModeRemoveDependency
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
		// While capturing, an attachment value is a pasted URL or path, so there
		// is nothing to suggest; search completes its &any/&none pair instead. The mode exists because quickCaptureTriggers drops a field
		// returning ACModeNone, and the registry projection must stay one-to-one;
		// RefreshAutocomplete has no default arm, so the mode simply shows nothing.
		return acModeAttachment
	case textgrammar.FieldRemoveAttachment:
		return acModeRemoveAttachment
	case textgrammar.FieldComment:
		// A comment is free prose, so there is nothing to suggest either; the mode
		// exists for the same one-to-one reason acModeAttachment does.
		return acModeComment
	default:
		return ACModeNone
	}
}
