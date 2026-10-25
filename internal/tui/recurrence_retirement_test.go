package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/gotree"
)

func TestRecurrenceVocabularyIsConfinedToMigrationAndRejectedInput(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate recurrence retirement audit")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	allowedProductionFiles := map[string]string{
		"internal/config/pre_consolidation_v6_snapshot.go":       "immutable migration audit",
		"internal/config/pre_consolidation_v6_snapshot_audit.go": "immutable migration audit",
		"internal/storage/format/legacy_v4_task.go":              "schema-4 migration reader",
		"internal/storage/format/legacy_v4_task_convert.go":      "schema-4 to schema-5 migration",
		"internal/storage/format/v5_task_codec.go":               "schema-5 migration codec",
		"internal/storage/format/v5_task_fields.go":              "schema-5 migration codec",
		"internal/storage/format/v5_task_validate.go":            "schema-5 migration codec",
		"internal/storage/format/v6_task_convert.go":             "schema-5 to schema-6 migration",
		"internal/storage/format/legacy_v4_metadata_decode.go":   "schema-4 to schema-5 migration",
		"internal/storage/v5_archive_reintegration.go":           "schema-5 migration reintegration",
		"internal/storage/v5_archive_reintegration_refs.go":      "schema-5 migration reintegration",
	}

	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != repositoryRoot && gotree.Ignored(entry.Name()) {
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
		relativePath = filepath.ToSlash(relativePath)
		if _, allowed := allowedProductionFiles[relativePath]; allowed {
			return nil
		}
		if containsRecurrenceVocabulary(relativePath) {
			violations = append(violations, relativePath+": production filename")
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			var value string
			switch node := node.(type) {
			case *ast.Ident:
				value = node.Name
			case *ast.BasicLit:
				if node.Kind != token.STRING {
					return true
				}
				unquoted, unquoteErr := strconv.Unquote(node.Value)
				if unquoteErr != nil {
					value = node.Value
				} else {
					value = unquoted
				}
			default:
				return true
			}
			if containsRecurrenceVocabulary(value) {
				position := fset.Position(node.Pos())
				violations = append(violations, relativePath+":"+strconv.Itoa(position.Line)+": "+value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go sources: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("recurrence returned outside migration or rejected-input guidance:\n%s", strings.Join(violations, "\n"))
	}
}

func containsRecurrenceVocabulary(value string) bool {
	if strings.Contains(value, "Recurr") || strings.Contains(value, "RECURR") {
		return true
	}
	lower := strings.ToLower(value)
	for offset := 0; ; {
		index := strings.Index(lower[offset:], "recur")
		if index < 0 {
			return false
		}
		index += offset
		if index == 0 || lower[index-1] < 'a' || lower[index-1] > 'z' {
			return true
		}
		offset = index + len("recur")
	}
}
