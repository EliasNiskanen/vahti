package intel

import (
	"strings"
	"testing"
)

// FuzzParseList feeds random text to the list parser and checks that it never
// crashes and never returns an invalid name.
func FuzzParseList(f *testing.F) {
	f.Add("0.0.0.0 ads.example.com\n||x.example.org^\nplain.example\n# c\n")
	f.Fuzz(func(t *testing.T, s string) {
		out, err := ParseList(strings.NewReader(s), 10000)
		if err != nil {
			return
		}
		for _, d := range out {
			if !ValidName(d) {
				t.Fatalf("invalid name returned: %q", d)
			}
		}
	})
}
