package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// This file is the negative half of the release. Every other test in it proves
// something the release added; these four prove the four things it was required
// not to add. A release that grew an eighth mode, moved a schema, spilled Peek's
// page onto disk, or started storing recurrence would still pass every positive
// test, because each of those is a capability nothing asked about.

// TestReleaseAddedNoOutlinerMode holds the catalog to the seven modes and their
// order, which is the Tab cycle and the 1-7 digit row at once. A new entry here
// is a new product surface with its own grouping, footer, and help; it is a
// decision, never a side effect of a feature.
func TestReleaseAddedNoOutlinerMode(t *testing.T) {
	want := []chrome.OutlinerModeID{
		chrome.ModeAll,
		chrome.ModeToday,
		chrome.ModeInbox,
		chrome.ModeNext,
		chrome.ModeBacklog,
		chrome.ModeDue,
		chrome.ModeRecent,
	}
	got := make([]chrome.OutlinerModeID, 0, len(chrome.OutlinerModeCatalog()))
	for _, mode := range chrome.OutlinerModeCatalog() {
		got = append(got, mode.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the mode catalog is %v, want the seven final modes %v", got, want)
	}
}

// TestReleaseAdvancedNoSchemaVersion pins both stored schemas. The release added
// comments, acknowledgements, checklist items, and repeats — every one of them
// writes through fields the current schemas already carry, so either number
// moving means a migration was written, and a migration is never incidental.
func TestReleaseAdvancedNoSchemaVersion(t *testing.T) {
	if got, want := storage.CurrentTaskSchemaVersion, 16; got != want {
		t.Errorf("live task Markdown is schema %d, want %d", got, want)
	}
	if got, want := config.CurrentConfigVersion, 18; got != want {
		t.Errorf("the catalog is schema %d, want %d", got, want)
	}
}

// machineStateJSONTags is the whole of what a machine keeps for itself. The set
// is spelled out rather than counted so that adding a key requires naming it,
// and so that the one thing this release must not have added — a Peek page, a
// scroll offset, a last-viewed task — cannot arrive as an unnoticed extra field.
var machineStateJSONTags = []string{
	"active_lens_id",
	"advisories_hidden",
	"lenses_adopted",
	"menu_bar_visible",
	"named_lenses",
	"theme",
}

// TestReleasePersistsNoPeekState is the durability half. Peek's page is keyed by
// task identity, document fingerprint, and terminal size, none of which survive
// a process; a page that reached disk would reopen stale against a document that
// had moved underneath it. The audit is structural on both sides: the
// machine-local file's key set is exactly the six chrome and lens keys, and no
// persistence-layer identifier or literal names Peek at all.
func TestReleasePersistsNoPeekState(t *testing.T) {
	stateType := reflect.TypeOf(config.MachineState{})
	tags := make([]string, 0, stateType.NumField())
	for index := range stateType.NumField() {
		tag := stateType.Field(index).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		tags = append(tags, name)
	}
	sort.Strings(tags)
	if !reflect.DeepEqual(tags, machineStateJSONTags) {
		t.Errorf("machine-local state persists %v, want exactly %v", tags, machineStateJSONTags)
	}

	found := scanProductionSources(t, func(relativePath string, file *ast.File, fset *token.FileSet) []string {
		if !strings.HasPrefix(relativePath, "internal/config/") && !strings.HasPrefix(relativePath, "internal/storage/") {
			return nil
		}
		return collectMatchingSourceWords(relativePath, file, fset, func(word string) bool {
			return word == "peek"
		})
	})
	if len(found) > 0 {
		t.Errorf("the persistence layers named Peek, so a page may be reaching disk:\n%s", strings.Join(found, "\n"))
	}
}

// recurrenceMigrationFiles are the places the word survives on purpose. The v4
// and v5 decoders have to be able to read a recurrence a historical document
// stored, and the v6 converter and snapshot audit have to be able to say how
// many they discarded. Every one of them is a statement about what was thrown
// away; none of them is reachable from live code, which is the only reason the
// word is allowed to appear at all. The list is exhaustive by construction: a
// new file naming recurrence fails until somebody adds it here and says why.
var recurrenceMigrationFiles = map[string]bool{
	"internal/config/pre_consolidation_v6_snapshot_audit.go": true,
	"internal/storage/format/legacy_v4_metadata_decode.go":   true,
	"internal/storage/format/legacy_v4_task.go":              true,
	"internal/storage/format/legacy_v4_task_convert.go":      true,
	"internal/storage/format/v5_task_codec.go":               true,
	"internal/storage/format/v5_task_fields.go":              true,
	"internal/storage/format/v5_task_validate.go":            true,
	"internal/storage/format/v6_task_convert.go":             true,
	"internal/storage/v5_archive_reintegration_refs.go":      true,
}

// TestReleaseGrewNoRecurringTaskMachinery fences the vocabulary a scheduler
// would need. `D` repeats a task by opening a draft the user still has to save,
// so "repeat" is live and deliberately unfenced; what the product does not have
// is a rule that fires on its own. A recurrence field, an interval, or a next-run
// date is the difference, and each of them has to be spelled to exist.
func TestReleaseGrewNoRecurringTaskMachinery(t *testing.T) {
	found := scanProductionSources(t, func(relativePath string, file *ast.File, fset *token.FileSet) []string {
		if recurrenceMigrationFiles[relativePath] {
			return nil
		}
		return collectMatchingSourceWords(relativePath, file, fset, func(word string) bool {
			return strings.HasPrefix(word, "recur") || word == "rrule" || word == "cron"
		})
	})
	if len(found) > 0 {
		t.Errorf("recurring-task machinery appeared outside the migration audit:\n%s", strings.Join(found, "\n"))
	}
}

// collectMatchingSourceWords reports every identifier and string literal in one
// parsed file that the predicate rejects, located for the failure message. The
// predicate reads whole words, not substrings: a fence spelled as a substring
// catches "Cursor" for "recurs" and "ColorRule" for "rrule", and a sweep that
// cries wolf is one somebody eventually deletes.
//
// Comments are not read: parsing without ParseComments leaves them out of the
// AST, which is what lets prose say "the one recurring app timer" without
// tripping a sweep about task recurrence.
func collectMatchingSourceWords(relativePath string, file *ast.File, fset *token.FileSet, reject func(word string) bool) []string {
	var found []string
	rejects := func(value string) bool {
		for _, word := range splitSourceWords(value) {
			if reject(word) {
				return true
			}
		}
		return false
	}
	record := func(pos token.Pos, detail string) {
		found = append(found, relativePath+":"+strconv.Itoa(fset.Position(pos).Line)+": "+detail)
	}
	if rejects(filepath.Base(relativePath)) {
		found = append(found, relativePath+": production filename")
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Ident:
			if rejects(node.Name) {
				record(node.Pos(), node.Name)
			}
		case *ast.BasicLit:
			if node.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(node.Value)
			if unquoteErr != nil {
				value = node.Value
			}
			if rejects(value) {
				record(node.Pos(), value)
			}
		}
		return true
	})
	return found
}

