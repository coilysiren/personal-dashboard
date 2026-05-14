package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/coilysiren/personal-dashboard/internal/session"
	"github.com/coilysiren/personal-dashboard/internal/voice"
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
	voice     *voice.Client
}

// PageData is the template payload every route renders against.
// Revealed mirrors the per-session reveal flag for the current route
// so the template can stamp .revealed on the route root.
type PageData struct {
	Route    string
	Revealed bool
}

// Config is what New takes. Zero values are usable: missing voice creds
// leave the voice client disabled, no crash.
type Config struct {
	ElevenLabsAPIKey  string
	ElevenLabsVoiceID string
}

func New(logger *slog.Logger, cfg Config) *Server {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html.tmpl"))
	v := voice.New(cfg.ElevenLabsAPIKey, cfg.ElevenLabsVoiceID)
	if !v.Enabled() {
		logger.Warn("voice disabled: missing ELEVENLABS_API_KEY or ELEVENLABS_VOICE_ID")
	}
	return &Server{
		logger:    logger,
		templates: tmpl,
		sessions:  session.NewStore(),
		voice:     v,
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
	mux.HandleFunc("POST /api/voice/say", s.handleVoiceSay)

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

// handleVoiceSay synthesizes audio for the "text" form value and streams
// mp3 back to the client. Used by the PWA to play event announcements.
func (s *Server) handleVoiceSay(w http.ResponseWriter, r *http.Request) {
	if !s.voice.Enabled() {
		http.Error(w, "voice disabled", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "missing text", http.StatusBadRequest)
		return
	}
	audio, err := s.voice.Synthesize(r.Context(), text)
	if err != nil {
		s.logger.Error("voice synthesize failed", "err", err)
		http.Error(w, "synthesize failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(audio)
}

func redirectTarget(r *http.Request) string {
	next := r.FormValue("next")
	if next == "" || next[0] != '/' {
		return "/"
	}
	return next
}
