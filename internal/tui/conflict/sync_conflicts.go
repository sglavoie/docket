package conflict

import (
	"fmt"
	"strings"
	"time"
)

// SyncConflictEntry is one open sync conflict as the browser lists it: the
// file whose versions compete, the version they started from, and each
// competing version with the devices' changes that left it. The shell
// converts the engine's conflict views into these, so this package stays a
// projection that never reads the history.
type SyncConflictEntry struct {
	ID    string
	Path  string
	Kind  string
	Label string
	// Base is the content every version started from, line by line.
	Base     []string
	Versions []SyncConflictVersion
	// Result is the digest of the actual file currently in the workspace.
	Result      string
	Details     []string
	BaseText    *string
	CurrentText *string
	Replaceable bool
}

// SyncConflictVersion is one competing version: its content digest, or empty
// for a deletion, its content, and the changes that wrote it.
type SyncConflictVersion struct {
	Index   int
	Text    *string
	Blob    string
	Content []string
	Current bool
	Writers []SyncConflictWriter
	// AlsoResolves names the other conflicted files the same change left,
	// which keeping this version keeps too.
	AlsoResolves []string
}

// SyncConflictWriter is one change that wrote a version: which device, what
// kind of change, and when.
type SyncConflictWriter struct {
	TxnID      string
	Device     string
	DeviceName string
	ThisDevice bool
	Change     string
	At         string
}

func syncConflictShort(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	return conflictBrowserShortID(id)
}

// syncConflictRowTitle leads with the conflict's short id, like a bundle row,
// and names the file in place of a project.
func syncConflictRowTitle(entry SyncConflictEntry) string {
	return syncConflictShort(entry.ID) + "  " + entry.Path
}

func syncConflictRowSummary(entry SyncConflictEntry) string {
	return fmt.Sprintf("sync conflict · %d versions · %s", len(entry.Versions), entry.Kind)
}

// syncConflictSummaryLines answers the decision before showing file contents
// or diagnostics. Every version's coupled effects remain beside its author.
func syncConflictSummaryLines(entry SyncConflictEntry) []string {
	lines := []string{entry.Label, fmt.Sprintf("%d versions need a choice; no version was chosen by time.", len(entry.Versions))}
	if len(entry.Details) > 0 {
		lines = append(lines, "", "What differs:")
		for _, detail := range entry.Details {
			lines = append(lines, "  "+detail)
		}
	}
	for i, version := range entry.Versions {
		lines = append(lines, "", fmt.Sprintf("Version %d", i+1))
		lines = append(lines, syncConflictVersionSummary(version)...)
	}
	return append(lines, "", "d shows changes · [/] selects a version", "Every version is kept in the sync history; nothing was lost.")
}

func syncConflictVersionSummary(version SyncConflictVersion) []string {
	var lines []string
	if version.Current {
		lines = append(lines, "  Shown on this device now")
	}
	if version.Blob == "" {
		lines = append(lines, "  Deletes this file")
	}
	for _, writer := range version.Writers {
		device := writer.DeviceName
		if device == "" {
			device = writer.Device
		}
		if device == "" {
			device = "an unnamed device"
		}
		if writer.ThisDevice {
			device += " (this device)"
		}
		change := writer.Change
		if change == "" {
			change = "change"
		}
		line := "  " + change + " on " + device
		if instant, err := time.Parse(time.RFC3339Nano, writer.At); err == nil {
			line += " · " + instant.Local().Format("Jan 2, 2006 15:04 MST")
		} else if writer.At != "" {
			line += " · " + writer.At
		}
		lines = append(lines, line)
	}
	for _, also := range version.AlsoResolves {
		lines = append(lines, "  Keeping this version also resolves: "+also)
	}
	return lines
}

// Whole files and exact identifiers stay available below the decision summary.
func syncConflictDetailLines(entry SyncConflictEntry) []string {
	lines := syncConflictSummaryLines(entry)
	lines = append(lines, "", "File contents", "Started from:")
	lines = append(lines, syncConflictBody(entry.Base)...)
	for i, version := range entry.Versions {
		lines = append(lines, "", fmt.Sprintf("Version %d contents:", i+1))
		lines = append(lines, syncConflictBody(version.Content)...)
	}
	lines = append(lines, "", "Diagnostics:",
		"Sync conflict "+strings.TrimPrefix(entry.ID, "sha256:"),
		"  File:      "+entry.Path, "  Kind:      "+entry.Kind,
		"  Holding:   "+syncConflictBlob(entry.Result))
	for i, version := range entry.Versions {
		heading := fmt.Sprintf("  [%d] %s", i+1, syncConflictBlob(version.Blob))
		if version.Current {
			heading += " · held meanwhile"
		}
		lines = append(lines, heading)
		for _, writer := range version.Writers {
			lines = append(lines, "      "+syncConflictWriterLine(writer))
		}
	}
	hint := "v keep selected version · c keep current"
	if entry.Replaceable {
		hint += " · e edit merged result"
	}
	return append(lines, "", hint)
}

func syncConflictWriterLine(writer SyncConflictWriter) string {
	device := writer.Device
	if writer.DeviceName != "" {
		device = writer.DeviceName + " (" + writer.Device + ")"
	}
	if writer.ThisDevice {
		device += ", this device"
	}
	change := writer.Change
	if change == "" {
		change = "change"
	}
	line := change + " on " + device
	if writer.At != "" {
		line += " at " + writer.At
	}
	return line + " · " + syncConflictShort(writer.TxnID)
}

func syncConflictBody(content []string) []string {
	out := make([]string, 0, len(content))
	for _, line := range content {
		out = append(out, "      │ "+line)
	}
	return out
}

func syncConflictBlob(digest string) string {
	if digest == "" {
		return "deleted"
	}
	return "content " + syncConflictShort(digest)
}
