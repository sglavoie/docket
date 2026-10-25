package synclog

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// HeldError is why a manifest is held (ErrHeld): the format version, schemas,
// or capabilities a device would need to read it. Both sides of a schema
// difference are kept, because a held manifest is reported with which device
// has to be updated, and that depends on which side is behind.
type HeldError struct {
	// FormatVersion is the manifest's format version when this device does not
	// support it, or zero.
	FormatVersion int
	// Schemas are the schema keys on which the manifest and this device differ.
	Schemas []SchemaDifference
	// Missing are the capabilities the manifest requires and this device lacks.
	Missing []string
}

// SchemaDifference is one schema key a manifest and the reading device write
// at different versions.
type SchemaDifference struct {
	Key      string `json:"key"`
	Manifest int    `json:"manifest"`
	Device   int    `json:"device"`
}

func (e *HeldError) Error() string {
	switch {
	case e.FormatVersion != 0:
		return fmt.Sprintf("%v: format_version %d is newer than %d", ErrHeld, e.FormatVersion, ManifestFormatVersion)
	case len(e.Schemas) > 0:
		parts := make([]string, 0, len(e.Schemas))
		for _, diff := range e.Schemas {
			parts = append(parts, fmt.Sprintf("%s %d (this device %d)", diff.Key, diff.Manifest, diff.Device))
		}
		return fmt.Sprintf("%v: schemas differ: %s", ErrHeld, strings.Join(parts, ", "))
	default:
		return fmt.Sprintf("%v: missing capabilities %s", ErrHeld, strings.Join(e.Missing, ", "))
	}
}

func (e *HeldError) Unwrap() error { return ErrHeld }

// UpgradeAuthor reports that the author, not this device, is behind: every
// difference is a schema the manifest writes older than this device does, and
// it needs nothing this device lacks.
func (e *HeldError) UpgradeAuthor() bool {
	if e.FormatVersion != 0 || len(e.Missing) > 0 || len(e.Schemas) == 0 {
		return false
	}
	for _, diff := range e.Schemas {
		if diff.Manifest >= diff.Device {
			return false
		}
	}
	return true
}

// RequiredUpgrade names the update a held manifest is waiting for, in the
// words a status surface shows: which device has to update Docket, and the schema
// or capability that decides it. It is empty for an error that is not a hold.
// A held manifest is never lost: it waits, and so does everything after it
// from the same author, until that update.
func RequiredUpgrade(err error, author string) string {
	var held *HeldError
	if !errors.As(err, &held) {
		return ""
	}
	if held.UpgradeAuthor() {
		return fmt.Sprintf("update Docket on device %s: it writes %s", author, held.schemaSummary())
	}
	switch {
	case held.FormatVersion != 0:
		return fmt.Sprintf("update Docket on this device: device %s writes sync format %d, this device reads %d",
			author, held.FormatVersion, ManifestFormatVersion)
	case len(held.Schemas) > 0:
		return fmt.Sprintf("update Docket on this device: device %s writes %s", author, held.schemaSummary())
	default:
		return fmt.Sprintf("update Docket on this device: device %s requires %s", author, strings.Join(held.Missing, ", "))
	}
}

// schemaSummary spells the differing schemas, the author's version first.
func (e *HeldError) schemaSummary() string {
	parts := make([]string, 0, len(e.Schemas))
	for _, diff := range e.Schemas {
		relation := "older than"
		if diff.Manifest > diff.Device {
			relation = "newer than"
		}
		parts = append(parts, fmt.Sprintf("%s schema %d, %s this device's %d",
			strings.ReplaceAll(diff.Key, "_", " "), diff.Manifest, relation, diff.Device))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
