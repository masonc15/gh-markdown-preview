package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirHandler_Index(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644)
	os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# guide"), 0644)

	param := &Param{reload: false}
	h := dirHandler(dir, param, http.FileServer(http.Dir(dir)))
	ts := httptest.NewServer(h)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.StatusCode != 200 {
		t.Errorf("index status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("index content-type = %s, want text/html", ct)
	}
}

func TestDirHandler_IndexContainsSidebar(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644)
	os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# guide"), 0644)

	param := &Param{reload: false}
	h := dirHandler(dir, param, http.FileServer(http.Dir(dir)))
	ts := httptest.NewServer(h)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	html := string(body)

	// Should contain sidebar elements
	if !strings.Contains(html, "sidebar") {
		t.Error("index should contain sidebar class")
	}
	if !strings.Contains(html, "README.md") {
		t.Error("index should list README.md")
	}
	if !strings.Contains(html, "guide.md") {
		t.Error("index should list guide.md")
	}
	// Should contain the markdown-body target for AJAX loading
	if !strings.Contains(html, "markdown-body") {
		t.Error("index should contain markdown-body element")
	}
}

func TestDirHandler_StaticFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "images"), 0755)
	os.WriteFile(filepath.Join(dir, "images", "test.txt"), []byte("hello"), 0644)

	param := &Param{reload: false}
	h := dirHandler(dir, param, http.FileServer(http.Dir(dir)))
	ts := httptest.NewServer(h)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/images/test.txt")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.StatusCode != 200 {
		t.Errorf("static file status = %d, want 200", res.StatusCode)
	}
}

func TestDirMdHandler_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("# test"), 0644)

	param := &Param{reload: false}
	h := dirMdHandler(dir, param)
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Path traversal should be rejected
	res, err := http.Get(ts.URL + "?path=../../../etc/passwd")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("traversal status = %d, want 403", res.StatusCode)
	}

	// Missing path param should be rejected
	res, err = http.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("missing path status = %d, want 400", res.StatusCode)
	}
}
