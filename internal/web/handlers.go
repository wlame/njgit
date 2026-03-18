package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
)

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/commits", s.handleListCommits)
	mux.HandleFunc("GET /api/file/{hash}/{path...}", s.handleGetFile)

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /", http.FileServer(http.FS(staticSub)))
}

type commitResponse struct {
	Hash     string   `json:"hash"`
	FullHash string   `json:"full_hash"`
	Message  string   `json:"message"`
	Author   string   `json:"author"`
	Email    string   `json:"email"`
	Date     string   `json:"date"`
	Files    []string `json:"files"`
}

func (s *Server) handleListCommits(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	limit := 200
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	commits, err := s.repo.GetHistory(path, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]commitResponse, 0, len(commits))
	for _, c := range commits {
		files := c.Files
		if files == nil {
			files = []string{}
		}
		resp = append(resp, commitResponse{
			Hash:     c.Hash,
			FullHash: c.FullHash,
			Message:  c.Message,
			Author:   c.Author,
			Email:    c.Email,
			Date:     c.Date.Format("2006-01-02 15:04:05"),
			Files:    files,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	path := r.PathValue("path")

	content, err := s.repo.GetFileAtCommit(hash, path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(content)
}
