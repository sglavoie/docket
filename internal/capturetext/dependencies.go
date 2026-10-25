package capturetext

import (
	"slices"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// parseDependencyTokens collects every ~task reference in input order. Removal
// and clearing reach this parser on purpose: capture tokenizes both so the
// refusal can name the operation instead of letting -~12 or ~0 fold into the
// title, and a task that does not exist yet has nothing to remove or clear.
//
// A dependency is a soft reference, so an addition asks only that the target be
// a real, still-open task the workspace loaded; nothing about the new task's
// readiness or ordering follows from it.
func (p *quickCaptureParser) parseDependencyTokens(add, remove []textgrammar.Token) {
	definition, ok := textgrammar.DefinitionFor(p.surface, textgrammar.FieldDependency)
	if !ok {
		p.failf("dependency grammar is unavailable")
		return
	}
	// The removal form is read from the edit surface because capture tokenizes it
	// without teaching it, so capture advertises no definition to name it with.
	removeDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldRemoveDependency)
	if p.surface == textgrammar.SurfaceOutline {
		p.parseOutlineDependencyTokens(add, remove, definition, removeDefinition)
		return
	}
	if len(remove) > 0 {
		p.failTokenf(remove[0], "%stask is not available when capturing; a new task has no dependencies to remove", removeDefinition.Trigger)
		return
	}
	for _, token := range add {
		if token.Value == DependencyClearValue {
			p.failTokenf(token, "%s%s is not available when capturing; a new task starts without dependencies", definition.Trigger, token.Value)
			return
		}
		ref, err := ResolveDependencyReference(token.Value, p.project, p.allProjects)
		if err != nil {
			if p.failTokenf(token, "%s", err) {
				return
			}
			continue
		}
		if !ref.Resolved {
			if p.failTokenf(token, "%s", DependencyNotFoundMessage(definition.Trigger, token.Value)) {
				return
			}
			continue
		}
		if ref.Terminal {
			if p.failTokenf(token, "cannot depend on %s: it is already closed", ref.Label()) {
				return
			}
			continue
		}
		p.result.DependencyRefs = AppendDependencyRef(p.result.DependencyRefs, ref)
	}
}

// parseOutlineDependencyTokens reads the outline buffer's declarative dependency
// set. The line is a complete statement, so the positive tokens are the final
// set: deleting a dumped ~task is what removes that dependency, -~task subtracts
// from the same line's positives for anyone who would rather strike a reference
// than delete it, and ~0 states the empty set.
//
// Nothing here judges whether a target is closed or loaded. The dump prints
// terminal and unavailable references so they can round-trip, and only the
// application layer knows which locators the task already stored — so it, not
// this parser, decides that an existing reference may stay while a newly typed
// one must name a loaded open task.
func (p *quickCaptureParser) parseOutlineDependencyTokens(add, remove []textgrammar.Token, definition, removeDefinition textgrammar.Definition) {
	clears := 0
	for _, token := range add {
		if token.Value == DependencyClearValue {
			clears++
		}
	}
	if clears > 0 {
		if clears != len(add) || len(remove) > 0 {
			p.failf("%s%s is the empty set and cannot be combined with adding or removing dependencies", definition.Trigger, DependencyClearValue)
		}
		return
	}

	removed := make([]app.TaskLocator, 0, len(remove))
	for _, token := range remove {
		if token.Value == DependencyClearValue {
			p.failTokenf(token, "%s%s is not a clear form; use %s%s to clear every dependency", removeDefinition.Trigger, token.Value, definition.Trigger, DependencyClearValue)
			return
		}
		ref, ok := p.resolveOutlineDependency(token, removeDefinition.Trigger)
		if !ok {
			return
		}
		removed = append(removed, ref.Locator)
	}
	for _, token := range add {
		ref, ok := p.resolveOutlineDependency(token, definition.Trigger)
		if !ok {
			return
		}
		if slices.Contains(removed, ref.Locator) {
			continue
		}
		p.result.DependencyRefs = AppendDependencyRef(p.result.DependencyRefs, ref)
	}
}

// resolveOutlineDependency addresses one outline token. A locator the workspace
// cannot resolve is still accepted when it is well-shaped, which is what lets the
// project-id:task-id fallback the dump printed for an unavailable target come
// back in unchanged.
func (p *quickCaptureParser) resolveOutlineDependency(token textgrammar.Token, trigger string) (CapturedDependencyRef, bool) {
	ref, err := ResolveDependencyReference(token.Value, p.project, p.allProjects)
	if err != nil {
		p.failTokenf(token, "%s", err)
		return CapturedDependencyRef{}, false
	}
	if ref.Locator.TaskID == "" {
		p.failTokenf(token, "%s", DependencyNotFoundMessage(trigger, token.Value))
		return CapturedDependencyRef{}, false
	}
	return ref, true
}
