package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// Password hashing: Argon2id with OWASP-recommended parameters.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
)

// hashSem limits concurrent hashing so login floods cannot exhaust memory.
var hashSem = make(chan struct{}, 2)

// HashPassword returns an encoded Argon2id hash.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hashSem <- struct{}{}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	<-hashSem
	enc := base64.RawStdEncoding
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword checks pw against an encoded hash in constant time.
func VerifyPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	if m == 0 || m > 1024*1024 || t == 0 || t > 20 || p == 0 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[3])
	want, err2 := enc.DecodeString(parts[4])
	if err1 != nil || err2 != nil || len(want) == 0 || len(want) > 128 {
		return false
	}
	hashSem <- struct{}{}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	<-hashSem
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Password policy errors. PasswordMessage turns them into text for the user.
var (
	ErrPasswordShort = errors.New("password shorter than 12 characters")
	ErrPasswordLong  = errors.New("password longer than 256 bytes")
)

// ValidatePassword enforces the password policy.
func ValidatePassword(pw string) error {
	if len([]rune(pw)) < 12 {
		return ErrPasswordShort
	}
	if len(pw) > 256 {
		return ErrPasswordLong
	}
	return nil
}

// PasswordMessage explains a ValidatePassword error to the user.
func PasswordMessage(err error) string {
	if errors.Is(err, ErrPasswordLong) {
		return "The password is too long (256 characters at most)."
	}
	return "The password must be at least 12 characters long."
}

// NewSetupCode returns a human-friendly one-time code and its hash.
func NewSetupCode() (code, hash string) {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	raw := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b) // 16 chars
	code = raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16]
	return code, HashSetupCode(code)
}

// HashSetupCode normalises and hashes a setup code.
func HashSetupCode(code string) string {
	// Codes use base32 (A-Z, 2-7); accept the digits people confuse with letters.
	c := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(code))
	c = strings.NewReplacer("0", "O", "1", "I", "8", "B").Replace(c)
	sum := sha256.Sum256([]byte("vahti-setup:" + c))
	return base64.RawStdEncoding.EncodeToString(sum[:])
}

// ---------------------------------------------------------------- sessions

type session struct {
	csrf     string
	created  time.Time
	lastSeen time.Time
}

const (
	sessionIdle     = 2 * time.Hour
	sessionAbsolute = 12 * time.Hour
	maxSessions     = 20
)

type sessions struct {
	mu  sync.Mutex
	m   map[string]*session // key: sha256(token)
	now func() time.Time
}

// newSessions creates an empty session store.
func newSessions() *sessions { return &sessions{m: map[string]*session{}, now: time.Now} }

// tokenKey hashes a session token with SHA-256. Only the hash is kept in memory, so a
// memory dump does not reveal usable tokens.
func tokenKey(tok string) string {
	s := sha256.Sum256([]byte(tok))
	return base64.RawStdEncoding.EncodeToString(s[:])
}

// randToken returns 32 random bytes from the system's secure random source, base64-encoded.
func randToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// create returns a new session token and its CSRF token.
func (s *sessions) create() (token, csrf string) {
	token, csrf = randToken(), randToken()
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.m) >= maxSessions {
		var oldestK string
		var oldest time.Time
		for k, v := range s.m {
			if oldestK == "" || v.lastSeen.Before(oldest) {
				oldestK, oldest = k, v.lastSeen
			}
		}
		delete(s.m, oldestK)
	}
	s.m[tokenKey(token)] = &session{csrf: csrf, created: now, lastSeen: now}
	return token, csrf
}

// get validates a token and refreshes its idle timer.
func (s *sessions) get(token string) (*session, bool) {
	if token == "" || len(token) > 100 {
		return nil, false
	}
	k := tokenKey(token)
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	if !ok {
		return nil, false
	}
	if now.Sub(v.lastSeen) > sessionIdle || now.Sub(v.created) > sessionAbsolute {
		delete(s.m, k)
		return nil, false
	}
	v.lastSeen = now
	c := *v
	return &c, true
}

// remove ends one session (logout).
func (s *sessions) remove(token string) {
	s.mu.Lock()
	delete(s.m, tokenKey(token))
	s.mu.Unlock()
}

// removeAllExcept logs out every other session (after a password change).
func (s *sessions) removeAllExcept(token string) {
	keep := tokenKey(token)
	s.mu.Lock()
	for k := range s.m {
		if k != keep {
			delete(s.m, k)
		}
	}
	s.mu.Unlock()
}

// ---------------------------------------------------------------- login throttling

// throttle slows down password guessing per client and globally.
type throttle struct {
	mu     sync.Mutex
	fails  map[netip.Addr]*failRec
	global []time.Time
	now    func() time.Time
}

type failRec struct {
	n     int
	until time.Time
	last  time.Time
}

// newThrottle creates an empty login throttle.
func newThrottle() *throttle { return &throttle{fails: map[netip.Addr]*failRec{}, now: time.Now} }

// check returns how long the client must wait before trying again.
func (t *throttle) check(a netip.Addr) time.Duration {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	if r := t.fails[a]; r != nil && now.Before(r.until) {
		return r.until.Sub(now)
	}
	// global: at most 30 failed attempts per 10 minutes from all clients together
	cut := now.Add(-10 * time.Minute)
	i := 0
	for i < len(t.global) && t.global[i].Before(cut) {
		i++
	}
	t.global = t.global[i:]
	if len(t.global) >= 30 {
		return t.global[0].Add(10 * time.Minute).Sub(now)
	}
	return 0
}

// fail records a failed password attempt from this address. From the fifth failure on the
// client must wait 15 s, then 30 s, 60 s and so on, at most 30 minutes. A quiet hour resets it.
func (t *throttle) fail(a netip.Addr) {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	r := t.fails[a]
	if r == nil || now.Sub(r.last) > time.Hour {
		r = &failRec{}
		t.fails[a] = r
	}
	r.n++
	r.last = now
	if r.n >= 5 {
		d := time.Duration(1<<min(r.n-5, 10)) * time.Second * 15 // 15s, 30s, ... capped
		if d > 30*time.Minute {
			d = 30 * time.Minute
		}
		r.until = now.Add(d)
	}
	t.global = append(t.global, now)
	if len(t.fails) > 10000 {
		for k, v := range t.fails {
			if now.Sub(v.last) > time.Hour {
				delete(t.fails, k)
			}
		}
	}
}

// success clears the failure count after a correct password.
func (t *throttle) success(a netip.Addr) {
	t.mu.Lock()
	delete(t.fails, a)
	t.mu.Unlock()
}
