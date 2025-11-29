package backend

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wlame/njgit/internal/config"
)

// TestNewGitHubBackend_ValidationErrors tests that NewGitHubBackend validates required fields
func TestNewGitHubBackend_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.GitConfig
		wantErrText string
	}{
		{
			name: "missing owner",
			config: &config.GitConfig{
				Repo:  "test-repo",
				Token: "test-token",
			},
			wantErrText: "github owner is required",
		},
		{
			name: "missing repo",
			config: &config.GitConfig{
				Owner: "test-owner",
				Token: "test-token",
			},
			wantErrText: "github repo is required",
		},
		{
			name: "missing token",
			config: &config.GitConfig{
				Owner: "test-owner",
				Repo:  "test-repo",
			},
			wantErrText: "github token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitHubBackend(tt.config)
			if err == nil {
				t.Errorf("NewGitHubBackend() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("NewGitHubBackend() error = %v, want error containing %v", err.Error(), tt.wantErrText)
			}
		})
	}
}

// TestGitHubBackend_Initialize tests the Initialize method
func TestGitHubBackend_Initialize(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "successful initialization",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "github authentication failed: invalid token",
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "github repository not found: test-owner/test-repo",
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			errMsg:     "github API error: status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify it's a HEAD request
				if r.Method != http.MethodHead {
					t.Errorf("Expected HEAD request, got %s", r.Method)
				}

				// Verify auth header
				auth := r.Header.Get("Authorization")
				if auth != "token test-token" {
					t.Errorf("Expected auth header 'token test-token', got %s", auth)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// Create backend with mock server URL
			cfg := &config.GitConfig{
				Owner:  "test-owner",
				Repo:   "test-repo",
				Token:  "test-token",
				Branch: "main",
			}

			backend, err := NewGitHubBackend(cfg)
			if err != nil {
				t.Fatalf("NewGitHubBackend() unexpected error: %v", err)
			}

			// Replace base URL with mock server
			backend.baseURL = server.URL

			// Test Initialize
			err = backend.Initialize()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Initialize() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Initialize() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Initialize() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestGitHubBackend_ReadFile tests the ReadFile method
func TestGitHubBackend_ReadFile(t *testing.T) {
	testContent := []byte("test file content")
	encodedContent := base64.StdEncoding.EncodeToString(testContent)

	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "successful read",
			statusCode: http.StatusOK,
			response: githubFileResponse{
				Name:    "test.hcl",
				Path:    "default/test.hcl",
				SHA:     "abc123",
				Content: encodedContent,
				Type:    "file",
			},
			wantErr: false,
		},
		{
			name:       "file not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "file not found: default/test.hcl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify auth header
				auth := r.Header.Get("Authorization")
				if auth != "token test-token" {
					t.Errorf("Expected auth header 'token test-token', got %s", auth)
				}

				// Verify branch parameter
				ref := r.URL.Query().Get("ref")
				if ref != "main" {
					t.Errorf("Expected ref=main, got %s", ref)
				}

				w.WriteHeader(tt.statusCode)
				if tt.response != nil {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			// Create backend
			cfg := &config.GitConfig{
				Owner:  "test-owner",
				Repo:   "test-repo",
				Token:  "test-token",
				Branch: "main",
			}

			backend, err := NewGitHubBackend(cfg)
			if err != nil {
				t.Fatalf("NewGitHubBackend() unexpected error: %v", err)
			}

			backend.baseURL = server.URL

			// Test ReadFile
			content, err := backend.ReadFile("default/test.hcl")
			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadFile() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("ReadFile() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ReadFile() unexpected error: %v", err)
					return
				}
				if string(content) != string(testContent) {
					t.Errorf("ReadFile() content = %s, want %s", string(content), string(testContent))
				}
			}
		})
	}
}

