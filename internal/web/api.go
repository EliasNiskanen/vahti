package web

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
)

// ---------------------------------------------------------------- session & auth

func (s *Server) apiSession(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"authenticated": false,
		"setupRequired": s.eng.Admin().PasswordHash == "",
		"demo":          s.cfg.Demo,
		"local":         s.cfg.Local,
		"version":       s.cfg.Version,
	}
	if c, err := r.Cookie(s.cookieName()); err == nil {
		if ss, ok := s.sess.get(c.Value); ok {
			out["authenticated"] = true
			out["csrf"] = ss.csrf
		}
	}
	if out["setupRequired"] == true {
		out["certFingerprint"] = s.cfg.CertFP
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) wait(w http.ResponseWriter, r *http.Request) bool {
	if d := s.thr.check(clientAddr(r)); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
		writeErr(w, http.StatusTooManyRequests, "Too many attempts. Wait "+humanDur(d)+" and try again.")
		return true
	}
	return false
}

func humanDur(d time.Duration) string {
	if d < time.Minute {
		return strconv.Itoa(int(d.Seconds())+1) + " seconds"
	}
	return strconv.Itoa(int(d.Minutes())+1) + " minutes"
}

func (s *Server) apiSetup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) || s.wait(w, r) {
		return
	}
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	adm := s.eng.Admin()
	if adm.PasswordHash != "" {
		writeErr(w, http.StatusConflict, "Vahti is already set up. Sign in.")
		return
	}
	if adm.SetupCodeHash == "" || subtle.ConstantTimeCompare([]byte(HashSetupCode(in.Code)), []byte(adm.SetupCodeHash)) != 1 {
		s.thr.fail(clientAddr(r))
		writeErr(w, http.StatusForbidden, "The setup code does not match. The code is shown in the Vahti log and in the file "+s.cfg.SetupCodePath+".")
		return
	}
	if err := ValidatePassword(in.Password); err != nil {
		writeErr(w, http.StatusBadRequest, PasswordMessage(err))
		return
	}
	h, err := HashPassword(in.Password)
	if err != nil {
		engineErr(w, err)
		return
	}
	s.eng.SetAdmin(engine.Admin{PasswordHash: h})
	if s.cfg.SetupCodePath != "" {
		_ = os.Remove(s.cfg.SetupCodePath)
	}
	s.thr.success(clientAddr(r))
	tok, csrf := s.sess.create()
	s.setSessionCookie(w, tok)
	slog.Info("setup completed", "client", clientAddr(r))
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "csrf": csrf})
}

func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) || s.wait(w, r) {
		return
	}
	adm := s.eng.Admin()
	if adm.PasswordHash == "" {
		writeErr(w, http.StatusConflict, "Vahti has not been set up yet.")
		return
	}
	if len(in.Password) > 256 || !VerifyPassword(adm.PasswordHash, in.Password) {
		s.thr.fail(clientAddr(r))
		slog.Warn("failed login", "client", clientAddr(r))
		writeErr(w, http.StatusUnauthorized, "Wrong password.")
		return
	}
	s.thr.success(clientAddr(r))
	tok, csrf := s.sess.create()
	s.setSessionCookie(w, tok)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "csrf": csrf})
}

