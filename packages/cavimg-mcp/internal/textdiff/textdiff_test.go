package textdiff

import (
	"strings"
	"testing"
)

func TestUnifiedIdenticalIsEmpty(t *testing.T) {
	if got := Unified("f.txt", "a\nb\n", "a\nb\n"); got != "" {
		t.Fatalf("want empty diff, got %q", got)
	}
}

func TestUnifiedShowsChange(t *testing.T) {
	got := Unified("f.txt", "a\nb\nc\n", "a\nB\nc\n")
	for _, want := range []string{"--- a/f.txt", "+++ b/f.txt", "@@ ", "\n-b", "\n+B"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff missing %q; full diff:\n%s", want, got)
		}
	}
}
