package httpapi

import (
	"strings"
	"testing"
)

func FuzzCanonicalCommand(f *testing.F) {
	for _, seed := range []string{"enqueue", "/enqueue", "/queue/reorder", " player.resync ", "", "/"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got := canonical(input)
		if strings.HasPrefix(strings.TrimSpace(input), "/") && strings.Contains(got, "/") {
			t.Fatalf("canonical(%q) retained slash: %q", input, got)
		}
		if got != strings.TrimSpace(got) {
			t.Fatalf("canonical(%q) retained surrounding whitespace: %q", input, got)
		}
	})
}
