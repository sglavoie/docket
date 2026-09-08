package format

import "regexp"

// The RESUME delimiters live beside the DESCRIPTION and COMMENTS ones rather
// than inside the schema that introduced them, because a delimiter is the
// spelling of a block and the escaping that keeps prose out of it is shared.
// Which schemas admit the block is decided by the body reader that names these,
// which is schema 16's alone.
var (
	resumeStartRegex = regexp.MustCompile(`^<!--\s*RESUME\s*-->$`)
	resumeEndRegex   = regexp.MustCompile(`^<!--\s*/RESUME\s*-->$`)
)
