package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/coilysiren/personal-dashboard/internal/session"
)

//go:embed templates/*.html.tmpl
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type ctxKey int

const sessionIDKey ctxKey = 1

type Server struct {
	logger    *slog.Logger
	templates *template.Template
	sessions  *session.Store
}

// PageData is the template payload every route renders against.
// Revealed mirrors the per-session reveal flag for the current route
// so the template can stamp .revealed on the route root.
type PageData struct {
	Route    string
	Revealed bool
}

func New(logger *slog.Logger) *Server {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html.tmpl"))
	return &Server{
		logger:    logger,
		templates: tmpl,
		sessions:  session.NewStore(),
	}
}

// Sessions exposes the session store for the caller (used by main to wire
// the background pruner).
func (s *Server) Sessions() *session.Store { return s.sessions }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /reveal", s.handleReveal)
	mux.HandleFunc("POST /hide", s.handleHide)

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	return s.withSession(mux)
}

// withSession assigns a session cookie if missing and stashes the id in
// the request context. The cookie is HttpOnly, SameSite=Strict. Secure is
// set when the request looks TLS-terminated; on the loopback dev bind we
// drop Secure so cookies are usable in plain HTTP.
func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id string
		if c, err := r.Cookie(session.CookieName); err == nil && c.Value != "" {
			id = c.Value
		} else {
			id = session.NewID()
			http.SetCookie(w, &http.Cookie{
				Name:     session.CookieName,
				Value:    id,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				Secure:   r.TLS != nil,
			})
		}
		s.sessions.Touch(id)
		ctx := context.WithValue(r.Context(), sessionIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func sessionID(r *http.Request) string {
	v, _ := r.Context().Value(sessionIDKey).(string)
	return v
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	route := "/"
	data := PageData{
		Route:    route,
		Revealed: s.sessions.IsRevealed(sessionID(r), route),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html.tmpl", data); err != nil {
		s.logger.Error("render index failed", "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// handleReveal flips a route revealed for the current session. The
// route to reveal is the form value "route"; redirect target is "next"
// (defaults to "/"). Form posts so a JS-free fallback works.
func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	route := r.FormValue("route")
	if route == "" {
		http.Error(w, "missing route", http.StatusBadRequest)
		return
	}
	s.sessions.Reveal(sessionID(r), route)
	http.Redirect(w, r, redirectTarget(r), http.StatusSeeOther)
}

func (s *Server) handleHide(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	route := r.FormValue("route")
	if route == "" {
		http.Error(w, "missing route", http.StatusBadRequest)
		return
	}
	s.sessions.Hide(sessionID(r), route)
	http.Redirect(w, r, redirectTarget(r), http.StatusSeeOther)
}

func redirectTarget(r *http.Request) string {
	next := r.FormValue("next")
	if next == "" || next[0] != '/' {
		return "/"
	}
	return next
}
