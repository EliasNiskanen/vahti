package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAreValid(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, u := range c.DNS.Upstreams {
		up, _ := ParseUpstream(u)
		if up.Net != "tcp-tls" {
			t.Errorf("default upstream %s should be encrypted", u)
		}
	}
}

func TestRejectsUnsafeConfig(t *testing.T) {
	bad := []func(*Config){
		func(c *Config) { c.DNS.Upstreams = []string{"tls://9.9.9.9:853"} },        // no server name
		func(c *Config) { c.DNS.Upstreams = []string{"udp://dns.example.com:53"} }, // hostname
		func(c *Config) { c.Lists[0].URL = "http://example.com/list" },             // not https
		func(c *Config) { c.Lists[0].Name = "../../etc/passwd" },                   // path
		func(c *Config) { c.DNS.AllowedClients = []string{"everything"} },
	}
	for i, f := range bad {
		c := Default()
		f(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(`{"dataDir":"/tmp/x","dsn":{}}`), 0o600)
	if _, _, err := Load(p); err == nil {
		t.Fatal("typo in config should be reported")
	}
	if _, found, err := Load(filepath.Join(t.TempDir(), "missing.json")); err != nil || found {
		t.Fatal("missing file should give defaults")
	}
}

func TestExampleConfigMatchesDefaults(t *testing.T) {
	c, found, err := Load("../../deploy/config.json")
	if err != nil || !found {
		t.Fatalf("deploy/config.json: found=%v err=%v", found, err)
	}
	d := Default()
	if c.DataDir != d.DataDir || len(c.DNS.Upstreams) != len(d.DNS.Upstreams) || c.Web.Listen != d.Web.Listen || !c.DNS.RebindProtection {
		t.Fatal("example config drifted from the defaults")
	}
}
