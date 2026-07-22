package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDetectViteReact(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"package.json":  `{"dependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
		"tsconfig.json": "{}",
		"src/main.tsx":  "",
	})
	r, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Framework != "Vite+React" {
		t.Errorf("framework = %q, want Vite+React", r.Framework)
	}
	if r.PackageManager != "npm" {
		t.Errorf("pm = %q, want npm (default)", r.PackageManager)
	}
	if !r.TypeScript {
		t.Error("expected TypeScript = true")
	}
}

func TestDetectNextWithPnpm(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"package.json":   `{"dependencies":{"next":"^14"}}`,
		"pnpm-lock.yaml": "",
	})
	r, _ := Run(dir)
	if r.Framework != "Next.js" {
		t.Errorf("framework = %q, want Next.js", r.Framework)
	}
	if r.PackageManager != "pnpm" {
		t.Errorf("pm = %q, want pnpm", r.PackageManager)
	}
}

func TestDetectAngular(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"package.json": `{"dependencies":{"@angular/core":"^18"}}`,
	})
	r, _ := Run(dir)
	if r.Framework != "Angular" {
		t.Errorf("framework = %q, want Angular", r.Framework)
	}
}

func TestDetectPlainHTML(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"index.html": "<img src='a.png'>",
	})
	r, _ := Run(dir)
	if r.Framework != "Plain HTML" {
		t.Errorf("framework = %q, want Plain HTML", r.Framework)
	}
}
