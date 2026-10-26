// Package intel classifies domain names into tracking categories and knows
// which company owns well-known tracking domains. All data is local: an
// embedded seed list plus blocklist files downloaded on request.
package intel

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/publicsuffix"
)

// Categories known to Vahti. Threat is always blocked.
const (
	Ads       = "ads"
	Analytics = "analytics"
	Telemetry = "telemetry"
	Location  = "location"
	Social    = "social"
	Bypass    = "bypass"
	Threat    = "threat"
)

// AllCategories lists categories in display order.
var AllCategories = []string{Ads, Analytics, Telemetry, Location, Social, Bypass, Threat}

// ValidCategory reports whether c is a known category.
func ValidCategory(c string) bool {
	for _, k := range AllCategories {
		if k == c {
			return true
		}
	}
	return false
}

// Entry describes one domain in the database.
type Entry struct {
	Category string `json:"category"`
	Owner    string `json:"owner,omitempty"`
	Note     string `json:"note,omitempty"`
	Source   string `json:"source,omitempty"`
}

// Match is the result of a lookup.
type Match struct {
	Entry
	Domain string `json:"domain"` // the list entry that matched (may be a parent)
}

// Found reports whether the lookup matched anything.
func (m Match) Found() bool { return m.Category != "" }

// DB is a concurrency-safe domain database.
type DB struct {
	mu      sync.RWMutex
	entries map[string]Entry
	owners  map[string]string // registrable domain -> owner
	lists   []ListInfo
}

// ListInfo summarises one loaded list.
type ListInfo struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Entries  int    `json:"entries"`
	Error    string `json:"error,omitempty"`
}

//go:embed seed.json
var seedJSON []byte

type seedFile struct {
	Domains map[string]Entry `json:"domains"`
}

// New returns a database containing the embedded seed list.
func New() *DB {
	db := &DB{entries: map[string]Entry{}, owners: map[string]string{}}
	if err := db.LoadJSON(strings.NewReader(string(seedJSON)), "Vahti"); err != nil {
		panic("intel: embedded seed is invalid: " + err.Error())
	}
	return db
}

// LoadJSON merges a seed-format JSON document.
func (db *DB) LoadJSON(r io.Reader, source string) error {
	var sf seedFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sf); err != nil {
		return err
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	n := 0
	for d, e := range sf.Domains {
		d = Normalize(d)
		if !ValidName(d) || !ValidCategory(e.Category) {
			return fmt.Errorf("invalid entry %q", d)
		}
		if e.Source == "" {
			e.Source = source
		}
		db.entries[d] = e
		if e.Owner != "" {
			db.owners[Registrable(d)] = e.Owner
		}
		n++
	}
	db.lists = append(db.lists, ListInfo{Name: source, Category: "mixed", Entries: n})
	return nil
}

// LoadListFile loads a hosts/adblock/plain domain list from disk.
func (db *DB) LoadListFile(path, name, category string) ListInfo {
	info := ListInfo{Name: name, Category: category}
	f, err := os.Open(path)
	if err != nil {
		info.Error = "list not downloaded"
		db.addInfo(info)
		return info
	}
	defer f.Close()
	domains, err := ParseList(f, 2_000_000)
	if err != nil {
		info.Error = err.Error()
		db.addInfo(info)
		return info
	}
	db.mu.Lock()
	for _, d := range domains {
		if old, ok := db.entries[d]; ok && old.Owner != "" {
			continue // keep curated seed entries
		}
		db.entries[d] = Entry{Category: category, Source: name}
	}
	info.Entries = len(domains)
	db.lists = append(db.lists, info)
	db.mu.Unlock()
	return info
}

// addInfo records information (name, size, errors) about a loaded list.
func (db *DB) addInfo(i ListInfo) {
	db.mu.Lock()
	db.lists = append(db.lists, i)
	db.mu.Unlock()
}

// Lists returns information about loaded lists.
func (db *DB) Lists() []ListInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return append([]ListInfo(nil), db.lists...)
}

// Add inserts a single entry (used by demo mode and tests).
func (db *DB) Add(domain string, e Entry) {
	db.mu.Lock()
	defer db.mu.Unlock()
	d := Normalize(domain)
	db.entries[d] = e
	if e.Owner != "" {
		db.owners[Registrable(d)] = e.Owner
	}
}

// Lookup finds the most specific entry for name or any parent domain.
func (db *DB) Lookup(name string) Match {
	name = Normalize(name)
	db.mu.RLock()
	defer db.mu.RUnlock()
	for d := name; d != ""; d = parent(d) {
		if e, ok := db.entries[d]; ok {
			if e.Owner == "" {
				e.Owner = db.owners[Registrable(name)]
			}
			return Match{Entry: e, Domain: d}
		}
	}
	return Match{Entry: Entry{Owner: db.owners[Registrable(name)]}}
}

// Owner returns the known owner of name's registrable domain.
func (db *DB) Owner(name string) string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.owners[Registrable(Normalize(name))]
}

// parent removes the first label of a domain (a.b.c -> b.c). It returns "" instead of a
// bare top-level domain, so a list entry can never block a whole TLD.
func parent(d string) string {
	i := strings.IndexByte(d, '.')
	if i < 0 {
		return ""
	}
	p := d[i+1:]
	if !strings.Contains(p, ".") {
		return "" // never match a bare TLD
	}
	return p
}

// Normalize lower-cases and strips a trailing dot.
func Normalize(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
}

// Registrable returns the eTLD+1 (e.g. "a.b.example.co.uk" -> "example.co.uk").
func Registrable(name string) string {
	r, err := publicsuffix.EffectiveTLDPlusOne(Normalize(name))
	if err != nil {
		return Normalize(name)
	}
	return r
}

// ValidName checks that s is a syntactically valid DNS name with at least two labels.
func ValidName(s string) bool {
	if len(s) == 0 || len(s) > 253 || !strings.Contains(s, ".") {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			ok := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
			if !ok {
				return false
			}
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
	}
	return true
}

// ParseList reads hosts-format ("0.0.0.0 example.com"), adblock-format
// ("||example.com^") or plain one-domain-per-line lists.
func ParseList(r io.Reader, max int) ([]string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	seen := map[string]struct{}{}
	var out []string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' || line[0] == '!' || line[0] == '[' {
			continue
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		var d string
		switch {
		case strings.HasPrefix(line, "||"):
			d = strings.TrimPrefix(line, "||")
			if i := strings.IndexAny(d, "^$/"); i >= 0 {
				if d[i] != '^' || i != len(d)-1 {
					continue // only plain domain rules are supported
				}
				d = d[:i]
			}
		default:
			fs := strings.Fields(line)
			switch len(fs) {
			case 1:
				d = fs[0]
			case 2:
				if fs[0] != "0.0.0.0" && fs[0] != "127.0.0.1" && fs[0] != "::" && fs[0] != "::1" {
					continue
				}
				d = fs[1]
			default:
				continue
			}
		}
		d = Normalize(d)
		if d == "localhost" || d == "localhost.localdomain" || d == "local" || d == "broadcasthost" || d == "0.0.0.0" {
			continue
		}
		if !ValidName(d) {
			continue
		}
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
		if len(out) > max {
			return nil, fmt.Errorf("list is too large (over %d lines)", max)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Replace swaps in the contents of another database (used on reload).
func (db *DB) Replace(other *DB) {
	other.mu.RLock()
	e, o, l := other.entries, other.owners, other.lists
	other.mu.RUnlock()
	db.mu.Lock()
	db.entries, db.owners, db.lists = e, o, l
	db.mu.Unlock()
}
