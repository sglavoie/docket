package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

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
