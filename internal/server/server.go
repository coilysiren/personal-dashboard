package server

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

//go:embed templates/*.html.tmpl
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	logger    *slog.Logger
	templates *template.Template
}

func New(logger *slog.Logger) *Server {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html.tmpl"))
	return &Server{logger: logger, templates: tmpl}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /{$}", s.handleIndex)

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html.tmpl", nil); err != nil {
		s.logger.Error("render index failed", "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
