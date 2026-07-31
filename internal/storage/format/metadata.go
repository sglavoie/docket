package format

import "regexp"

var metadataRegex = regexp.MustCompile(`^<!--\s*(.+?)\s*-->\s*$`)

func MetadataRegex() *regexp.Regexp { return metadataRegex }
