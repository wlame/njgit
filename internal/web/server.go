package web

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gitpkg "github.com/wlame/njgit/internal/git"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	repo *gitpkg.Repository
	bind string
	port int
}

func NewServer(repo *gitpkg.Repository, bind string, port int) *Server {
	return &Server{
		repo: repo,
		bind: bind,
		port: port,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	addr := net.JoinHostPort(s.bind, fmt.Sprintf("%d", s.port))
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	log.Printf("njgit dashboard: http://%s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