func (s *Server) apiLogout(w http.ResponseWriter, r *http.Request, tok string) {
	s.sess.remove(tok)
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiPassword(w http.ResponseWriter, r *http.Request, tok string) {
	var in struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	if !readJSON(w, r, &in) || s.wait(w, r) {
		return
	}
	if len(in.Current) > 256 || !VerifyPassword(s.eng.Admin().PasswordHash, in.Current) {
		s.thr.fail(clientAddr(r))
		writeErr(w, http.StatusForbidden, "The current password is wrong.")
		return
	}
	if err := ValidatePassword(in.Next); err != nil {
		writeErr(w, http.StatusBadRequest, PasswordMessage(err))
		return
	}
	h, err := HashPassword(in.Next)
	if err != nil {
		engineErr(w, err)
		return
	}
	s.eng.SetAdmin(engine.Admin{PasswordHash: h})
	s.sess.removeAllExcept(tok)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------- data

type logOut struct {
	engine.LogEntry
	DeviceName string `json:"deviceName"`
	Owner      string `json:"owner,omitempty"`
}

func (s *Server) logEntries(in []engine.LogEntry) []logOut {
	names := map[string]string{}
	out := make([]logOut, 0, len(in))
	for _, le := range in {
		n, ok := names[le.Device]
		if !ok {
			n = s.eng.DeviceName(le.Device)
			names[le.Device] = n
		}
		out = append(out, logOut{LogEntry: le, DeviceName: n, Owner: s.db.Owner(le.Domain)})
	}
	return out
}

func (s *Server) apiOverview(w http.ResponseWriter, r *http.Request, _ string) {
	total, _, cats := s.eng.Stats()
	st := s.eng.Settings()
	lists := s.db.Lists()
	listed := 0
	for _, l := range lists {
		if l.Name != "Vahti" && l.Error == "" {
			listed += l.Entries
		}
	}
	var warnings []string
	if listed == 0 && !s.cfg.Demo {
		cmd := "sudo systemctl start vahti-update-lists"
		if s.cfg.Local {
			cmd = ".\\vahti.exe update-lists -local"
		}
		warnings = append(warnings, "No blocklists are loaded, so Vahti only recognises its ~50 built-in tracking services. Download the lists with: "+cmd)
	}
	out := map[string]any{
		"allowed":    total.Allowed,
		"blocked":    total.Blocked,
		"queue":      s.eng.QueueLen(),
		"devices":    len(s.eng.Devices()),
		"categories": cats,
		"recent":     s.logEntries(s.eng.Log(engine.LogFilter{Limit: 12})),
		"warnings":   warnings,
	}
	if time.Now().Before(st.PausedUntil) {
		out["pausedUntil"] = st.PausedUntil
	}
	writeJSON(w, http.StatusOK, out)
}

type deviceOut struct {
	engine.Device
	Allowed uint64 `json:"allowed"`
	Blocked uint64 `json:"blocked"`
}

func (s *Server) apiDevices(w http.ResponseWriter, r *http.Request, _ string) {
	_, per, _ := s.eng.Stats()
	devs := s.eng.Devices()
	out := make([]deviceOut, 0, len(devs))
	for _, d := range devs {
		c := per[d.ID]
		out = append(out, deviceOut{Device: d, Allowed: c.Allowed, Blocked: c.Blocked})
	}
	writeJSON(w, http.StatusOK, out)
}

type destOut struct {
	engine.Destination
	Owner string `json:"owner,omitempty"`
}

func (s *Server) apiDevice(w http.ResponseWriter, r *http.Request, _ string) {
	id := r.PathValue("id")
	d, ok := s.eng.Device(id)
	if !ok {
		engineErr(w, engine.ErrNotFound)
		return
	}
	_, per, _ := s.eng.Stats()
	dests := s.eng.Destinations(id, 60)
	do := make([]destOut, 0, len(dests))
	for _, x := range dests {
		do = append(do, destOut{Destination: x, Owner: s.db.Owner(x.Domain)})
	}
	var rules []engine.Rule
	for _, ru := range s.eng.Rules() {
		if ru.Device == id {
			rules = append(rules, ru)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device":       deviceOut{Device: d, Allowed: per[id].Allowed, Blocked: per[id].Blocked},
		"destinations": do,
		"rules":        rules,
		"defaults":     s.eng.Settings().CategoryDefaults,
	})
}

func (s *Server) apiDeviceUpdate(w http.ResponseWriter, r *http.Request, _ string) {
	var in engine.DeviceUpdate
	if !readJSON(w, r, &in) {
		return
	}
	d, err := s.eng.UpdateDevice(r.PathValue("id"), in)
	if err != nil {
		engineErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) apiDeviceDelete(w http.ResponseWriter, r *http.Request, _ string) {
	if err := s.eng.ForgetDevice(r.PathValue("id")); err != nil {
		engineErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type queueOut struct {
	engine.QueueItem
	DeviceName   string   `json:"deviceName"`
	DeviceKind   string   `json:"deviceKind"`
	Registrable  string   `json:"registrable"`
	Owner        string   `json:"owner,omitempty"`
	Category     string   `json:"category,omitempty"`
	Note         string   `json:"note,omitempty"`
	ListSource   string   `json:"listSource,omitempty"`
	OtherDevices []string `json:"otherDevices"`
	Siblings     []string `json:"siblings"`
	Vias         []string `json:"vias"`
}

func (s *Server) apiQueue(w http.ResponseWriter, r *http.Request, _ string) {
	items := s.eng.Queue()
	if len(items) > 100 {
		items = items[:100] // oldest first; the rest appear as these are decided
	}
	out := make([]queueOut, 0, len(items))
	for _, q := range items {
		m := s.db.Lookup(q.Domain)
		others, sib, via := s.eng.Related(q.Device, q.Domain)
		d, _ := s.eng.Device(q.Device)
		out = append(out, queueOut{
			QueueItem: q, DeviceName: d.Name, DeviceKind: d.Kind, Registrable: intel.Registrable(q.Domain),
			Owner: m.Owner, Category: m.Category, Note: m.Note, ListSource: m.Source,
			OtherDevices: others, Siblings: sib, Vias: via,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiQueueResolve(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct {
		Decision string `json:"decision"`
		Scope    string `json:"scope"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ru, err := s.eng.Resolve(r.PathValue("id"), in.Decision, in.Scope)
	if err != nil {
		engineErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ru)
}

type ruleOut struct {
	engine.Rule
	DeviceName string `json:"deviceName"`
}

func (s *Server) apiRules(w http.ResponseWriter, r *http.Request, _ string) {
	rs := s.eng.Rules()
	out := make([]ruleOut, 0, len(rs))
	for _, ru := range rs {
		out = append(out, ruleOut{Rule: ru, DeviceName: s.eng.DeviceName(ru.Device)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiRuleAdd(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct {
		Device string        `json:"device"`
		Domain string        `json:"domain"`
		Action engine.Action `json:"action"`
		Hours  int           `json:"hours"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Device == "" {
		in.Device = "*"
	}
	src := "user"
	if in.Action == engine.Allow && in.Hours > 0 {
		src = "share"
	}
	ru, err := s.eng.AddRule(in.Device, in.Domain, in.Action, in.Hours, false, src)
	if err != nil {
		engineErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ru)
}

func (s *Server) apiRuleDelete(w http.ResponseWriter, r *http.Request, _ string) {
	if err := s.eng.DeleteRule(r.PathValue("id")); err != nil {
		engineErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiSettings(w http.ResponseWriter, r *http.Request, _ string) {
	st := s.eng.Settings()
	out := map[string]any{
		"categories":    st.CategoryDefaults,
		"newDeviceMode": st.NewDeviceMode,
		"lists":         s.db.Lists(),
		"upstreams":     s.cfg.Upstreams,
		"version":       s.cfg.Version,
	}
	if time.Now().Before(st.PausedUntil) {
		out["pausedUntil"] = st.PausedUntil
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiSettingsUpdate(w http.ResponseWriter, r *http.Request, _ string) {
	var in engine.SettingsUpdate
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.eng.UpdateSettings(in); err != nil {
		engineErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiPause(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct {
		Minutes int `json:"minutes"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	until, err := s.eng.Pause(in.Minutes)
	if err != nil {
		engineErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pausedUntil": until})
}

func (s *Server) apiLog(w http.ResponseWriter, r *http.Request, _ string) {
	q := r.URL.Query()
	f := engine.LogFilter{Device: q.Get("device"), Query: q.Get("q")}
	switch a := q.Get("action"); a {
	case "allow", "block":
		f.Action = engine.Action(a)
	case "", "all":
	default:
		writeErr(w, http.StatusBadRequest, "Unknown filter.")
		return
	}
	if len(f.Query) > 253 || len(f.Device) > 100 {
		writeErr(w, http.StatusBadRequest, "The search is too long.")
		return
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	writeJSON(w, http.StatusOK, s.logEntries(s.eng.Log(f)))
}
