package storage

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

func TestConvertedDecisionCommentUsesOrdinaryLivePaths(t *testing.T) {
	stamp := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	converted, err := format.ConvertV5Task(format.V5TaskRecord{
		ID:        "converted-task",
		ShortID:   1,
		Title:     "Choose storage",
		State:     "ready",
		Priority:  3,
		CreatedAt: stamp,
		UpdatedAt: stamp,
		Comments: []format.V5TaskComment{
			{Text: "Existing comment", CreatedAt: stamp.Add(-time.Minute)},
		},
		Decisions: []format.V5TaskDecision{
			{Text: "Use SQLite", Rationale: "Local-first durability", CreatedAt: stamp},
		},
	})
	if err != nil {
		t.Fatalf("ConvertV5Task: %v", err)
	}
	if _, ok := reflect.TypeOf(models.Task{}).FieldByName("Decisions"); ok {
		t.Fatal("live Task still exposes decision records")
	}

	projectDocument := V8ProjectDocumentFromV7(V7ProjectDocument{
		ProjectName: "Converted",
		Tasks:       []format.V6TaskRecord{converted.Task},
	})
	tasks, err := modelsFromV9ProjectDocument(projectDocument, time.UTC)
	if err != nil {
		t.Fatalf("modelsFromV9ProjectDocument: %v", err)
	}
	if len(tasks) != 1 || len(tasks[0].Comments) != 2 {
		t.Fatalf("converted live comments = %+v", tasks)
	}
	wantConvertedText := "Decision: Use SQLite\nRationale: Local-first durability"
	if got := tasks[0].Comments[1]; got.Text != wantConvertedText || !got.CreatedAt.Equal(stamp) {
		t.Fatalf("converted comment = %+v, want text %q at %v", got, wantConvertedText, stamp)
	}

	live := &models.Project{Name: "Converted", Tasks: tasks}
	encoded, err := ToProjectDocument(live).WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	loaded, err := ParseProjectDocument(encoded)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if got := loaded.Tasks[0].Comments[1]; got.Text != wantConvertedText || !got.CreatedAt.Equal(stamp) {
		t.Fatalf("load/save changed converted comment: %+v", got)
	}

	base := cloneDecisionRetirementDocument(t, loaded)
	local := cloneDecisionRetirementDocument(t, loaded)
	current := cloneDecisionRetirementDocument(t, loaded)
	local.Tasks[0].Comments[1].Text = "Decision: Use SQLite\nRationale: Embedded deployment"
	local.Tasks[0].UpdatedAt = stamp.Add(time.Minute)
	current.Tasks[0].Description = "Changed concurrently"
	current.Tasks[0].UpdatedAt = stamp.Add(2 * time.Minute)
	merged, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if merged.HasConflicts() || merged.Merged == nil {
		t.Fatalf("ordinary comment merge conflicted: %+v", merged.Conflicts)
	}
	mergedTask := merged.Merged.Tasks[0]
	if got := mergedTask.Comments[1].Text; got != local.Tasks[0].Comments[1].Text {
		t.Fatalf("merged converted comment = %q, want %q", got, local.Tasks[0].Comments[1].Text)
	}
	if mergedTask.Description != current.Tasks[0].Description {
		t.Fatalf("independent field edit was lost: %q", mergedTask.Description)
	}

	exported := WriteProjectMarkdownClean(&models.Project{Name: "Converted", Tasks: merged.Merged.Tasks})
	for _, want := range []string{"**Comments:**", "Decision: Use SQLite", "Rationale: Embedded deployment"} {
		if !strings.Contains(exported, want) {
			t.Fatalf("clean export missing %q:\n%s", want, exported)
		}
	}
	if strings.Contains(exported, "**Decisions:**") {
		t.Fatalf("clean export restored a decision section:\n%s", exported)
	}

	testConvertedCommentUndoRedo(t, converted.Task, wantConvertedText)
}

