package tui

import "regexp"

// taskIDPattern matches queries like "#123" for direct task ID jump.
var taskIDPattern = regexp.MustCompile(`^#(\d+)$`)

// inlineTaskPattern matches queries starting with "+" for inline task creation.
var inlineTaskPattern = regexp.MustCompile(`^\+(.+)$`)

// quotedPhrasePattern matches quoted phrases: "exact phrase" or 'exact phrase'
var quotedPhrasePattern = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)
