// Package engine holds Vahti's decision logic: devices, rules, the approval
// queue, the in-memory traffic log and persisted settings.
package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/EliasNiskanen/vahti/internal/intel"
)

// Action is a filtering decision.
type Action string

const (
	Allow Action = "allow"
	Block Action = "block"
)

// Mode controls how a device's unknown destinations are handled.
type Mode string

const (
	// Open allows unknown destinations; only known tracking categories are filtered.
	Open Mode = "open"
	// Learning allows unknown destinations and records them as allowed, then locks.
	Learning Mode = "learning"
	// Locked blocks unknown destinations and puts them in the approval queue.
	Locked Mode = "locked"
)

// ValidMode reports whether m is a known mode.
func ValidMode(m Mode) bool { return m == Open || m == Learning || m == Locked }

// Device is one client on the home network.
type Device struct {
	ID          string            `json:"id"`
	MAC         string            `json:"mac,omitempty"`
	IP          string            `json:"ip"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	Mode        Mode              `json:"mode"`
	LearnUntil  time.Time         `json:"learnUntil,omitzero"`
	FirstSeen   time.Time         `json:"firstSeen"`
	LastSeen    time.Time         `json:"lastSeen"`
	CatOverride map[string]Action `json:"catOverride,omitempty"`
}

// Rule allows or blocks a domain (and its subdomains) for one device or all ("*").
type Rule struct {
	ID      string    `json:"id"`
	Device  string    `json:"device"`
	Domain  string    `json:"domain"`
	Action  Action    `json:"action"`
	Until   time.Time `json:"until,omitzero"`
	Once    bool      `json:"once,omitempty"`
	Source  string    `json:"source"` // user, queue, learned, share
	Created time.Time `json:"created"`
}

// QueueItem is an unknown destination a locked device tried to reach.
type QueueItem struct {
	ID     string    `json:"id"`
	Device string    `json:"device"`
	Domain string    `json:"domain"`
	Count  int       `json:"count"`
	First  time.Time `json:"first"`
	Last   time.Time `json:"last"`
}

// Settings are the user's global choices.
type Settings struct {
	CategoryDefaults map[string]Action `json:"categoryDefaults"`
	NewDeviceMode    Mode              `json:"newDeviceMode"`
	PausedUntil      time.Time         `json:"pausedUntil,omitzero"`
}

// Admin holds credentials. The setup code is stored only as a hash.
type Admin struct {
	PasswordHash  string `json:"passwordHash,omitempty"`
	SetupCodeHash string `json:"setupCodeHash,omitempty"`
}

// State is everything persisted to disk.
type State struct {
	Version  int                   `json:"version"`
	Settings Settings              `json:"settings"`
	Devices  map[string]*Device    `json:"devices"`
	Rules    map[string]*Rule      `json:"rules"`
	Queue    map[string]*QueueItem `json:"queue"`
	Admin    Admin                 `json:"admin"`
}

// Verdict is the outcome of a decision.
type Verdict struct {
	Action   Action
	Reason   string
	Category string
	Queued   bool
}

// LogEntry is one DNS decision in the in-memory log.
type LogEntry struct {
	Seq      uint64    `json:"seq"`
	Time     time.Time `json:"time"`
	Device   string    `json:"device"`
	Domain   string    `json:"domain"`
	QType    string    `json:"qtype"`
	Category string    `json:"category,omitempty"`
	Action   Action    `json:"action"`
	Reason   string    `json:"reason"`
	Via      string    `json:"via,omitempty"` // CNAME target that triggered a block
	Queued   bool      `json:"queued,omitempty"`
}

// Engine is safe for concurrent use.
type Engine struct {
	mu    sync.Mutex
	st    *State
	dirty bool
	db    *intel.DB
	now   func() time.Time

	log    []LogEntry
	logPos int
	logN   int
	seq    uint64

	stats map[string]*Counter // per device
	cats  map[string]uint64   // blocked count per category
	total Counter
}

// Counter counts decisions.
type Counter struct {
	Allowed uint64 `json:"allowed"`
	Blocked uint64 `json:"blocked"`
}

// DefaultSettings returns privacy-first defaults: every tracking category blocked.
func DefaultSettings() Settings {
	m := map[string]Action{}
	for _, c := range intel.AllCategories {
		m[c] = Block
	}
	return Settings{CategoryDefaults: m, NewDeviceMode: Open}
}

// NewState returns an empty state with defaults.
func NewState() *State {
	return &State{
		Version:  1,
		Settings: DefaultSettings(),
		Devices:  map[string]*Device{},
		Rules:    map[string]*Rule{},
		Queue:    map[string]*QueueItem{},
	}
}

// New creates an engine around a state. logSize bounds the in-memory log.
func New(st *State, db *intel.DB, logSize int) *Engine {
	if st == nil {
		st = NewState()
	}
	normalizeState(st)
	if logSize < 100 {
		logSize = 100
	}
	return &Engine{
		st: st, db: db, now: time.Now,
		log:   make([]LogEntry, logSize),
		stats: map[string]*Counter{}, cats: map[string]uint64{},
	}
}

func normalizeState(st *State) {
	if st.Devices == nil {
		st.Devices = map[string]*Device{}
	}
	if st.Rules == nil {
		st.Rules = map[string]*Rule{}
	}
	if st.Queue == nil {
		st.Queue = map[string]*QueueItem{}
	}
	def := DefaultSettings()
	if st.Settings.CategoryDefaults == nil {
		st.Settings.CategoryDefaults = def.CategoryDefaults
	}
	for _, c := range intel.AllCategories {
		if _, ok := st.Settings.CategoryDefaults[c]; !ok {
			st.Settings.CategoryDefaults[c] = Block
		}
	}
	st.Settings.CategoryDefaults[intel.Threat] = Block
	if !ValidMode(st.Settings.NewDeviceMode) {
		st.Settings.NewDeviceMode = Open
	}
	for _, d := range st.Devices {
		if !ValidMode(d.Mode) {
			d.Mode = Open
		}
	}
}

// SetClock replaces the time source (tests).
func (e *Engine) SetClock(f func() time.Time) { e.now = f }

// DB returns the intel database.
func (e *Engine) DB() *intel.DB { return e.db }

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------- devices

// MaxDevices bounds how many devices are remembered.
const MaxDevices = 2000

// OverflowDevice is used for clients seen after MaxDevices is reached. It is
// never stored and is treated like an open device.
const OverflowDevice = "overflow"

// MaxRules bounds the number of stored rules, including learned ones.
const MaxRules = 20000

// Identify returns the device ID for a client, creating the device on first sight.
// mac may be empty when the ARP table does not know the client.
func (e *Engine) Identify(ip netip.Addr, mac string) string {
	now := e.now()
	ipS := ip.String()
	id := "ip:" + ipS
	if mac != "" {
		id = "mac:" + strings.ToLower(mac)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	d, ok := e.st.Devices[id]
	if !ok && len(e.st.Devices) >= MaxDevices {
		// Spoofed source addresses must not grow state without bound.
		return OverflowDevice
	}
	if !ok {
		d = &Device{
			ID: id, MAC: mac, IP: ipS, Name: defaultName(ip), Kind: "unknown",
			Mode: e.st.Settings.NewDeviceMode, FirstSeen: now,
		}
		if d.Mode == Learning {
			d.LearnUntil = now.Add(48 * time.Hour)
		}
		e.st.Devices[id] = d
		e.dirty = true
	}
	if d.IP != ipS {
		d.IP = ipS
		e.dirty = true
	}
	if now.Sub(d.LastSeen) > time.Minute {
		e.dirty = true // persist last-seen coarsely
	}
	d.LastSeen = now
	return id
}

func defaultName(ip netip.Addr) string {
	if ip.IsLoopback() {
		return "Vahti-laite itse"
	}
	return "Uusi laite " + ip.String()
}

// SetDeviceNameHint names a device if the user has not named it yet.
func (e *Engine) SetDeviceNameHint(id, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.st.Devices[id]
	if d == nil || name == "" || !strings.HasPrefix(d.Name, "Uusi laite ") {
		return
	}
	d.Name = name
	e.dirty = true
}

// DeviceUpdate holds optional device changes.
type DeviceUpdate struct {
	Name        *string            `json:"name"`
	Kind        *string            `json:"kind"`
	Mode        *Mode              `json:"mode"`
	LearnHours  *int               `json:"learnHours"`
	CatOverride map[string]*Action `json:"catOverride"` // nil value clears the override
}

// ErrNotFound is returned for unknown IDs.
var ErrNotFound = errors.New("not found")

// ErrInvalid is returned for invalid input.
var ErrInvalid = errors.New("invalid input")

// UpdateDevice applies a validated update.
func (e *Engine) UpdateDevice(id string, u DeviceUpdate) (Device, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.st.Devices[id]
	if d == nil {
		return Device{}, ErrNotFound
	}
	if u.Name != nil {
		n := strings.TrimSpace(*u.Name)
		if n == "" || len([]rune(n)) > 60 {
			return Device{}, ErrInvalid
		}
		d.Name = n
	}
	if u.Kind != nil {
		if !validKind(*u.Kind) {
			return Device{}, ErrInvalid
		}
		d.Kind = *u.Kind
	}
	if u.Mode != nil {
		if !ValidMode(*u.Mode) {
			return Device{}, ErrInvalid
		}
		d.Mode = *u.Mode
		if d.Mode == Learning {
			h := 48
			if u.LearnHours != nil {
				h = *u.LearnHours
			}
			if h < 1 || h > 24*14 {
				return Device{}, ErrInvalid
			}
			d.LearnUntil = e.now().Add(time.Duration(h) * time.Hour)
		} else {
			d.LearnUntil = time.Time{}
		}
	}
	for c, a := range u.CatOverride {
		if !intel.ValidCategory(c) || c == intel.Threat {
			return Device{}, ErrInvalid
		}
		if a == nil {
			delete(d.CatOverride, c)
			continue
		}
		if *a != Allow && *a != Block {
			return Device{}, ErrInvalid
		}
		if d.CatOverride == nil {
			d.CatOverride = map[string]Action{}
		}
		d.CatOverride[c] = *a
	}
	e.dirty = true
	return copyDevice(d), nil
}

var kinds = []string{"unknown", "phone", "computer", "tablet", "tv", "speaker", "camera", "appliance", "console", "other"}

func validKind(k string) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}

// ForgetDevice removes a device and its rules and queue items.
func (e *Engine) ForgetDevice(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.st.Devices[id]; !ok {
		return ErrNotFound
	}
	delete(e.st.Devices, id)
	for k, r := range e.st.Rules {
		if r.Device == id {
			delete(e.st.Rules, k)
		}
	}
	for k, q := range e.st.Queue {
		if q.Device == id {
			delete(e.st.Queue, k)
		}
	}
	delete(e.stats, id)
	e.dirty = true
	return nil
}

func copyDevice(d *Device) Device {
	c := *d
	if d.CatOverride != nil {
		c.CatOverride = make(map[string]Action, len(d.CatOverride))
		for k, v := range d.CatOverride {
			c.CatOverride[k] = v
		}
	}
	return c
}

// Devices returns copies of all devices sorted by last activity.
func (e *Engine) Devices() []Device {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Device, 0, len(e.st.Devices))
	for _, d := range e.st.Devices {
		out = append(out, copyDevice(d))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

// Device returns one device.
func (e *Engine) Device(id string) (Device, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	d, ok := e.st.Devices[id]
	if !ok {
		return Device{}, false
	}
	return copyDevice(d), true
}

// DeviceName returns the display name for an ID ("*" means all devices).
func (e *Engine) DeviceName(id string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.deviceNameLocked(id)
}

func (e *Engine) deviceNameLocked(id string) string {
	if id == "*" {
		return "Kaikki laitteet"
	}
	if id == OverflowDevice {
		return "Muut laitteet"
	}
	if d := e.st.Devices[id]; d != nil {
		return d.Name
	}
	return id
}

// ---------------------------------------------------------------- decisions

// Decide applies the rules for a DNS query. It may create queue items and
// learned rules, and consumes one-time rules.
func (e *Engine) Decide(devID, name string, m intel.Match) Verdict {
	return e.decide(devID, intel.Normalize(name), m, true)
}

// DecideCNAME evaluates a CNAME target found in an upstream answer. Unknown
// targets are never queued: the original name was already approved.
func (e *Engine) DecideCNAME(devID, target string, m intel.Match) Verdict {
	return e.decide(devID, intel.Normalize(target), m, false)
}

func (e *Engine) decide(devID, name string, m intel.Match, primary bool) Verdict {
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	cat := m.Category
	if cat == intel.Threat {
		return Verdict{Block, "Uhkalista", cat, false}
	}
	if now.Before(e.st.Settings.PausedUntil) {
		return Verdict{Allow, "Suojaus tauolla", cat, false}
	}
	if r := e.matchRuleLocked(devID, name, now); r != nil {
		reason := "Oma sääntö"
		switch {
		case r.Once:
			reason = "Kertalupa"
			delete(e.st.Rules, r.ID)
			e.dirty = true
		case !r.Until.IsZero():
			reason = "Määräaikainen lupa"
		case r.Source == "learned":
			reason = "Opittu kohde"
		case r.Device == "*":
			reason = "Yhteinen sääntö"
		}
		return Verdict{r.Action, reason, cat, false}
	}
	d := e.st.Devices[devID]
	if cat != "" {
		if d != nil {
			if a, ok := d.CatOverride[cat]; ok {
				return Verdict{a, "Laitekohtainen asetus", cat, false}
			}
		}
		return Verdict{e.st.Settings.CategoryDefaults[cat], "Oletusasetus", cat, false}
	}
	if d == nil || !primary {
		return Verdict{Allow, "Tuntematon kohde", "", false}
	}
	if d.Mode == Learning && !now.Before(d.LearnUntil) {
		d.Mode = Locked
		d.LearnUntil = time.Time{}
		e.dirty = true
	}
	switch d.Mode {
	case Learning:
		if len(e.st.Rules) >= MaxRules {
			return Verdict{Allow, "Opettelu: sääntöjen enimmäismäärä täynnä", "", false}
		}
		site := intel.Registrable(name)
		r := &Rule{ID: newID(), Device: devID, Domain: site, Action: Allow, Source: "learned", Created: now}
		e.st.Rules[r.ID] = r
		e.dirty = true
		return Verdict{Allow, "Opettelu: kohde tallennettiin sallituksi", "", false}
	case Locked:
		var q *QueueItem
		for _, it := range e.st.Queue {
			if it.Device == devID && it.Domain == name {
				q = it
				break
			}
		}
		if q == nil {
			if len(e.st.Queue) >= 500 {
				return Verdict{Block, "Lukittu laite: jono on täynnä", "", false}
			}
			q = &QueueItem{ID: newID(), Device: devID, Domain: name, First: now}
			e.st.Queue[q.ID] = q
		}
		q.Count++
		q.Last = now
		e.dirty = true
		return Verdict{Block, "Odottaa päätöstäsi", "", true}
	}
	return Verdict{Allow, "Tuntematon kohde", "", false}
}

// matchRuleLocked finds the most specific live rule for device, then global.
func (e *Engine) matchRuleLocked(devID, name string, now time.Time) *Rule {
	var best *Rule
	bestLen := -1
	bestDev := false
	for id, r := range e.st.Rules {
		if !r.Until.IsZero() && !now.Before(r.Until) {
			delete(e.st.Rules, id)
			e.dirty = true
			continue
		}
		if r.Device != devID && r.Device != "*" {
			continue
		}
		if name != r.Domain && !strings.HasSuffix(name, "."+r.Domain) {
			continue
		}
		isDev := r.Device == devID
		// device rules beat global rules; then the most specific domain wins;
		// on a tie, block beats allow.
		better := false
		switch {
		case best == nil:
			better = true
		case isDev != bestDev:
			better = isDev
		case len(r.Domain) != bestLen:
			better = len(r.Domain) > bestLen
		default:
			better = r.Action == Block && best.Action == Allow
		}
		if better {
			best, bestLen, bestDev = r, len(r.Domain), isDev
		}
	}
	return best
}

// ---------------------------------------------------------------- rules & queue

// AddRule validates and stores a rule. hours<=0 means permanent.
func (e *Engine) AddRule(device, domain string, action Action, hours int, once bool, source string) (Rule, error) {
	domain = intel.Normalize(domain)
	if !intel.ValidName(domain) || (action != Allow && action != Block) || hours < 0 || hours > 24*365 {
		return Rule{}, ErrInvalid
	}
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	if device != "*" {
		if _, ok := e.st.Devices[device]; !ok {
			return Rule{}, ErrNotFound
		}
	}
	if len(e.st.Rules) >= MaxRules {
		return Rule{}, ErrInvalid
	}
	// replace an existing rule for the same device and domain
	for id, r := range e.st.Rules {
		if r.Device == device && r.Domain == domain {
			delete(e.st.Rules, id)
		}
	}
	r := &Rule{ID: newID(), Device: device, Domain: domain, Action: action, Once: once, Source: source, Created: now}
	if hours > 0 {
		r.Until = now.Add(time.Duration(hours) * time.Hour)
	}
	e.st.Rules[r.ID] = r
	e.dirty = true
	return *r, nil
}

// DeleteRule removes a rule.
func (e *Engine) DeleteRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.st.Rules[id]; !ok {
		return ErrNotFound
	}
	delete(e.st.Rules, id)
	e.dirty = true
	return nil
}

// Rules returns live rules, newest first.
func (e *Engine) Rules() []Rule {
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	out := []Rule{}
	for id, r := range e.st.Rules {
		if !r.Until.IsZero() && !now.Before(r.Until) {
			delete(e.st.Rules, id)
			e.dirty = true
			continue
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

// Queue returns pending items, oldest first, in a stable order.
func (e *Engine) Queue() []QueueItem {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]QueueItem, 0, len(e.st.Queue))
	for _, q := range e.st.Queue {
		out = append(out, *q)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].First.Equal(out[j].First) {
			return out[i].First.Before(out[j].First)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// QueueLen returns the number of pending items.
func (e *Engine) QueueLen() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.st.Queue)
}

// Resolve answers a queue item. decision: once, hour, allow, block.
// scope "site" applies the rule to the registrable domain.
func (e *Engine) Resolve(id, decision, scope string) (Rule, error) {
	e.mu.Lock()
	q, ok := e.st.Queue[id]
	if !ok {
		e.mu.Unlock()
		return Rule{}, ErrNotFound
	}
	item := *q
	e.mu.Unlock()

	domain := item.Domain
	if scope == "site" {
		domain = intel.Registrable(domain)
	} else if scope != "name" && scope != "" {
		return Rule{}, ErrInvalid
	}
	var r Rule
	var err error
	switch decision {
	case "once":
		r, err = e.AddRule(item.Device, domain, Allow, 0, true, "queue")
	case "hour":
		r, err = e.AddRule(item.Device, domain, Allow, 1, false, "queue")
	case "allow":
		r, err = e.AddRule(item.Device, domain, Allow, 0, false, "queue")
	case "block":
		r, err = e.AddRule(item.Device, domain, Block, 0, false, "queue")
	default:
		return Rule{}, ErrInvalid
	}
	if err != nil {
		return Rule{}, err
	}
	e.mu.Lock()
	delete(e.st.Queue, id)
	if scope == "site" { // the site rule covers sibling queue items too
		for k, it := range e.st.Queue {
			if it.Device == item.Device && (it.Domain == domain || strings.HasSuffix(it.Domain, "."+domain)) {
				delete(e.st.Queue, k)
			}
		}
	}
	e.dirty = true
	e.mu.Unlock()
	return r, nil
}

// ---------------------------------------------------------------- settings

// Settings returns a copy of the settings.
func (e *Engine) Settings() Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.st.Settings
	s.CategoryDefaults = map[string]Action{}
	for k, v := range e.st.Settings.CategoryDefaults {
		s.CategoryDefaults[k] = v
	}
	return s
}

// CategoryAction returns the global default for a category.
func (e *Engine) CategoryAction(cat string) Action {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.st.Settings.CategoryDefaults[cat]
}

// SettingsUpdate holds optional settings changes.
type SettingsUpdate struct {
	Categories    map[string]Action `json:"categories"`
	NewDeviceMode *Mode             `json:"newDeviceMode"`
}

// UpdateSettings validates and applies changes.
func (e *Engine) UpdateSettings(u SettingsUpdate) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for c, a := range u.Categories {
		if !intel.ValidCategory(c) || (a != Allow && a != Block) {
			return ErrInvalid
		}
		if c == intel.Threat && a != Block {
			return ErrInvalid
		}
	}
	if u.NewDeviceMode != nil && !ValidMode(*u.NewDeviceMode) {
		return ErrInvalid
	}
	for c, a := range u.Categories {
		e.st.Settings.CategoryDefaults[c] = a
	}
	if u.NewDeviceMode != nil {
		e.st.Settings.NewDeviceMode = *u.NewDeviceMode
	}
	e.dirty = true
	return nil
}

// Pause turns filtering off for the given minutes (0 resumes).
func (e *Engine) Pause(minutes int) (time.Time, error) {
	if minutes < 0 || minutes > 24*60 {
		return time.Time{}, ErrInvalid
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if minutes == 0 {
		e.st.Settings.PausedUntil = time.Time{}
	} else {
		e.st.Settings.PausedUntil = e.now().Add(time.Duration(minutes) * time.Minute)
	}
	e.dirty = true
	return e.st.Settings.PausedUntil, nil
}

// ---------------------------------------------------------------- admin

// Admin returns the credential record.
func (e *Engine) Admin() Admin {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.st.Admin
}

// SetAdmin replaces the credential record.
func (e *Engine) SetAdmin(a Admin) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.st.Admin = a
	e.dirty = true
}

// ---------------------------------------------------------------- log & stats

// Record appends a decision to the in-memory log and updates counters.
func (e *Engine) Record(le LogEntry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seq++
	le.Seq = e.seq
	if le.Time.IsZero() {
		le.Time = e.now()
	}
	e.log[e.logPos] = le
	e.logPos = (e.logPos + 1) % len(e.log)
	if e.logN < len(e.log) {
		e.logN++
	}
	c := e.stats[le.Device]
	if c == nil {
		c = &Counter{}
		e.stats[le.Device] = c
	}
	if le.Action == Block {
		c.Blocked++
		e.total.Blocked++
		if le.Category != "" {
			e.cats[le.Category]++
		}
	} else {
		c.Allowed++
		e.total.Allowed++
	}
}

// LogFilter selects log entries.
type LogFilter struct {
	Device string
	Action Action
	Query  string
	Limit  int
}

// Log returns newest-first entries matching f.
func (e *Engine) Log(f LogFilter) []LogEntry {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 200
	}
	q := strings.ToLower(f.Query)
	e.mu.Lock()
	defer e.mu.Unlock()
	out := []LogEntry{}
	for i := 0; i < e.logN && len(out) < f.Limit; i++ {
		idx := (e.logPos - 1 - i + len(e.log)) % len(e.log)
		le := e.log[idx]
		if f.Device != "" && le.Device != f.Device {
			continue
		}
		if f.Action != "" && le.Action != f.Action {
			continue
		}
		if q != "" && !strings.Contains(le.Domain, q) {
			continue
		}
		out = append(out, le)
	}
	return out
}

// Destination aggregates log entries for one device and domain.
type Destination struct {
	Domain   string    `json:"domain"`
	Category string    `json:"category,omitempty"`
	Allowed  int       `json:"allowed"`
	Blocked  int       `json:"blocked"`
	Last     time.Time `json:"last"`
}

// Destinations summarises where a device has connected (from the in-memory log).
func (e *Engine) Destinations(devID string, limit int) []Destination {
	e.mu.Lock()
	m := map[string]*Destination{}
	for i := 0; i < e.logN; i++ {
		idx := (e.logPos - 1 - i + len(e.log)) % len(e.log)
		le := e.log[idx]
		if le.Device != devID {
			continue
		}
		d := m[le.Domain]
		if d == nil {
			d = &Destination{Domain: le.Domain, Category: le.Category, Last: le.Time}
			m[le.Domain] = d
		}
		if le.Action == Block {
			d.Blocked++
		} else {
			d.Allowed++
		}
	}
	e.mu.Unlock()
	out := make([]Destination, 0, len(m))
	for _, d := range m {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].Allowed+out[i].Blocked, out[j].Allowed+out[j].Blocked
		if ti != tj {
			return ti > tj
		}
		return out[i].Domain < out[j].Domain
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Related finds what the log knows about a domain: other devices that asked
// for it and sibling names under the same registrable domain.
func (e *Engine) Related(devID, name string) (otherDevices []string, siblings []string, vias []string) {
	site := intel.Registrable(name)
	e.mu.Lock()
	defer e.mu.Unlock()
	devs := map[string]bool{}
	sib := map[string]bool{}
	via := map[string]bool{}
	for i := 0; i < e.logN; i++ {
		idx := (e.logPos - 1 - i + len(e.log)) % len(e.log)
		le := e.log[idx]
		if le.Domain == name {
			if le.Device != devID {
				devs[e.deviceNameLocked(le.Device)] = true
			}
			if le.Via != "" {
				via[le.Via] = true
			}
		} else if le.Domain == site || strings.HasSuffix(le.Domain, "."+site) {
			sib[le.Domain] = true
		}
	}
	return keys(devs, 10), keys(sib, 10), keys(via, 5)
}

func keys(m map[string]bool, max int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// Stats returns totals, per-device counters and blocked-per-category counts.
func (e *Engine) Stats() (Counter, map[string]Counter, map[string]uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	per := make(map[string]Counter, len(e.stats))
	for k, v := range e.stats {
		per[k] = *v
	}
	cats := make(map[string]uint64, len(e.cats))
	for k, v := range e.cats {
		cats[k] = v
	}
	return e.total, per, cats
}

// ---------------------------------------------------------------- persistence

// Snapshot returns a deep copy of the state if it changed since the last call.
func (e *Engine) Snapshot(force bool) (*State, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.dirty && !force {
		return nil, false
	}
	e.dirty = false
	s := &State{Version: e.st.Version, Settings: e.st.Settings, Admin: e.st.Admin,
		Devices: map[string]*Device{}, Rules: map[string]*Rule{}, Queue: map[string]*QueueItem{}}
	s.Settings.CategoryDefaults = map[string]Action{}
	for k, v := range e.st.Settings.CategoryDefaults {
		s.Settings.CategoryDefaults[k] = v
	}
	for k, d := range e.st.Devices {
		c := copyDevice(d)
		s.Devices[k] = &c
	}
	for k, r := range e.st.Rules {
		c := *r
		s.Rules[k] = &c
	}
	for k, q := range e.st.Queue {
		c := *q
		s.Queue[k] = &c
	}
	return s, true
}

// MarkDirty forces the next Snapshot to return state (used after failed saves).
func (e *Engine) MarkDirty() {
	e.mu.Lock()
	e.dirty = true
	e.mu.Unlock()
}
