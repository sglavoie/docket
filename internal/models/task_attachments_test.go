package models

import (
	"testing"
)

func Test_validateAttachmentPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid path", "/home/user/file.txt", false},
		{"valid URL", "https://example.com/file.pdf", false},
		{"path with delimiter", "/home/user/a||b.txt", true},
		{"empty path", "", false},
		{"path with semicolon", "/home/user/a;b.txt", true},
		{"URL with semicolon", "https://example.com/page?a=1;b=2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAttachmentPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAttachmentPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestAddAttachment(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Add a file attachment
	if err := task.AddAttachment("report.pdf", false, "/home/user/report.pdf"); err != nil {
		t.Fatalf("AddAttachment failed: %v", err)
	}
	if len(task.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(task.Attachments))
	}
	if task.Attachments[0].Filename != "report.pdf" {
		t.Errorf("expected filename 'report.pdf', got %q", task.Attachments[0].Filename)
	}
	if task.Attachments[0].IsURL {
		t.Error("expected IsURL false")
	}

	// Add a URL attachment
	if err := task.AddAttachment("https://example.com", true, "https://example.com"); err != nil {
		t.Fatalf("AddAttachment URL failed: %v", err)
	}
	if len(task.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(task.Attachments))
	}
	if !task.Attachments[1].IsURL {
		t.Error("expected IsURL true for URL attachment")
	}

	// Adding with invalid path should fail
	if err := task.AddAttachment("bad||name.txt", false, "/tmp/file.txt"); err == nil {
		t.Error("expected error for filename containing delimiter")
	}
	if err := task.AddAttachment("file.txt", false, "/tmp/a||b/file.txt"); err == nil {
		t.Error("expected error for original path containing delimiter")
	}
	// Attachment count should not have changed
	if len(task.Attachments) != 2 {
		t.Errorf("expected 2 attachments after failed adds, got %d", len(task.Attachments))
	}
}

func TestAddCodeReference(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	if err := task.AddCodeReference("/home/user/main.go", 42); err != nil {
		t.Fatalf("AddCodeReference failed: %v", err)
	}
	if len(task.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(task.Attachments))
	}

	att := task.Attachments[0]
	if !att.IsCodeRef {
		t.Error("expected IsCodeRef true")
	}
	if att.LineNumber != 42 {
		t.Errorf("expected line number 42, got %d", att.LineNumber)
	}
	if att.OriginalPath != "/home/user/main.go" {
		t.Errorf("expected path '/home/user/main.go', got %q", att.OriginalPath)
	}

	// Invalid path
	if err := task.AddCodeReference("/bad||path.go", 10); err == nil {
		t.Error("expected error for path containing delimiter")
	}

	// Invalid line numbers should not create code references. The CLI parser
	// only accepts positive line numbers; keep direct model callers consistent.
	if err := task.AddCodeReference("/home/user/main.go", 0); err == nil {
		t.Error("expected error for zero line number")
	}
	if err := task.AddCodeReference("/home/user/main.go", -1); err == nil {
		t.Error("expected error for negative line number")
	}
	if len(task.Attachments) != 1 {
		t.Errorf("expected failed code references not to mutate attachments, got %d", len(task.Attachments))
	}
}

func TestRemoveAttachment(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	_ = task.AddAttachment("a.txt", false, "/a.txt")
	_ = task.AddAttachment("b.txt", false, "/b.txt")
	_ = task.AddAttachment("c.txt", false, "/c.txt")

	// Remove middle attachment
	removed, ok := task.RemoveAttachment(1)
	if !ok {
		t.Error("expected RemoveAttachment to return true")
	}
	if removed.Filename != "b.txt" {
		t.Errorf("expected removed 'b.txt', got %q", removed.Filename)
	}
	if len(task.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(task.Attachments))
	}

	// Remaining should be a.txt and c.txt
	if task.Attachments[0].Filename != "a.txt" || task.Attachments[1].Filename != "c.txt" {
		t.Error("unexpected attachment order after removal")
	}

	// Out-of-bounds removal
	_, ok = task.RemoveAttachment(-1)
	if ok {
		t.Error("expected false for negative index")
	}
	_, ok = task.RemoveAttachment(99)
	if ok {
		t.Error("expected false for out-of-range index")
	}
}

func TestAttachmentCountAndHas(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	if task.HasAttachments() {
		t.Error("expected no attachments initially")
	}
	if task.AttachmentCount() != 0 {
		t.Errorf("expected count 0, got %d", task.AttachmentCount())
	}

	_ = task.AddAttachment("file.txt", false, "/file.txt")

	if !task.HasAttachments() {
		t.Error("expected HasAttachments true after add")
	}
	if task.AttachmentCount() != 1 {
		t.Errorf("expected count 1, got %d", task.AttachmentCount())
	}
}

func TestGetAttachment(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Nil for empty
	if task.GetAttachment(0) != nil {
		t.Error("expected nil for empty attachments")
	}

	_ = task.AddAttachment("file.txt", false, "/file.txt")

	att := task.GetAttachment(0)
	if att == nil {
		t.Fatal("expected non-nil attachment at index 0")
	}
	if att.Filename != "file.txt" {
		t.Errorf("expected 'file.txt', got %q", att.Filename)
	}

	// Out of bounds
	if task.GetAttachment(-1) != nil {
		t.Error("expected nil for negative index")
	}
	if task.GetAttachment(1) != nil {
		t.Error("expected nil for out-of-range index")
	}
}