func testConvertedCommentUndoRedo(t *testing.T, record format.V6TaskRecord, originalText string) {
	t.Helper()
	dataDir := t.TempDir()
	ledgerDir := filepath.Join(dataDir, ".tt", "undo")
	ledger := NewWorkspaceLedger(ledgerDir, []string{dataDir})
	service := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project := newCommitTestProject(t, dataDir, "converted", record.Title)
	tasks, err := modelsFromV9ProjectDocument(V8ProjectDocumentFromV7(V7ProjectDocument{ProjectName: project.Name, Tasks: []format.V6TaskRecord{record}}), time.UTC)
	if err != nil {
		t.Fatalf("modelsFromV9ProjectDocument: %v", err)
	}
	project.Tasks = tasks
	project.BuildTaskTree()

	created, err := service.Commit(ChangeSet{Project: project, UndoAction: "create converted comment"})
	if err != nil {
		t.Fatalf("initial Commit: %v", err)
	}
	base, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	project.Tasks[0].Comments[1].Text = "Decision: Use SQLite\nRationale: Edited as a comment"
	project.Tasks[0].UpdatedAt = project.Tasks[0].UpdatedAt.Add(time.Minute)
	if _, err := service.Commit(ChangeSet{
		Project:          project,
		ExpectedRevision: created.Revision,
		Base:             base,
		UndoAction:       "edit converted comment",
	}); err != nil {
		t.Fatalf("edit Commit: %v", err)
	}

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	assertDecisionRetirementLoadedComment(t, project, originalText)
	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	assertDecisionRetirementLoadedComment(t, project, "Decision: Use SQLite\nRationale: Edited as a comment")
}

func assertDecisionRetirementLoadedComment(t *testing.T, project *models.Project, want string) {
	t.Helper()
	loaded, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	loaded.Slug = project.Slug
	if err := LoadProject(loaded); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(loaded.Tasks) != 1 || len(loaded.Tasks[0].Comments) != 2 || loaded.Tasks[0].Comments[1].Text != want {
		t.Fatalf("loaded comments = %+v, want second text %q", loaded.Tasks, want)
	}
}

func cloneDecisionRetirementDocument(t *testing.T, document *ProjectDocument) *ProjectDocument {
	t.Helper()
	content, err := document.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent clone: %v", err)
	}
	cloned, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument clone: %v", err)
	}
	return cloned
}

func TestDecisionRetirementBackendInventory(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate decision retirement audit")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	allowedMigrationFiles := map[string]bool{
		"internal/config/pre_consolidation_v6_snapshot.go":  true,
		"internal/storage/format/decisions.go":              true,
		"internal/storage/format/legacy_v4_task.go":         true,
		"internal/storage/format/legacy_v4_task_convert.go": true,
		"internal/storage/format/v5_task_codec.go":          true,
		"internal/storage/format/v6_task_convert.go":        true,
		"internal/storage/v5_archive_reintegration.go":      true,
	}
	scanRoots := []string{"cmd", "internal/app", "internal/config", "internal/models", "internal/storage", "internal/workspace"}

	fset := token.NewFileSet()
	var violations []string
	for _, scanRoot := range scanRoots {
		root := filepath.Join(repositoryRoot, filepath.FromSlash(scanRoot))
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relativePath, relErr := filepath.Rel(repositoryRoot, path)
			if relErr != nil {
				return relErr
			}
			relativePath = filepath.ToSlash(relativePath)
			if allowedMigrationFiles[relativePath] {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(file, func(node ast.Node) bool {
				identifier, isIdentifier := node.(*ast.Ident)
				if !isIdentifier || !strings.Contains(strings.ToLower(identifier.Name), "decision") {
					return true
				}
				position := fset.Position(identifier.Pos())
				violations = append(violations, relativePath+":"+position.String()+": "+identifier.Name)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", scanRoot, err)
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("decision records returned outside migration code:\n%s", strings.Join(violations, "\n"))
	}
}

func TestDecisionMetadataCannotEnterLiveStorageOrHistory(t *testing.T) {
	dataDir := t.TempDir()
	ledgerDir := filepath.Join(dataDir, ".tt", "undo")
	ledger := NewWorkspaceLedger(ledgerDir, []string{dataDir})
	service := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project := newCommitTestProject(t, dataDir, "retirement", "No decision records")
	project.Tasks[0].Comments = []*models.Comment{{Text: "Decision: retained only as ordinary comment", CreatedAt: time.Now().UTC()}}
	if _, err := service.Commit(ChangeSet{Project: project, UndoAction: "create task"}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	assertTreeHasNoDecisionMetadata(t, project.Path)
	assertTreeHasNoDecisionMetadata(t, ledgerDir)
}

func assertTreeHasNoDecisionMetadata(t *testing.T, root string) {
	t.Helper()
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"<!-- decisions -->", "<!-- decision:", `"decisions"`} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%s contains retired decision metadata %q", path, forbidden)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("inspect %s: %v", root, err)
	}
}
