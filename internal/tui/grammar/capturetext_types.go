package grammar

import "github.com/sglavoie/tt/internal/capturetext"

// The capture-line parser lives in internal/capturetext so the mobile bridge
// reads a line exactly as the composer does without linking a terminal stack.
// These aliases keep the presentation half of the grammar spelling the parse
// result in its own vocabulary.
type (
	QuickCaptureResult    = capturetext.QuickCaptureResult
	CapturedDependencyRef = capturetext.CapturedDependencyRef
	LineError             = capturetext.LineError
)

// DependencyClearValue is the edit-only clear form's value.
const DependencyClearValue = capturetext.DependencyClearValue
