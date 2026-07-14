package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// findSavedTask returns the persisted copy of a task after runAttach reloads,
// mutates, and saves its own project instance. Inspecting the in-memory task
// returned by setupAttachmentTest would miss those side effects.
func findSavedTask(t *testing.T, deps *testDeps, projectID, taskID string) *models.Task {
	t.Helper()
	saved := deps.Repository.Projects[projectID]
	if saved == nil {
		t.Fatalf("saved project %q not found", projectID)
	}
	for _, st := range saved.Tasks {
		if st.ID == taskID {
			return st
		}
	}
	t.Fatalf("saved task %q not found", taskID)
	return nil
}

// TestRunAttach_URLRejectsMissingScheme guards against accepting non-URL strings
// via --url. Previously, any string (e.g. "example.com" or "not a url") was
// silently stored as a URL attachment; later, opening it would fail with a
// confusing OS-level "file not found" error since the system opener tries to
// interpret schemeless input as a local path.
func TestRunAttach_URLRejectsMissingScheme(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"plain text", "not a url"},
		{"domain without scheme", "example.com"},
		{"relative path", "/var/log/something"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, project := setupAttachmentTest(t)
			defer deps.Cleanup()

			task := project.Tasks[0]
			taskID := fmt.Sprintf("%d", task.ShortID)

			attachURL = tc.url
			defer func() { attachURL = "" }()

			err := runAttach(attachCmd, []string{taskID})
			if err == nil {
				t.Fatalf("expected error attaching schemeless URL %q", tc.url)
			}
			if !strings.Contains(err.Error(), "scheme") && !strings.Contains(err.Error(), "URL") {
				t.Errorf("expected error to mention scheme/URL, got %v", err)
			}
			// The attachment must not have been added on the task.
			if len(task.Attachments) != 0 {
				t.Errorf("expected no attachments after rejection, found %d", len(task.Attachments))
			}
		})
	}
}

// TestRunAttach_URLAcceptsValidSchemes confirms that the scheme check does not
// over-reject legitimate URL forms.
func TestRunAttach_URLAcceptsValidSchemes(t *testing.T) {
	cases := []string{
		"https://example.com/path",
		"http://localhost:8080",
		"mailto:foo@example.com",
		"file:///tmp/foo.txt",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			deps, project := setupAttachmentTest(t)
			defer deps.Cleanup()

			task := project.Tasks[0]
			taskID := fmt.Sprintf("%d", task.ShortID)

			attachURL = raw
			defer func() { attachURL = "" }()

			if err := runAttach(attachCmd, []string{taskID}); err != nil {
				t.Fatalf("unexpected error attaching valid URL %q: %v", raw, err)
			}
		})
	}
}

// TestRunAttach_RejectsDirectory guards against passing a directory path as the
// source. Previously the check only matched os.IsNotExist, so a directory slipped
// through and CopyAttachment failed downstream with an opaque "is a directory"
// io error.
func TestRunAttach_RejectsDirectory(t *testing.T) {
	deps, project := setupAttachmentTest(t)
	defer deps.Cleanup()

	srcDir := t.TempDir()

	task := project.Tasks[0]
	taskID := fmt.Sprintf("%d", task.ShortID)

	attachURL = ""

	err := runAttach(attachCmd, []string{taskID, srcDir})
	if err == nil {
		t.Fatal("expected error when attaching a directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("expected error to mention 'directory', got %v", err)
	}
}

// TestRunAttach_FileWithReservedCharLeavesNoOrphan guards against orphaning a
// copied attachment when the filename fails post-copy validation. The task's
// AddAttachment method rejects filenames containing reserved delimiters ("||"
// or ";"); attach.go must not leave the file in the .attachments directory
// after such a failure.
func TestRunAttach_FileWithReservedCharLeavesNoOrphan(t *testing.T) {
	deps, project := setupAttachmentTest(t)
	defer deps.Cleanup()

	// Create a source file whose basename contains a semicolon — a reserved
	// attachment-metadata delimiter that AddAttachment will reject.
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "bad;name.txt")
	if err := os.WriteFile(srcPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("seed source file: %v", err)
	}

	task := project.Tasks[0]
	taskID := fmt.Sprintf("%d", task.ShortID)

	// Reset URL flag to ensure file-path mode.
	attachURL = ""

	err := runAttach(attachCmd, []string{taskID, srcPath})
	if err == nil {
		t.Fatal("expected error attaching file with reserved character in name")
	}

	// The copied file must not remain in the task's attachments directory.
	attachmentsDir := storage.GetTaskAttachmentsDir(project, task)
	entries, statErr := os.ReadDir(attachmentsDir)
	if statErr != nil && !os.IsNotExist(statErr) {
		t.Fatalf("read attachments dir: %v", statErr)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected no orphan files in %s, found: %v", attachmentsDir, names)
	}
}

