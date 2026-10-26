package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
	"github.com/EliasNiskanen/vahti/internal/netutil"
)

const testCode = "ABCD-EFGH-IJKL-MNOP"

// harness wraps the web server and its engine for tests.
type harness struct {
	t   *testing.T
	h   http.Handler
	eng *engine.Engine
}

// newHarness returns a web server that has not been set up yet, with the test setup code.
func newHarness(t *testing.T) *harness {
	t.Helper()
	eng := engine.New(nil, intel.New(), 100)
	eng.SetAdmin(engine.Admin{SetupCodeHash: HashSetupCode(testCode)})
	s, err := New(Config{AllowedClients: netutil.DefaultLocalPrefixes, Hostnames: []string{"vahti.lan"}, TLS: true}, eng)
	if err != nil {
		t.Fatal(err)
	}
	return &harness{t: t, h: s.Handler(), eng: eng}
}

type reqOpt func(*http.Request)

// Request options for do: the client address, the Host header, any header,
// the session cookie and the CSRF token.
func from(ip string) reqOpt  { return func(r *http.Request) { r.RemoteAddr = ip + ":40000" } }
func host(h string) reqOpt   { return func(r *http.Request) { r.Host = h } }
func hdr(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }
func cookie(c string) reqOpt { return func(r *http.Request) { r.Header.Set("Cookie", c) } }
func csrf(tok string) reqOpt { return hdr("X-CSRF-Token", tok) }

// do sends a request to the web server, by default from a home network
// address to https://vahti.lan, and returns the recorded response.
func (h *harness) do(method, path, body string, opts ...reqOpt) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "https://vahti.lan"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, "https://vahti.lan"+path, nil)
	}
	r.RemoteAddr = "192.168.1.20:40000"
	r.Host = "vahti.lan"
	for _, o := range opts {
		o(r)
	}
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	return w
}

