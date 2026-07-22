package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// resolvedTempDir returns a symlink-resolved temp dir so containment checks are stable
// across platforms (macOS /var→/private/var, Windows 8.3 names, etc.).
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return d
}

func TestConfineAllowsPathInsideRoot(t *testing.T) {
	root := resolvedTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "proj", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Confine(root, "proj/src")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "proj", "src")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestConfineRejectsDotDotEscape(t *testing.T) {
	root := resolvedTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Confine(root, "proj/../.."); err == nil {
		t.Fatal("expected error for .. escape, got nil")
	}
}

func TestConfineRejectsAbsoluteOutside(t *testing.T) {
	root := resolvedTempDir(t)
	outside := resolvedTempDir(t) // a different temp dir
	if _, err := Confine(root, outside); err == nil {
		t.Fatal("expected error for absolute-outside path, got nil")
	}
}

func TestConfineRejectsSymlinkEscape(t *testing.T) {
	root := resolvedTempDir(t)
	outside := resolvedTempDir(t)
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}
	if _, err := Confine(root, "escape"); err == nil {
		t.Fatal("expected error for symlink escape, got nil")
	}
}
