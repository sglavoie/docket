package tui

// populationState is the rebuild memoization guard over the loaded project
// population. It holds the inputs the last crossProjectRebuildRows pass ran
// under — the row fingerprint, the focus-stack depth, and the total task count —
// plus the one-shot flag that bypasses them.
//
// The flag is the reason this is a named sub-state rather than four promoted
// fields. Everything that changes what the population would produce has to say
// so, and it says so by calling invalidate(); "who invalidates the population"
// is then a call graph rather than an assignment grep. The zero value is a cold
// guard whose first pass always rebuilds, since an empty fingerprint cannot
// match a real one.
type populationState struct {
	fingerprint  string
	focusDepth   int
	taskCount    int
	forceRebuild bool
}

// invalidate declares the loaded population, or the projection taken over it,
// changed underneath the guard, so the next rebuild runs whatever the guard
// inputs say. It is one-shot: the rebuild that honors it consumes it through
// record.
func (p *populationState) invalidate() { p.forceRebuild = true }

// memoized reports whether the pass about to run would reproduce the rows the
// last one already built. A standing force flag answers no on its own, before
// any input is compared.
func (p *populationState) memoized(fingerprint string, focusDepth, taskCount int) bool {
	return !p.forceRebuild &&
		fingerprint == p.fingerprint &&
		focusDepth == p.focusDepth &&
		taskCount == p.taskCount
}

// record stores the guard inputs the running pass is rebuilding under and
// consumes the force flag, so one invalidate() buys exactly one rebuild.
func (p *populationState) record(fingerprint string, focusDepth, taskCount int) {
	p.fingerprint = fingerprint
	p.focusDepth = focusDepth
	p.taskCount = taskCount
	p.forceRebuild = false
}

// recordFocusDepth re-records the depth alone. Focus-stack validation runs after
// record has already stored the depth the guard was entered with, and a repaired
// stack is the depth the next pass must compare against or that pass rebuilds
// for nothing.
func (p *populationState) recordFocusDepth(focusDepth int) { p.focusDepth = focusDepth }
