package tempfile

import (
	"os"
	"strings"
	"testing"

	"resparse/internal/apperr"
)

func TestDownloadToFile(t *testing.T) {
	content := "0123456789"
	dl, err := DownloadToFile(strings.NewReader(content), 100, "metaparse-test-", ".mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer RemoveDir(dl.Dir)

	if dl.Bytes != 10 {
		t.Errorf("bytes = %d, want 10", dl.Bytes)
	}
	if !strings.HasSuffix(dl.FilePath, "input.mp4") {
		t.Errorf("filePath = %q", dl.FilePath)
	}
	got, err := os.ReadFile(dl.FilePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}

	RemoveDir(dl.Dir)
	if _, err := os.Stat(dl.Dir); !os.IsNotExist(err) {
		t.Errorf("dir should be removed")
	}
}

func TestDownloadToFileTooLarge(t *testing.T) {
	dl, err := DownloadToFile(strings.NewReader("0123456789"), 5, "metaparse-test-", "")
	if err == nil {
		RemoveDir(dl.Dir)
		t.Fatal("expected file_too_large error")
	}
	ae, ok := apperr.As(err)
	if !ok || ae.Code != "file_too_large" || ae.Status != 413 {
		t.Fatalf("unexpected error: %v", err)
	}
}
