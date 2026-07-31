package format

import "regexp"

var (
	descriptionStartRegex = regexp.MustCompile(`^<!--\s*DESCRIPTION\s*-->$`)
	descriptionEndRegex   = regexp.MustCompile(`^<!--\s*/DESCRIPTION\s*-->$`)
)