// splitSourceWords breaks an identifier, filename, or literal into the lowercase
// words it is spelled from, cutting on camel-case humps and on everything that
// is not a letter. "OperationCoreCursorDown" is four words, none of which is a
// recurrence.
func splitSourceWords(value string) []string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			words = append(words, strings.ToLower(current.String()))
			current.Reset()
		}
	}
	previousUpper := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			if !previousUpper {
				flush()
			}
			current.WriteRune(r)
			previousUpper = true
		case r >= 'a' && r <= 'z':
			// A hump like "IDList" ends one word before its last capital, so the
			// trailing capital starts the next one rather than closing the last.
			if previousUpper && current.Len() > 1 {
				tail := current.String()
				current.Reset()
				current.WriteString(tail[:len(tail)-1])
				flush()
				current.WriteString(tail[len(tail)-1:])
			}
			current.WriteRune(r)
			previousUpper = false
		default:
			flush()
			previousUpper = false
		}
	}
	flush()
	return words
}

// scanProductionSources parses every non-test Go file in the repository and
// gathers what the inspector returns, sorted so a failure reads the same way
// twice. It is the shared walk behind the release fences; the per-audit decision
// about which files matter stays in the inspector.
func scanProductionSources(t *testing.T, inspect func(relativePath string, file *ast.File, fset *token.FileSet) []string) []string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the release shape audit")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))

	fset := token.NewFileSet()
	var found []string
	read := 0
	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relativePath, relErr := filepath.Rel(repositoryRoot, path)
		if relErr != nil {
			return relErr
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		read++
		found = append(found, inspect(filepath.ToSlash(relativePath), file, fset)...)
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go sources: %v", err)
	}
	if read == 0 {
		t.Fatal("the sweep read no production Go file")
	}
	sort.Strings(found)
	return found
}
