// Package web serves Vahti's management interface and JSON API. It is only
// reachable from the home network and protects every change with a login
// session, a CSRF token, Origin checks and Host-header validation.
package web

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
	"github.com/EliasNiskanen/vahti/internal/netutil"
)

//go:embed static
var staticFS embed.FS

// Config configures the web server.
type Config struct {
	AllowedClients []string
	Hostnames      []string
	TLS            bool
	CertFP         string
	SetupCodePath  string
	Demo           bool
	Version        string
	Upstreams      []string
}

// Server is the HTTP handler.
type Server struct {
	cfg       Config
	eng       *engine.Engine
	db        *intel.DB
	sess      *sessions
	thr       *throttle
	allowed   netutil.PrefixSet
	hostnames map[string]bool
	setupMu   sync.Mutex
	files     map[string][]byte
}

// New creates the web server.
func New(cfg Config, eng *engine.Engine) (*Server, error) {
	allowed, err := netutil.ParsePrefixes(cfg.AllowedClients)
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, eng: eng, db: eng.DB(), sess: newSessions(), thr: newThrottle(),
		allowed: allowed, hostnames: map[string]bool{"localhost": true}, files: map[string][]byte{}}
	for _, h := range cfg.Hostnames {
		s.hostnames[intel.Normalize(h)] = true
	}
	if h, err := os.Hostname(); err == nil {
		s.hostnames[strings.ToLower(h)] = true
	}
	err = fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := staticFS.ReadFile(p)
		s.files["/"+p] = b
		return err
	})
	return s, err
}

func (s *Server) cookieName() string {
	if s.cfg.TLS {
		return "__Host-vahti"
	}
	return "vahti_session"
}

// Handler returns the full handler chain.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.serveIndex)
	mux.HandleFunc("GET /static/{file}", s.serveStatic)

	mux.HandleFunc("GET /api/session", s.apiSession)
	mux.HandleFunc("POST /api/setup", s.apiSetup)
	mux.HandleFunc("POST /api/login", s.apiLogin)
	mux.HandleFunc("POST /api/logout", s.auth(s.apiLogout))
	mux.HandleFunc("POST /api/password", s.auth(s.apiPassword))

	mux.HandleFunc("GET /api/overview", s.auth(s.apiOverview))
	mux.HandleFunc("GET /api/devices", s.auth(s.apiDevices))
	mux.HandleFunc("GET /api/devices/{id}", s.auth(s.apiDevice))
	mux.HandleFunc("PATCH /api/devices/{id}", s.auth(s.apiDeviceUpdate))
	mux.HandleFunc("DELETE /api/devices/{id}", s.auth(s.apiDeviceDelete))
	mux.HandleFunc("GET /api/queue", s.auth(s.apiQueue))
	mux.HandleFunc("POST /api/queue/{id}", s.auth(s.apiQueueResolve))
	mux.HandleFunc("GET /api/rules", s.auth(s.apiRules))
	mux.HandleFunc("POST /api/rules", s.auth(s.apiRuleAdd))
	mux.HandleFunc("DELETE /api/rules/{id}", s.auth(s.apiRuleDelete))
	mux.HandleFunc("GET /api/settings", s.auth(s.apiSettings))
	mux.HandleFunc("PUT /api/settings", s.auth(s.apiSettingsUpdate))
	mux.HandleFunc("POST /api/pause", s.auth(s.apiPause))
	mux.HandleFunc("GET /api/log", s.auth(s.apiLog))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "Tuntematon rajapinta.")
	})
	return s.guard(mux)
}

// RedirectHandler sends plain-HTTP visitors to HTTPS (after the same checks).
func (s *Server) RedirectHandler() http.Handler {
	return s.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := url.URL{Scheme: "https", Host: r.Host, Path: "/"}
		http.Redirect(w, r, u.String(), http.StatusPermanentRedirect)
	}))
}

// guard enforces network, Host and Origin rules and sets security headers.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		if s.cfg.TLS {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}

		ap, err := netip.ParseAddrPort(r.RemoteAddr)
		if err != nil || !s.allowed.Contains(ap.Addr()) {
			http.Error(w, "Vahtia voi käyttää vain kotiverkosta.", http.StatusForbidden)
			return
		}
		if !s.hostAllowed(r.Host) {
			// Blocks DNS rebinding: a malicious site's name pointing at Vahti.
			http.Error(w, "Tuntematon osoite. Avaa Vahti osoitteella https://vahti.lan tai laitteen IP-osoitteella.", http.StatusMisdirectedRequest)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !s.sameOrigin(r) {
				writeErr(w, http.StatusForbidden, "Pyyntö tuli toiselta sivustolta, joten se hylättiin.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) hostAllowed(host string) bool {
	if host == "" || len(host) > 255 {
		return false
	}
	if hh, _, err := net.SplitHostPort(host); err == nil {
		host = hh
	}
	host = strings.Trim(host, "[]")
	if _, err := netip.ParseAddr(host); err == nil {
		return true // an IP literal cannot be rebound
	}
	return s.hostnames[intel.Normalize(host)]
}

func (s *Server) sameOrigin(r *http.Request) bool {
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || u.Host != r.Host {
			return false
		}
		if s.cfg.TLS && u.Scheme != "https" {
			return false
		}
		return true
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
		return true
	}
	return false
}

type authedFunc func(w http.ResponseWriter, r *http.Request, token string)

// auth requires a valid session; state-changing requests also need the CSRF token.
func (s *Server) auth(next authedFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.cookieName())
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "Kirjaudu sisään.")
			return
		}
		ss, ok := s.sess.get(c.Value)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Istunto on vanhentunut. Kirjaudu uudelleen.")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			tok := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(tok), []byte(ss.csrf)) != 1 {
				writeErr(w, http.StatusForbidden, "Suojaustunniste puuttuu. Lataa sivu uudelleen.")
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r, c.Value)
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: s.cookieName(), Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.TLS,
		SameSite: http.SameSiteStrictMode, MaxAge: int(sessionAbsolute / time.Second),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: "", Path: "/", HttpOnly: true,
		Secure: s.cfg.TLS, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

// ---------------------------------------------------------------- static

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(s.files["/static/index.html"])
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	b, ok := s.files["/static/"+name]
	if !ok || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	ct := mime.TypeByExtension(path.Ext(name))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

// ---------------------------------------------------------------- helpers

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeErr(w, http.StatusUnsupportedMediaType, "Pyynnön pitää olla JSON-muodossa.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "Pyyntö oli virheellinen.")
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "Pyyntö oli virheellinen.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("write json", "err", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func engineErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, engine.ErrNotFound):
		writeErr(w, http.StatusNotFound, "Kohdetta ei löytynyt. Se on ehkä jo poistettu.")
	case errors.Is(err, engine.ErrInvalid):
		writeErr(w, http.StatusBadRequest, "Tarkista syötetyt tiedot.")
	default:
		writeErr(w, http.StatusInternalServerError, "Jokin meni vikaan. Yritä uudelleen.")
	}
}

func clientAddr(r *http.Request) netip.Addr {
	ap, _ := netip.ParseAddrPort(r.RemoteAddr)
	return ap.Addr()
}
