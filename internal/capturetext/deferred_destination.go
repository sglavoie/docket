package capturetext

import "github.com/sglavoie/docket/internal/app"

// ComposerDeferredDestination follows the deferral route the application layer
// takes, so the composer header and the request it submits agree about where a
// deferred line lands. A terminal destination is left alone because the refusal
// belongs to the application layer and has its own wording.
func ComposerDeferredDestination(parsed *QuickCaptureResult, destination app.CaptureDestination) app.CaptureDestination {
	if parsed == nil || !parsed.DeferSet || parsed.DeferUntil == nil {
		return destination
	}
	if destination.WorkflowState().IsTerminal() {
		return destination
	}
	return app.CaptureDestinationBacklog
}
