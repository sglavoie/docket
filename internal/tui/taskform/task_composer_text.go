package taskform

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

// ParseCaptureText reads text exactly as the compact composer would on
// submit — every nonblank line through the capture grammar and the preset's
// defaults — without a composer on screen. A host capturing through the
// control socket uses it, so a line refused here is one `a` would refuse too.
func ParseCaptureText(preset TaskComposerPreset, runtime *config.RuntimeState, text string) ([]app.CaptureRequest, []grammar.LineError) {
	composer := NewTaskComposerModel(preset, runtime)
	composer.Input.SetValue(text)
	reqs, ok := composer.ParseCompactLines()
	if !ok {
		return nil, composer.Errors
	}
	return reqs, nil
}

// CapturePreviewText describes a host's full draft through the composer's own
// validation and route summary. Invalid drafts return no misleading route;
// opening an empty field still describes the default destination.
func CapturePreviewText(preset TaskComposerPreset, runtime *config.RuntimeState, text string) (string, []grammar.LineError) {
	composer := NewTaskComposerModel(preset, runtime)
	composer.Input.SetValue(text)
	var reqs []app.CaptureRequest
	if strings.TrimSpace(text) != "" {
		var ok bool
		reqs, ok = composer.ParseCompactLines()
		if !ok {
			return "", composer.Errors
		}
	}
	project, plan, state := composer.effectiveRouteHeader().strings()
	parts := []string{"Project: " + project}
	if plan != "" {
		parts = append(parts, "Plan: "+plan)
	}
	parts = append(parts, "Status: "+state)
	now := time.Now()
	var due string
	for i, req := range reqs {
		value := ""
		if req.Due != nil {
			value = chrome.FormatWeekdayDate(*req.Due, now, (&models.Task{DueDate: req.Due}).DueHasTime())
		}
		if i == 0 {
			due = value
		} else if due != value {
			due = composerMixedValue
		}
	}
	if due != "" {
		parts = append(parts, "Due: "+due)
	}
	for _, field := range []struct {
		label string
		value func(app.CaptureRequest) string
	}{
		{"Defer until", func(req app.CaptureRequest) string {
			if req.Defer == nil {
				return ""
			}
			return chrome.FormatWeekdayDate(req.Defer.Local(), now, true)
		}},
		{"Priority", func(req app.CaptureRequest) string {
			if req.Priority == models.PriorityUnset {
				return ""
			}
			return models.PriorityName(req.Priority)
		}},
	} {
		value := ""
		for i, req := range reqs {
			next := field.value(req)
			if i == 0 {
				value = next
			} else if next != value {
				value = composerMixedValue
			}
		}
		if value != "" {
			parts = append(parts, field.label+": "+value)
		}
	}
	var notices []string
	for _, req := range reqs {
		project := composer.projectByID(req.ProjectID)
		input := grammar.QuickCaptureInput{Project: project,
			Preview: &grammar.QuickCaptureResult{Title: req.Title, ParentID: req.ParentID}}
		if hint := input.DuplicateCaptureHint(); hint != "" {
			if len(reqs) > 1 {
				hint = fmt.Sprintf("Line %d: %s", req.SourceLine, hint)
			}
			notices = append(notices, hint)
		}
	}
	return strings.Join(append([]string{strings.Join(parts, " · ")}, notices...), "\n"), nil
}

// CaptureCompletion is one suggestion CaptureCompletions offers: Label is the
// plain text a list shows and Insert the whole replacement for the token.
type CaptureCompletion struct {
	Label  string
	Insert string
}

// CaptureCompletionSet is what the compact composer would offer at a caret.
// Token is the text being completed, sigil included, and empty when the caret
// stands on no sigil the grammar completes. Exact is the composer's own test
// for whether Enter submits instead of accepting.
type CaptureCompletionSet struct {
	Token string
	Items []CaptureCompletion
	Exact bool
}

// captureCompletionWidth is the width a headless composer truncates its task
// titles at. A host draws its own list, so the budget is generous rather than a
// terminal's, while still bounding a pathological title.
const captureCompletionWidth = 120

// CaptureCompletions reads text as the draft up to the caret and returns what
// the compact composer would suggest there, each suggestion as accepting it
// would rewrite the token. A host completing through the control socket uses
// it, so it offers exactly what `a` offers.
func CaptureCompletions(preset TaskComposerPreset, runtime *config.RuntimeState, text string) CaptureCompletionSet {
	composer := NewTaskComposerModel(preset, runtime)
	composer.SetSize(captureCompletionWidth, composer.height)
	composer.Input.SetValue(text)
	composer.RefreshAutocomplete()
	value := composer.Input.Value()
	if !composer.AC.IsActive() || composer.AC.TriggerPos > len(value) {
		return CaptureCompletionSet{}
	}
	trigger := grammar.TriggerCharForMode(composer.AC.Mode, composer.Triggers)
	set := CaptureCompletionSet{
		Token: value[composer.AC.TriggerPos:],
		Items: make([]CaptureCompletion, 0, len(composer.AC.Items)),
		Exact: composer.AutocompleteLiteralComplete(),
	}
	for _, item := range composer.AC.Items {
		set.Items = append(set.Items, CaptureCompletion{
			Label:  ansi.Strip(item.Display),
			Insert: trigger + item.Value + " ",
		})
	}
	return set
}
