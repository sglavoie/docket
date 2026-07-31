package format

import "regexp"

var (
	decisionsStartRegex  = regexp.MustCompile(`^<!--\s*DECISIONS\s*-->$`)
	decisionsEndRegex    = regexp.MustCompile(`^<!--\s*/DECISIONS\s*-->$`)
	decisionHeaderRegex  = regexp.MustCompile(`^<!--\s*decision:\s*(.+?)\s*-->$`)
	rationaleMarkerRegex = regexp.MustCompile(`^<!--\s*rationale\s*-->$`)
)
