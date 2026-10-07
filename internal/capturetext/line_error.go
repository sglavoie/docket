package capturetext

// LineError is one refusal reported against one input line. Both the multiline
// composer and the outline buffer validate every nonblank line before they
// commit anything, so a refusal has to name the line it came from rather than
// stop at the first problem.
type LineError struct {
	Line    int
	Column  int // zero-based rune column; zero when no token owns the refusal
	Message string
}
