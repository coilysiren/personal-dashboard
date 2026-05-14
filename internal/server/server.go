package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coilysiren/personal-dashboard/internal/session"
	"github.com/coilysiren/personal-dashboard/internal/sources/bluesky"
	"github.com/coilysiren/personal-dashboard/internal/sources/coilyaudit"
	"github.com/coilysiren/personal-dashboard/internal/sources/o2r"
	"github.com/coilysiren/personal-dashboard/internal/sources/reddit"
	"github.com/coilysiren/personal-dashboard/internal/sources/steam"
	"github.com/coilysiren/personal-dashboard/internal/sources/vaultinbox"
	"github.com/coilysiren/personal-dashboard/internal/state"
	"github.com/coilysiren/personal-dashboard/internal/voice"
)

//go:embed templates/*.html.tmpl templates/panels/*.html.tmpl
var templateFS embed.FS

// pageTemplates is the path to each page-specific template, keyed by
// page name. Each page gets its own template namespace at construction
// time so {{define "main"}} blocks do not collide across pages.
var pageTemplates = map[string]string{
	"index":         "templates/index.html.tmpl",
	"allowlist-gap": "templates/panels/allowlist-gap.html.tmpl",
	"daily-inbox":   "templates/panels/daily-inbox.html.tmpl",
	"steam":         "templates/panels/steam.html.tmpl",
	"luca-o2r":      "templates/panels/luca-o2r.html.tmpl",
	"social":        "templates/panels/social.html.tmpl",
}

//go:embed static
var staticFS embed.FS

type ctxKey int

const sessionIDKey ctxKey = 1

type Server struct {
	logger     *slog.Logger
	pages      map[string]*template.Template
	sessions   *session.Store
	voice      *voice.Client
	coilyAudit *coilyaudit.Source
	vaultInbox *vaultinbox.Source
	inboxRead  *state.InboxRead
	steam      *steam.Client
	o2r        *o2r.Source
	bluesky    *bluesky.Client
	reddit     *reddit.Client
}

// PageData is the template payload every route renders against.
// Revealed mirrors the per-session reveal flag for the current route
// so the template can stamp .revealed on the route root.
type PageData struct {
	Route    string
	Revealed bool
}

// Config is what New takes. Zero values are usable: missing voice creds
// leave the voice client disabled, no crash. Empty CoilyAuditDir falls
// back to ~/.coily/audit.
type Config struct {
	ElevenLabsAPIKey  string
	ElevenLabsVoiceID string
	CoilyAuditDir     string
	VaultInboxDir     string
	StateDir          string
	SteamAPIKey       string
	SteamUserID       string
	GrafanaURL        string
	PhoenixURL        string
	VictoriaMetricsURL string
	BlueskyHandle     string
	RedditInboxRSS    string
}

func New(logger *slog.Logger, cfg Config) *Server {
	// Parse base.html.tmpl once, clone per page, then layer the page-
	// specific template on top. This gives each page its own
	// {{define "main"}} namespace so they cannot stomp on each other.
	base := template.Must(template.ParseFS(templateFS, "templates/base.html.tmpl"))
	pages := make(map[string]*template.Template, len(pageTemplates))
	for name, path := range pageTemplates {
		t, err := base.Clone()
		if err != nil {
			panic("server: clone base template: " + err.Error())
		}
		pages[name] = template.Must(t.ParseFS(templateFS, path))
	}
	v := voice.New(cfg.ElevenLabsAPIKey, cfg.ElevenLabsVoiceID)
	if !v.Enabled() {
		logger.Warn("voice disabled: missing ELEVENLABS_API_KEY or ELEVENLABS_VOICE_ID")
	}
	var audit *coilyaudit.Source
	if cfg.CoilyAuditDir != "" {
		audit = coilyaudit.NewWithDir(cfg.CoilyAuditDir)
	} else {
		audit = coilyaudit.New()
	}
	inbox := vaultinbox.New(cfg.VaultInboxDir)

	stateDir := cfg.StateDir
	if stateDir == "" {
		home, _ := os.UserHomeDir()
		stateDir = home + "/.personal-dashboard/state"
	}
	inboxRead, err := state.LoadInboxRead(stateDir + "/inbox-read.json")
	if err != nil {
		logger.Warn("inbox-read state load failed", "err", err)
		inboxRead, _ = state.LoadInboxRead("")
	}
	st := steam.New(cfg.SteamAPIKey, cfg.SteamUserID)
	if !st.Enabled() {
		logger.Warn("steam disabled: missing STEAM_API_KEY or STEAM_USER_ID")
	}
	o2rSrc := o2r.New(cfg.GrafanaURL, cfg.PhoenixURL, cfg.VictoriaMetricsURL)
	bs := bluesky.New(cfg.BlueskyHandle)
	if !bs.Enabled() {
		logger.Warn("bluesky disabled: missing BLUESKY_HANDLE")
	}
	rd := reddit.New(cfg.RedditInboxRSS)
	if !rd.Enabled() {
		logger.Warn("reddit disabled: missing REDDIT_INBOX_RSS")
	}
	return &Server{
		logger:     logger,
		pages:      pages,
		sessions:   session.NewStore(),
		voice:      v,
		coilyAudit: audit,
		vaultInbox: inbox,
		inboxRead:  inboxRead,
		steam:      st,
		o2r:        o2rSrc,
		bluesky:    bs,
		reddit:     rd,
	}
}

