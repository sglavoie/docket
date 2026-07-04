package cmd

// ExitCodeError carries a specific process exit code alongside an optional
// underlying error. Commands that need to distinguish failure modes (e.g. the
// hidden `tt tui-test` CLI mapping scenario failures vs. invalid input to
// different codes) return one of these; main.go reads Code() to set the exit
// status. When Err is nil the process exits with Code without printing anything,
// which lets a command that already wrote its own machine-readable output on
// stdout signal failure without an extra stderr line.
type ExitCodeError struct {
	Code int
	Err  error
}

// Error returns the underlying error's message, or an empty string when there
// is no underlying error. main.go suppresses the stderr line for empty messages.
func (e *ExitCodeError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// Unwrap exposes the wrapped error for errors.Is/As.
func (e *ExitCodeError) Unwrap() error {
	return e.Err
}