// TestGitHubBackend_FileExists tests the FileExists method
func TestGitHubBackend_FileExists(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantExists bool
		wantErr    bool
	}{
		{
			name:       "file exists",
			statusCode: http.StatusOK,
			wantExists: true,
			wantErr:    false,
		},
		{
			name:       "file does not exist",
			statusCode: http.StatusNotFound,
			wantExists: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodHead {
					t.Errorf("Expected HEAD request, got %s", r.Method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// Create backend
			cfg := &config.GitConfig{
				Owner:  "test-owner",
				Repo:   "test-repo",
				Token:  "test-token",
				Branch: "main",
			}

			backend, err := NewGitHubBackend(cfg)
			if err != nil {
				t.Fatalf("NewGitHubBackend() unexpected error: %v", err)
			}

			backend.baseURL = server.URL

			// Test FileExists
			exists, err := backend.FileExists("default/test.hcl")
			if tt.wantErr {
				if err == nil {
					t.Errorf("FileExists() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("FileExists() unexpected error: %v", err)
					return
				}
				if exists != tt.wantExists {
					t.Errorf("FileExists() = %v, want %v", exists, tt.wantExists)
				}
			}
		})
	}
}

// TestGitHubBackend_WriteAndCommit tests the WriteFile and Commit methods
func TestGitHubBackend_WriteAndCommit(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// FileExists check - file doesn't exist
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method == http.MethodPut {
			// Commit request
			var req githubCommitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Verify request
			if req.Message != "Test commit" {
				t.Errorf("Expected message 'Test commit', got %s", req.Message)
			}

			// Decode and verify content
			content, err := base64.StdEncoding.DecodeString(req.Content)
			if err != nil {
				t.Errorf("Failed to decode content: %v", err)
			}
			if string(content) != "test content" {
				t.Errorf("Expected content 'test content', got %s", string(content))
			}

			// Return success response
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(githubCommitResponse{
				Content: struct {
					SHA string `json:"sha"`
				}{SHA: "file-sha-123"},
				Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "commit-sha-456"},
			})
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	// Create backend
	cfg := &config.GitConfig{
		Owner:       "test-owner",
		Repo:        "test-repo",
		Token:       "test-token",
		Branch:      "main",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	backend, err := NewGitHubBackend(cfg)
	if err != nil {
		t.Fatalf("NewGitHubBackend() unexpected error: %v", err)
	}

	backend.baseURL = server.URL

	// Test WriteFile
	err = backend.WriteFile("default/test.hcl", []byte("test content"))
	if err != nil {
		t.Errorf("WriteFile() unexpected error: %v", err)
	}

	// Test Commit
	hash, err := backend.Commit("Test commit")
	if err != nil {
		t.Errorf("Commit() unexpected error: %v", err)
	}

	// GitHub API backend returns empty string for commit hash
	if hash != "" {
		t.Errorf("Commit() hash = %s, want empty string", hash)
	}
}

// TestGitHubBackend_GetName tests the GetName method
func TestGitHubBackend_GetName(t *testing.T) {
	cfg := &config.GitConfig{
		Owner:  "test-owner",
		Repo:   "test-repo",
		Token:  "test-token",
		Branch: "main",
	}

	backend, err := NewGitHubBackend(cfg)
	if err != nil {
		t.Fatalf("NewGitHubBackend() unexpected error: %v", err)
	}

	name := backend.GetName()
	if name != "github-api" {
		t.Errorf("GetName() = %s, want github-api", name)
	}
}

// TestGitHubBackend_PushAndClose tests Push and Close methods (should be no-ops)
func TestGitHubBackend_PushAndClose(t *testing.T) {
	cfg := &config.GitConfig{
		Owner:  "test-owner",
		Repo:   "test-repo",
		Token:  "test-token",
		Branch: "main",
	}

	backend, err := NewGitHubBackend(cfg)
	if err != nil {
		t.Fatalf("NewGitHubBackend() unexpected error: %v", err)
	}

	// Push should be a no-op
	if err := backend.Push(); err != nil {
		t.Errorf("Push() unexpected error: %v", err)
	}

	// Close should be a no-op
	if err := backend.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
}
