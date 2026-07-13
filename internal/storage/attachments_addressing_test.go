package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestTaskAttachmentsDirIsHashed verifies the on-disk directory is addressed by
// sha256(task.ID), not the raw ID, so short-ID reuse and crafted IDs cannot
// clobber or influence the layout.
func TestTaskAttachmentsDirIsHashed(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	dir := GetTaskAttachmentsDir(project, task)
	name := filepath.Base(dir)

	sum := sha256.Sum256([]byte(task.ID))
	want := hex.EncodeToString(sum[:])
	if name != want {
		t.Fatalf("attachments dir name = %q, want sha256 address %q", name, want)
	}
	if name == task.ID {
		t.Fatal("attachments dir must not be the raw task ID")
	}
	if len(name) != sha256.Size*2 {
		t.Fatalf("hashed dir name length = %d, want %d", len(name), sha256.Size*2)
	}
	// The directory must live directly under the project's .attachments tree.
	wantParent := filepath.Join(project.Path, attachmentsDirName)
	if filepath.Dir(dir) != wantParent {
		t.Fatalf("attachments dir parent = %q, want %q", filepath.Dir(dir), wantParent)
	}
}

// TestCraftedTaskIDStaysContained verifies that a task ID full of traversal
// metacharacters is neutralized by hashing: writes land inside the project's
// .attachments tree and never escape it.
func TestCraftedTaskIDStaysContained(t *testing.T) {
	project := newTestProject(t)
	crafted := &models.Task{ID: "../../../../etc/passwd", Title: "Crafted"}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "payload.txt")
	if err := os.WriteFile(srcPath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	filename, err := CopyAttachment(project, crafted, srcPath)
	if err != nil {
		t.Fatalf("CopyAttachment for crafted ID failed: %v", err)
	}

	dest := GetAttachmentPath(project, crafted, filename)
	root := filepath.Join(project.Path, attachmentsDirName)
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("crafted-ID attachment escaped .attachments tree: %q", dest)
	}
	if _, statErr := os.Stat(dest); statErr != nil {
		t.Fatalf("crafted-ID attachment not written where expected: %v", statErr)
	}

	// Two distinct crafted IDs must resolve to distinct directories.
	other := &models.Task{ID: "../../../../etc/shadow", Title: "Other"}
	if GetTaskAttachmentsDir(project, crafted) == GetTaskAttachmentsDir(project, other) {
		t.Fatal("distinct crafted IDs collided to the same directory")
	}
}

