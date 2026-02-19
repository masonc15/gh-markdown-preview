package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindMarkdownFiles(t *testing.T) {
	files, err := findMarkdownFiles("../testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// testdata has: gfm-alerts.md, gfm-checkboxes.md, markdown-demo.md
	// README has no .md extension so should be excluded
	if len(files) != 3 {
		t.Errorf("got %d files, want 3: %v", len(files), files)
	}

	// All files should be relative paths
	for _, f := range files {
		if filepath.IsAbs(f) {
			t.Errorf("expected relative path, got absolute: %s", f)
		}
	}

	// Should be sorted alphabetically (no README.md to sort first)
	for i := 1; i < len(files); i++ {
		if files[i] < files[i-1] {
			t.Errorf("files not sorted: %v", files)
			break
		}
	}
}

func TestFindMarkdownFilesSkipsHidden(t *testing.T) {
	// Create a temp dir with a hidden subdir
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0755)
	os.WriteFile(filepath.Join(dir, "visible.md"), []byte("# test"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden", "secret.md"), []byte("# hidden"), 0644)

	files, err := findMarkdownFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("got %d files, want 1 (should skip hidden): %v", len(files), files)
	}
	if len(files) > 0 && files[0] != "visible.md" {
		t.Errorf("got %s, want visible.md", files[0])
	}
}

func TestFindMarkdownFilesReadmeFirst(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "zebra.md"), []byte("# z"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644)
	os.WriteFile(filepath.Join(dir, "alpha.md"), []byte("# a"), 0644)

	files, err := findMarkdownFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("got %d files, want 3", len(files))
	}
	if files[0] != "README.md" {
		t.Errorf("README.md should be first, got %s", files[0])
	}
	if files[1] != "alpha.md" {
		t.Errorf("alpha.md should be second, got %s", files[1])
	}
}

func TestSafePath(t *testing.T) {
	baseDir := "/home/user/docs"

	// Valid paths
	validCases := []struct {
		relPath  string
		expected string
	}{
		{"README.md", "/home/user/docs/README.md"},
		{"subdir/file.md", "/home/user/docs/subdir/file.md"},
		{"./README.md", "/home/user/docs/README.md"},
	}
	for _, tc := range validCases {
		result, err := safePath(baseDir, tc.relPath)
		if err != nil {
			t.Errorf("safePath(%q, %q) unexpected error: %v", baseDir, tc.relPath, err)
		}
		if result != tc.expected {
			t.Errorf("safePath(%q, %q) = %q, want %q", baseDir, tc.relPath, result, tc.expected)
		}
	}

	// Invalid paths (traversal)
	invalidCases := []string{
		"../../../etc/passwd",
		"../sibling/file.md",
		"/etc/passwd",
	}
	for _, relPath := range invalidCases {
		_, err := safePath(baseDir, relPath)
		if err == nil {
			t.Errorf("safePath(%q, %q) should have failed", baseDir, relPath)
		}
	}
}
