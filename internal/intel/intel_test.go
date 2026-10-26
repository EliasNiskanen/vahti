package intel

import (
	"strings"
	"testing"
)

func TestParseListFormats(t *testing.T) {
	in := `# comment
0.0.0.0 ads.example.com
127.0.0.1 track.example.net # trailing
0.0.0.0 localhost
||adblock.example.org^
||path.example.org^$third-party
plain.example.io
10.0.0.1 not-a-block.example.com
not a valid line here
0.0.0.0 bad_label-.example.com
ads.example.com
`
	got, err := ParseList(strings.NewReader(in), 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ads.example.com", "track.example.net", "adblock.example.org", "plain.example.io"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := ParseList(strings.NewReader("a.com\nb.com\nc.com\n"), 2); err == nil {
		t.Fatal("size limit not enforced")
	}
}

func TestLookupMostSpecific(t *testing.T) {
	db := New()
	db.Add("example.com", Entry{Category: Ads})
	db.Add("safe.example.com", Entry{Category: Telemetry})
	if m := db.Lookup("A.Safe.Example.com."); m.Category != Telemetry || m.Domain != "safe.example.com" {
		t.Fatalf("got %+v", m)
	}
	if m := db.Lookup("other.example.com"); m.Category != Ads {
		t.Fatalf("got %+v", m)
	}
	if m := db.Lookup("com"); m.Found() {
		t.Fatal("a bare TLD must never match")
	}
	if m := db.Lookup("www.facebook.com"); m.Found() || m.Owner != "Meta" {
		t.Fatalf("owner should be known without a category: %+v", m)
	}
}

func TestRegistrable(t *testing.T) {
	cases := map[string]string{
		"a.b.example.co.uk": "example.co.uk",
		"api.vendor.fi":     "vendor.fi",
		"x.y.example":       "y.example",
	}
	for in, want := range cases {
		if got := Registrable(in); got != want {
			t.Errorf("Registrable(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeedIsValid(t *testing.T) {
	db := New()
	if l := db.Lists(); len(l) != 1 || l[0].Entries < 40 {
		t.Fatalf("seed list too small: %+v", l)
	}
}