// TestCopyAttachmentRejectsSymlinkedAttachmentsParent verifies containment is
// re-checked after joining: if the project's .attachments directory is a
// symlink that escapes the project, the resolver refuses to write through it.
func TestCopyAttachmentRejectsSymlinkedAttachmentsParent(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	outside := t.TempDir()
	attachRoot := filepath.Join(project.Path, attachmentsDirName)
	if err := os.Symlink(outside, attachRoot); err != nil {
		t.Fatalf("failed to symlink .attachments outside project: %v", err)
	}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "escape.txt")
	if err := os.WriteFile(srcPath, []byte("escape"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	if _, err := CopyAttachment(project, task, srcPath); err == nil {
		t.Fatal("expected CopyAttachment to reject a symlinked .attachments parent that escapes the project")
	}

	// Nothing should have been written into the escape target.
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatalf("failed to read outside dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("attachment leaked through symlink; outside dir has %d entries", len(entries))
	}
}

// TestAttachmentCollisionDistinctTasks verifies two different tasks never share
// an attachment directory even when they carry identically named files.
func TestAttachmentCollisionDistinctTasks(t *testing.T) {
	project := newTestProject(t)
	a := &models.Task{ID: "task-a-uuid", Title: "A"}
	b := &models.Task{ID: "task-b-uuid", Title: "B"}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "same.txt")
	if err := os.WriteFile(srcPath, []byte("shared name"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	if _, err := CopyAttachment(project, a, srcPath); err != nil {
		t.Fatalf("CopyAttachment for task A failed: %v", err)
	}
	// Same filename for a different task must not be blocked by A's copy.
	if _, err := CopyAttachment(project, b, srcPath); err != nil {
		t.Fatalf("CopyAttachment for task B failed: %v", err)
	}

	if GetTaskAttachmentsDir(project, a) == GetTaskAttachmentsDir(project, b) {
		t.Fatal("distinct tasks resolved to the same attachments dir")
	}
	if !AttachmentExists(project, a, "same.txt") || !AttachmentExists(project, b, "same.txt") {
		t.Fatal("both tasks should independently own their same-named attachment")
	}
}

// TestCopyAttachmentCleansUpPartialOnCopyError forces a mid-copy failure (the
// source is a directory, so io.Copy fails after the destination file is
// created) and verifies no partial file is left behind to block a retry.
func TestCopyAttachmentCleansUpPartialOnCopyError(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	// A directory opens fine but fails during io.Copy on read.
	srcDir := t.TempDir()
	badSource := filepath.Join(srcDir, "iamadir")
	if err := os.Mkdir(badSource, 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	if _, err := CopyAttachment(project, task, badSource); err == nil {
		t.Fatal("expected CopyAttachment to fail when the source is a directory")
	}

	// The partially created destination must have been removed.
	destPath := GetAttachmentPath(project, task, "iamadir")
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("partial destination file was not cleaned up: stat err = %v", statErr)
	}
}

// TestMoveTaskAttachmentsInterruptedLeavesSourceIntact verifies that a move
// blocked by a pre-existing destination directory (a simulated
// previously-interrupted move) fails without silently merging or losing the
// source files.
func TestMoveTaskAttachmentsInterruptedLeavesSourceIntact(t *testing.T) {
	src := newTestProject(t)
	dest := &models.Project{ID: "dest-project", Name: "Dest", Path: t.TempDir()}
	task := newTestTask()

	srcFile := t.TempDir()
	srcPath := filepath.Join(srcFile, "doc.txt")
	if err := os.WriteFile(srcPath, []byte("original"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if _, err := CopyAttachment(src, task, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}

	// Pre-create the destination dir to simulate a prior interrupted move.
	destDir := GetTaskAttachmentsDir(dest, task)
	if err := os.MkdirAll(destDir, DirPermissions); err != nil {
		t.Fatalf("failed to pre-create destination dir: %v", err)
	}

	if err := MoveTaskAttachments(src, dest, task); err == nil {
		t.Fatal("expected MoveTaskAttachments to fail when destination already exists")
	}

	// Source files must remain intact — the move must not have partially applied.
	if !AttachmentExists(src, task, "doc.txt") {
		t.Fatal("source attachment was lost after a rejected move")
	}
	got, err := os.ReadFile(GetAttachmentPath(src, task, "doc.txt"))
	if err != nil {
		t.Fatalf("failed to read source attachment: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("source content = %q, want %q", string(got), "original")
	}
}

// TestDiscoverLegacyAttachmentDirs verifies migration discovery finds raw-ID
// directories and skips sha256-addressed (current) ones.
func TestDiscoverLegacyAttachmentDirs(t *testing.T) {
	project := newTestProject(t)
	base := filepath.Join(project.Path, attachmentsDirName)
	if err := os.MkdirAll(base, DirPermissions); err != nil {
		t.Fatalf("failed to create .attachments: %v", err)
	}

	// A current (hashed) directory that must be ignored.
	current := newTestTask()
	if err := os.MkdirAll(GetTaskAttachmentsDir(project, current), DirPermissions); err != nil {
		t.Fatalf("failed to create hashed dir: %v", err)
	}

	// Two legacy raw-ID directories.
	legacyIDs := []string{"legacy-uuid-1234", "42"}
	for _, id := range legacyIDs {
		if err := os.MkdirAll(filepath.Join(base, id), DirPermissions); err != nil {
			t.Fatalf("failed to create legacy dir %q: %v", id, err)
		}
	}
	// A stray file must not be reported as a directory.
	if err := os.WriteFile(filepath.Join(base, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create stray file: %v", err)
	}

	found, err := DiscoverLegacyAttachmentDirs(project)
	if err != nil {
		t.Fatalf("DiscoverLegacyAttachmentDirs failed: %v", err)
	}

	gotIDs := make(map[string]bool)
	for _, d := range found {
		gotIDs[d.TaskID] = true
		if d.Path != filepath.Join(base, d.TaskID) {
			t.Errorf("legacy dir path = %q, want %q", d.Path, filepath.Join(base, d.TaskID))
		}
	}
	if len(found) != len(legacyIDs) {
		t.Fatalf("found %d legacy dirs, want %d (%v)", len(found), len(legacyIDs), gotIDs)
	}
	for _, id := range legacyIDs {
		if !gotIDs[id] {
			t.Errorf("legacy dir %q not discovered", id)
		}
	}
	// The hashed current directory must never be flagged as legacy.
	if gotIDs[filepath.Base(GetTaskAttachmentsDir(project, current))] {
		t.Error("hashed directory was misclassified as legacy")
	}
}

// TestDiscoverLegacyAttachmentDirsNoTree returns nothing (no error) when a
// project has never had any attachments.
func TestDiscoverLegacyAttachmentDirsNoTree(t *testing.T) {
	project := newTestProject(t)
	found, err := DiscoverLegacyAttachmentDirs(project)
	if err != nil {
		t.Fatalf("DiscoverLegacyAttachmentDirs on empty project failed: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected no legacy dirs, got %d", len(found))
	}
}

// TestIsHashedAttachmentDirName pins the address-classification helper that
// migration discovery relies on.
func TestIsHashedAttachmentDirName(t *testing.T) {
	valid := hex.EncodeToString(func() []byte { s := sha256.Sum256([]byte("x")); return s[:] }())
	cases := []struct {
		name string
		want bool
	}{
		{valid, true},
		{"legacy-uuid-1234", false},
		{"", false},
		{strings.ToUpper(valid), false},     // uppercase hex is not our address form
		{valid[:len(valid)-1] + "g", false}, // non-hex character
		{valid[:63], false},                 // too short
		{valid + "0", false},                // too long
	}
	for _, tc := range cases {
		if got := isHashedAttachmentDirName(tc.name); got != tc.want {
			t.Errorf("isHashedAttachmentDirName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