// render runs the named page template, isolated from other pages'
// {{define}} blocks. Each page is parsed with its own clone of the
// base layout at construction time.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	t, ok := s.pages[name]
	if !ok {
		s.logger.Error("unknown page template", "name", name)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		s.logger.Error("render failed", "page", name, "err", err)
		// Headers already sent; just stop writing.
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
	mux.HandleFunc("GET /panels/allowlist-gap", s.handleAllowlistGap)
	mux.HandleFunc("GET /panels/daily-inbox", s.handleDailyInbox)
	mux.HandleFunc("POST /panels/daily-inbox/read", s.handleDailyInboxRead)
	mux.HandleFunc("POST /panels/daily-inbox/unread", s.handleDailyInboxUnread)
	mux.HandleFunc("GET /panels/steam", s.handleSteam)
	mux.HandleFunc("GET /panels/luca-o2r", s.handleLucaO2R)
	mux.HandleFunc("GET /panels/social", s.handleSocial)

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
	s.render(w, "index", data)
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

// allowlistGapRow is the per-row payload for the allowlist-gap panel.
type allowlistGapRow struct {
	Verb    string
	Argv    string
	Error   string
	RelTime string
}

type allowlistGapData struct {
	PageData
	Panel struct {
		Rows []allowlistGapRow
	}
}

func (s *Server) handleAllowlistGap(w http.ResponseWriter, r *http.Request) {
	const route = "/panels/allowlist-gap"
	rows, err := s.coilyAudit.Denials(time.Now().Add(-7*24*time.Hour), 50)
	if err != nil {
		s.logger.Error("read coily denials failed", "err", err)
		http.Error(w, "audit read failed", http.StatusInternalServerError)
		return
	}
	data := allowlistGapData{
		PageData: PageData{
			Route:    route,
			Revealed: s.sessions.IsRevealed(sessionID(r), route),
		},
	}
	for _, row := range rows {
		data.Panel.Rows = append(data.Panel.Rows, allowlistGapRow{
			Verb:    row.Verb,
			Argv:    strings.Join(row.Argv, " "),
			Error:   firstLine(row.Error),
			RelTime: humanizeRel(row.Time),
		})
	}
	s.render(w, "allowlist-gap", data)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func humanizeRel(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmtInt(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return fmtInt(int(d.Hours())) + "h ago"
	default:
		return fmtInt(int(d.Hours()/24)) + "d ago"
	}
}

func fmtInt(i int) string {
	if i < 0 {
		return "0"
	}
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// dailyInboxEntry is the per-row payload for the daily-inbox panel.
type dailyInboxEntry struct {
	ID        string
	Date      string
	Category  string
	Title     string
	Synthesis string
	Read      bool
}

type dailyInboxData struct {
	PageData
	Panel struct {
		Entries []dailyInboxEntry
	}
}

func (s *Server) handleDailyInbox(w http.ResponseWriter, r *http.Request) {
	const route = "/panels/daily-inbox"
	files, err := s.vaultInbox.List(60)
	if err != nil {
		s.logger.Error("read vault inbox failed", "err", err)
		http.Error(w, "vault read failed", http.StatusInternalServerError)
		return
	}
	data := dailyInboxData{
		PageData: PageData{
			Route:    route,
			Revealed: s.sessions.IsRevealed(sessionID(r), route),
		},
	}
	for _, df := range files {
		data.Panel.Entries = append(data.Panel.Entries, dailyInboxEntry{
			ID:        df.ID,
			Date:      df.Date,
			Category:  df.Category,
			Title:     df.Title,
			Synthesis: df.Synthesis,
			Read:      s.inboxRead.IsRead(df.ID),
		})
	}
	s.render(w, "daily-inbox", data)
}

func (s *Server) handleDailyInboxRead(w http.ResponseWriter, r *http.Request) {
	s.handleDailyInboxFlip(w, r, true)
}

func (s *Server) handleDailyInboxUnread(w http.ResponseWriter, r *http.Request) {
	s.handleDailyInboxFlip(w, r, false)
}

func (s *Server) handleDailyInboxFlip(w http.ResponseWriter, r *http.Request, read bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	var err error
	if read {
		err = s.inboxRead.MarkRead(id)
	} else {
		err = s.inboxRead.MarkUnread(id)
	}
	if err != nil {
		s.logger.Error("inbox read flip failed", "id", id, "err", err)
	}
	http.Redirect(w, r, redirectTarget(r), http.StatusSeeOther)
}

// steamData is the payload for the Steam panel.
type steamData struct {
	PageData
	Panel struct {
		Enabled bool
		Err     string
		Games   []steam.RecentlyPlayed
	}
}

func (s *Server) handleSteam(w http.ResponseWriter, r *http.Request) {
	const route = "/panels/steam"
	data := steamData{
		PageData: PageData{
			Route:    route,
			Revealed: s.sessions.IsRevealed(sessionID(r), route),
		},
	}
	data.Panel.Enabled = s.steam.Enabled()
	if data.Panel.Enabled {
		games, err := s.steam.Recent(r.Context(), 5)
		if err != nil {
			data.Panel.Err = err.Error()
		} else {
			data.Panel.Games = games
		}
	}
	s.render(w, "steam", data)
}

type lucaO2RData struct {
	PageData
	Panel struct {
		GrafanaURL string
		PhoenixURL string
		Digest     o2r.Digest
	}
}

func (s *Server) handleLucaO2R(w http.ResponseWriter, r *http.Request) {
	const route = "/panels/luca-o2r"
	data := lucaO2RData{
		PageData: PageData{
			Route:    route,
			Revealed: s.sessions.IsRevealed(sessionID(r), route),
		},
	}
	data.Panel.GrafanaURL = s.o2r.GrafanaURL
	data.Panel.PhoenixURL = s.o2r.PhoenixURL
	data.Panel.Digest = s.o2r.FetchDigest(r.Context())
	s.render(w, "luca-o2r", data)
}

type socialData struct {
	PageData
	Panel struct {
		Bluesky struct {
			Enabled bool
			Err     string
			Posts   []bluesky.Post
		}
		Reddit struct {
			Enabled bool
			Err     string
			Items   []reddit.Item
		}
	}
}

func (s *Server) handleSocial(w http.ResponseWriter, r *http.Request) {
	const route = "/panels/social"
	data := socialData{
		PageData: PageData{
			Route:    route,
			Revealed: s.sessions.IsRevealed(sessionID(r), route),
		},
	}
	data.Panel.Bluesky.Enabled = s.bluesky.Enabled()
	if data.Panel.Bluesky.Enabled {
		posts, err := s.bluesky.Recent(r.Context(), 5)
		if err != nil {
			data.Panel.Bluesky.Err = err.Error()
		} else {
			data.Panel.Bluesky.Posts = posts
		}
	}
	data.Panel.Reddit.Enabled = s.reddit.Enabled()
	if data.Panel.Reddit.Enabled {
		items, err := s.reddit.Unread(r.Context())
		if err != nil {
			data.Panel.Reddit.Err = err.Error()
		} else {
			data.Panel.Reddit.Items = items
		}
	}
	s.render(w, "social", data)
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