// login completes setup and returns the session cookie and CSRF token.
func (h *harness) login() (string, string) {
	h.t.Helper()
	w := h.do("POST", "/api/setup", `{"code":"abcd efgh ijkl mnop","password":"correct horse battery"}`)
	if w.Code != 200 {
		h.t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	var out struct{ CSRF string }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	c := w.Result().Cookies()[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Name != "__Host-vahti" {
		h.t.Fatalf("weak session cookie: %+v", c)
	}
	return c.Name + "=" + c.Value, out.CSRF
}

// Requests from the internet or with a foreign Host name (DNS rebinding) are refused.
func TestNetworkAndHostChecks(t *testing.T) {
	h := newHarness(t)
	if w := h.do("GET", "/", "", from("203.0.113.5")); w.Code != http.StatusForbidden {
		t.Fatalf("internet client: %d", w.Code)
	}
	if w := h.do("GET", "/", "", host("evil.example")); w.Code != http.StatusMisdirectedRequest {
		t.Fatalf("rebinding host: %d", w.Code)
	}
	for _, ok := range []string{"vahti.lan", "192.168.1.2", "192.168.1.2:443", "[fd00::2]:443", "VAHTI.LAN"} {
		if w := h.do("GET", "/", "", host(ok)); w.Code != 200 {
			t.Errorf("host %q: %d", ok, w.Code)
		}
	}
}

// Every page gets the security headers (CSP, frame blocking, HSTS and others).
func TestSecurityHeaders(t *testing.T) {
	h := newHarness(t)
	w := h.do("GET", "/", "")
	for _, k := range []string{"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy", "Strict-Transport-Security"} {
		if w.Header().Get(k) == "" {
			t.Errorf("missing %s", k)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self'") {
		t.Error("CSP must forbid inline and external scripts")
	}
}

// The first setup needs the right code and a strong password, works only once
// and discards the setup code afterwards.
func TestSetupFlow(t *testing.T) {
	h := newHarness(t)
	if w := h.do("GET", "/api/devices", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated API: %d", w.Code)
	}
	if w := h.do("POST", "/api/setup", `{"code":"WRONG-CODE","password":"correct horse battery"}`); w.Code != http.StatusForbidden {
		t.Fatalf("wrong code: %d", w.Code)
	}
	if w := h.do("POST", "/api/setup", `{"code":"`+testCode+`","password":"short"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("weak password: %d", w.Code)
	}
	c, tok := h.login()
	if w := h.do("POST", "/api/setup", `{"code":"`+testCode+`","password":"another long password"}`); w.Code != http.StatusConflict {
		t.Fatalf("second setup must be refused: %d", w.Code)
	}
	if w := h.do("GET", "/api/overview", "", cookie(c)); w.Code != 200 {
		t.Fatalf("overview: %d", w.Code)
	}
	if h.eng.Admin().SetupCodeHash != "" {
		t.Fatal("setup code must be discarded after use")
	}
	_ = tok
}

// Changes need a valid CSRF token and must come from the Vahti page itself.
func TestCSRFAndOrigin(t *testing.T) {
	h := newHarness(t)
	c, tok := h.login()
	body := `{"device":"*","domain":"tracker.example","action":"block","hours":0}`
	if w := h.do("POST", "/api/rules", body, cookie(c)); w.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF token: %d", w.Code)
	}
	if w := h.do("POST", "/api/rules", body, cookie(c), csrf("forged")); w.Code != http.StatusForbidden {
		t.Fatalf("wrong CSRF token: %d", w.Code)
	}
	if w := h.do("POST", "/api/rules", body, cookie(c), csrf(tok), hdr("Origin", "https://evil.example")); w.Code != http.StatusForbidden {
		t.Fatalf("cross-site origin: %d", w.Code)
	}
	if w := h.do("POST", "/api/rules", body, cookie(c), csrf(tok), hdr("Sec-Fetch-Site", "cross-site")); w.Code != http.StatusForbidden {
		t.Fatalf("cross-site fetch: %d", w.Code)
	}
	if w := h.do("POST", "/api/rules", body, cookie(c), csrf(tok), hdr("Origin", "https://vahti.lan")); w.Code != 200 {
		t.Fatalf("valid request: %d %s", w.Code, w.Body)
	}
}

// Request bodies with unknown fields or extra data are rejected.
func TestStrictJSON(t *testing.T) {
	h := newHarness(t)
	c, tok := h.login()
	r := h.do("POST", "/api/pause", `{"minutes":5,"extra":true}`, cookie(c), csrf(tok))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d", r.Code)
	}
	r = h.do("POST", "/api/pause", `{"minutes":5}{"minutes":6}`, cookie(c), csrf(tok))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("trailing data: %d", r.Code)
	}
	r = h.do("POST", "/api/pause", `{"minutes":5}`, cookie(c), csrf(tok), hdr("Content-Type", "text/plain"))
	if r.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("form content type: %d", r.Code)
	}
	big := `{"device":"*","domain":"` + strings.Repeat("a", 70000) + `","action":"block"}`
	if r = h.do("POST", "/api/rules", big, cookie(c), csrf(tok)); r.Code != http.StatusBadRequest {
		t.Fatalf("oversized body: %d", r.Code)
	}
}

// After several wrong passwords the login is temporarily blocked.
func TestLoginThrottle(t *testing.T) {
	h := newHarness(t)
	h.login()
	codes := []int{}
	for i := 0; i < 7; i++ {
		codes = append(codes, h.do("POST", "/api/login", `{"password":"wrong password!"}`).Code)
	}
	if codes[0] != http.StatusUnauthorized || codes[6] != http.StatusTooManyRequests {
		t.Fatalf("expected throttling after repeated failures, got %v", codes)
	}
	if w := h.do("POST", "/api/login", `{"password":"correct horse battery"}`); w.Code != http.StatusTooManyRequests {
		t.Fatal("throttled client must wait even with the right password")
	}
	if w := h.do("POST", "/api/login", `{"password":"correct horse battery"}`, from("192.168.1.99")); w.Code != 200 {
		t.Fatalf("other client: %d", w.Code)
	}
}

// Passwords are hashed with Argon2id, wrong or malformed hashes never match,
// and setup codes are compared without caring about case or separators.
func TestPasswordHash(t *testing.T) {
	enc, err := HashPassword("a-password-that-is-long")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "argon2id$") || !VerifyPassword(enc, "a-password-that-is-long") || VerifyPassword(enc, "wrong") {
		t.Fatal("hash/verify")
	}
	for _, bad := range []string{"", "argon2id$v=19$m=999999999,t=1,p=1$AA$AA", "plain"} {
		if VerifyPassword(bad, "x") {
			t.Fatalf("accepted %q", bad)
		}
	}
	if HashSetupCode("abcd-efgh") != HashSetupCode("ABCD EFGH") {
		t.Fatal("setup code normalisation")
	}
}

// The web interface never builds HTML from strings, loads nothing from other hosts
// and cannot be used to read files outside the static folder.
func TestStaticAssetsAreSafe(t *testing.T) {
	b, err := staticFS.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "eval(", "new Function", "document.write"} {
		if strings.Contains(string(b), bad) {
			t.Errorf("app.js uses %s; build DOM with createElement and textContent", bad)
		}
	}
	idx, _ := staticFS.ReadFile("static/index.html")
	if strings.Contains(string(idx), "http://") || strings.Contains(string(idx), "https://") {
		t.Error("index.html must not load anything from other hosts")
	}
	h := newHarness(t)
	if w := h.do("GET", "/static/../state.json", ""); w.Code == 200 {
		t.Fatal("path traversal")
	}
}

// The client's IPv6 address is read correctly from the connection.
func TestClientAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "[fd00::5]:1234"
	if clientAddr(r) != netip.MustParseAddr("fd00::5") {
		t.Fatal(clientAddr(r))
	}
}