// TestRunAttach_NormalizesPastedPaths verifies that pasted shell-style file
// paths are normalized before routing. A path pasted with backslash-escaped
// spaces (e.g. "/tmp/my\ file.txt") or split across multiple args after the
// task id must collapse to the real path so the file is copied successfully.
func TestRunAttach_NormalizesPastedPaths(t *testing.T) {
	cases := []struct {
		name     string
		basename string
		// buildArgs turns the task id and copied source path into the args passed
		// to runAttach, simulating how the shell would deliver a pasted path.
		buildArgs func(taskID, srcPath string) []string
	}{
		{
			name:     "backslash-escaped space in single arg",
			basename: "my file.txt",
			buildArgs: func(taskID, srcPath string) []string {
				// Emulate a literal "\ " escape sequence in the pasted single token.
				escaped := strings.ReplaceAll(srcPath, " ", `\ `)
				return []string{taskID, escaped}
			},
		},
		{
			name:     "path split across multiple args",
			basename: "my file.txt",
			buildArgs: func(taskID, srcPath string) []string {
				// A shell that word-splits an unquoted spaced path delivers the
				// path as several args after the task id.
				parts := strings.Split(srcPath, " ")
				return append([]string{taskID}, parts...)
			},
		},
		{
			name:     "single-quoted path",
			basename: "my file.txt",
			buildArgs: func(taskID, srcPath string) []string {
				return []string{taskID, "'" + srcPath + "'"}
			},
		},
		{
			name:     "plain path without normalization",
			basename: "normal.txt",
			buildArgs: func(taskID, srcPath string) []string {
				return []string{taskID, srcPath}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, project := setupAttachmentTest(t)
			defer deps.Cleanup()

			srcDir := t.TempDir()
			srcPath := filepath.Join(srcDir, tc.basename)
			if err := os.WriteFile(srcPath, []byte("payload"), 0o600); err != nil {
				t.Fatalf("seed source file: %v", err)
			}

			task := project.Tasks[0]
			taskID := fmt.Sprintf("%d", task.ShortID)

			attachURL = ""

			args := tc.buildArgs(taskID, srcPath)
			if err := runAttach(attachCmd, args); err != nil {
				t.Fatalf("runAttach(%v) unexpected error: %v", args, err)
			}

			saved := findSavedTask(t, deps, project.ID, task.ID)
			if len(saved.Attachments) != 1 {
				t.Fatalf("expected 1 attachment, got %d", len(saved.Attachments))
			}
		})
	}
}

// TestRunAttach_NormalizesEscapedSpaceCodeReference verifies that a code
// reference pasted with backslash-escaped spaces is normalized before code-ref
// detection, so "/tmp/my\ file.go:42" is recognized as a valid code reference.
func TestRunAttach_NormalizesEscapedSpaceCodeReference(t *testing.T) {
	deps, project := setupAttachmentTest(t)
	defer deps.Cleanup()

	task := project.Tasks[0]
	taskID := fmt.Sprintf("%d", task.ShortID)

	attachURL = ""

	// Code references are stored without requiring the file to exist.
	escaped := `/tmp/my\ file.go:42`
	if err := runAttach(attachCmd, []string{taskID, escaped}); err != nil {
		t.Fatalf("runAttach code reference unexpected error: %v", err)
	}

	saved := findSavedTask(t, deps, project.ID, task.ID)
	if len(saved.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(saved.Attachments))
	}
	att := saved.Attachments[0]
	if !att.IsCodeRef {
		t.Errorf("expected a code reference attachment, got %+v", att)
	}
	if !strings.Contains(att.Filename, "my file.go") {
		t.Errorf("expected normalized code-ref path to contain 'my file.go', got %q", att.Filename)
	}
}
