package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitpkg "github.com/wlame/njgit/internal/git"
)

func setupTestRepo(t *testing.T) (*gitpkg.Repository, string) {
	t.Helper()

	dir := t.TempDir()

	// Init a bare git repo
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	// Create a file and commit
	jobDir := filepath.Join(dir, "global", "default")
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	content := []byte(`job "web-app" {
  datacenters = ["dc1"]
  type        = "service"
}
`)
	if err := os.WriteFile(filepath.Join(jobDir, "web-app.hcl"), content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if _, err := wt.Add("global/default/web-app.hcl"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = wt.Commit("Add web-app", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Second commit with updated content
	content2 := []byte(`job "web-app" {
  datacenters = ["dc1", "dc2"]
  type        = "service"
}
`)
	if err := os.WriteFile(filepath.Join(jobDir, "web-app.hcl"), content2, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if _, err := wt.Add("global/default/web-app.hcl"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = wt.Commit("Update web-app", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Open with our wrapper
	gitRepo, err := gitpkg.NewLocalRepository(dir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	return gitRepo, dir
}

func TestHandleListCommits(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	req := httptest.NewRequest("GET", "/api/commits", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var commits []commitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &commits); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// Most recent commit first
	if commits[0].Message != "Update web-app" {
		t.Errorf("expected first commit message 'Update web-app', got %q", commits[0].Message)
	}
	if commits[1].Message != "Add web-app" {
		t.Errorf("expected second commit message 'Add web-app', got %q", commits[1].Message)
	}

	// Check files
	if len(commits[0].Files) != 1 || commits[0].Files[0] != "global/default/web-app.hcl" {
		t.Errorf("expected files [global/default/web-app.hcl], got %v", commits[0].Files)
	}

	// Check hash fields
	if len(commits[0].Hash) != 8 {
		t.Errorf("expected 8-char short hash, got %q", commits[0].Hash)
	}
	if len(commits[0].FullHash) != 40 {
		t.Errorf("expected 40-char full hash, got %q", commits[0].FullHash)
	}
}

func TestHandleListCommits_WithLimit(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	req := httptest.NewRequest("GET", "/api/commits?limit=1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var commits []commitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &commits); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(commits) != 1 {
		t.Errorf("expected 1 commit with limit=1, got %d", len(commits))
	}
}

func TestHandleListCommits_EmptyResult(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// Filter by non-existent path
	req := httptest.NewRequest("GET", "/api/commits?path=nonexistent/file.hcl", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var commits []commitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &commits); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(commits) != 0 {
		t.Errorf("expected 0 commits for non-existent path, got %d", len(commits))
	}
}

func TestHandleGetFile(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// First get commits to find a hash
	req := httptest.NewRequest("GET", "/api/commits", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var commits []commitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &commits); err != nil {
		t.Fatalf("failed to decode commits: %v", err)
	}

	// Get the latest commit's file
	hash := commits[0].FullHash
	req = httptest.NewRequest("GET", "/api/file/"+hash+"/global/default/web-app.hcl", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty file content")
	}

	// Latest commit should have dc2
	expected := `job "web-app" {
  datacenters = ["dc1", "dc2"]
  type        = "service"
}
`
	if body != expected {
		t.Errorf("file content mismatch.\ngot:  %q\nwant: %q", body, expected)
	}

	// Check content type
	ct := w.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("expected Content-Type text/plain, got %q", ct)
	}
}

func TestHandleGetFile_NotFound(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// Get commits to find a valid hash
	req := httptest.NewRequest("GET", "/api/commits", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var commits []commitResponse
	json.Unmarshal(w.Body.Bytes(), &commits)
	hash := commits[0].FullHash

	// Request non-existent file
	req = httptest.NewRequest("GET", "/api/file/"+hash+"/nonexistent.hcl", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetFile_InvalidHash(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	req := httptest.NewRequest("GET", "/api/file/0000000000000000000000000000000000000000/global/default/web-app.hcl", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetFile_OlderCommit(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// Get commits
	req := httptest.NewRequest("GET", "/api/commits", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var commits []commitResponse
	json.Unmarshal(w.Body.Bytes(), &commits)

	// Get file from the older (initial) commit
	oldHash := commits[1].FullHash
	req = httptest.NewRequest("GET", "/api/file/"+oldHash+"/global/default/web-app.hcl", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	// Old commit should only have dc1
	expected := `job "web-app" {
  datacenters = ["dc1"]
  type        = "service"
}
`
	if body != expected {
		t.Errorf("old commit file content mismatch.\ngot:  %q\nwant: %q", body, expected)
	}
}

func TestStaticFileServing(t *testing.T) {
	repo, _ := setupTestRepo(t)
	srv := NewServer(repo, "127.0.0.1", 0)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty HTML response")
	}

	// Should contain our dashboard title
	if !containsString(body, "njgit") {
		t.Error("HTML should contain 'njgit'")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
