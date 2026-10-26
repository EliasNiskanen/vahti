package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EliasNiskanen/vahti/internal/store"
)

// EnsureCert loads or creates a self-signed certificate in dir and returns it
// with its SHA-256 fingerprint (shown to the user so they can verify it).
func EnsureCert(dir string, hostnames []string) (tls.Certificate, string, error) {
	certPath := filepath.Join(dir, "tls-cert.pem")
	keyPath := filepath.Join(dir, "tls-key.pem")
	if c, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		leaf, perr := x509.ParseCertificate(c.Certificate[0])
		if perr == nil && time.Now().Add(30*24*time.Hour).Before(leaf.NotAfter) {
			return c, fingerprint(c.Certificate[0]), nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		// unreadable or corrupt: generate a new pair below
		_ = err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: hostnames[0], Organization: []string{"Vahti (local)"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(825 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              append([]string{}, hostnames...),
	}
	if h, err := os.Hostname(); err == nil && h != "" && !strings.Contains(h, " ") {
		tpl.DNSNames = append(tpl.DNSNames, h)
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLinkLocalUnicast() {
				tpl.IPAddresses = append(tpl.IPAddresses, ipn.IP)
			}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	if err := store.WriteFileAtomic(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600); err != nil {
		return tls.Certificate{}, "", err
	}
	if err := store.WriteFileAtomic(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return tls.Certificate{}, "", err
	}
	c, err := tls.LoadX509KeyPair(certPath, keyPath)
	return c, fingerprint(der), err
}

// fingerprint returns the certificate's SHA-256 fingerprint as AB:CD:... so the admin can
// compare it with what the browser shows.
func fingerprint(der []byte) string {
	s := sha256.Sum256(der)
	h := strings.ToUpper(hex.EncodeToString(s[:]))
	var b strings.Builder
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(h[i : i+2])
	}
	return b.String()
}
