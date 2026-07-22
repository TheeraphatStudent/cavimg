// Package workspace confines every filesystem path a tool touches to a single
// mounted workspace root, rejecting escapes via "..", absolute paths, or symlinks.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvRoot is the environment variable naming the workspace root.
const EnvRoot = "CAVIMG_WORKSPACE_ROOT"

const defaultRoot = "/workspace"

// Root returns the resolved (symlink-free) workspace root.
func Root() (string, error) {
	raw := os.Getenv(EnvRoot)
	if raw == "" {
		raw = defaultRoot
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("workspace root %q: %w", abs, err)
	}
	return resolved, nil
}

// Confine resolves candidate (relative candidates are joined to root) and
// guarantees the real path stays within root. It returns the resolved path.
func Confine(root, candidate string) (string, error) {
	if candidate == "" {
		return "", errors.New("empty path")
	}
	abs := candidate
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, candidate)
	}
	abs = filepath.Clean(abs)

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path %q: %w", candidate, err)
	}

	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", fmt.Errorf("path %q is outside the workspace", candidate)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the workspace root", candidate)
	}
	return resolved, nil
}
