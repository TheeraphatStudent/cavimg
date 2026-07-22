# cavimg-mcp Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Status: IMPLEMENTED (2026-07-23).** All nine tasks are complete and committed on
> `feature/mcp-tools`. Two deviations were discovered during execution and shipped —
> see **Execution deviations** at the end of this file. Acceptance criteria #1, #3,
> and #4 were verified live against a real podman container; #2 is Codex-UI dependent
> and documented in the README.

**Goal:** Build a stdio MCP server (`cavimg-mcp`) exposing four tools — `detect_project`, `install_cavimg`, `list_image_usages`, `apply_cavimg` — that let an AI agent adopt the `cavimg` npm package into a frontend project, then containerize and verify it.

**Architecture:** A thin `main.go` builds an `mcp.Server`, registers four typed tool handlers (`internal/tools`), and runs over stdio. Each handler validates its `project_path` through a shared path-confinement helper (`internal/workspace`), then delegates to a focused pure package: `internal/detect` (stack detection), `internal/scan` (image-usage grep), `internal/rewrite` (`<img>`→`<cav-img>` transform + wiring guidance), and `internal/textdiff` (unified diff). The typed `Out` struct becomes the structured JSON result; `CallToolResult.Content` carries a one-line human summary.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk` **v1.6.1** (already in `go.mod`/`go.sum`), Podman (multi-stage `golang:alpine` → `node:22-alpine`), Make.

## Global Constraints

- **Module path:** `cavimg-mcp` (per existing `go.mod`); internal imports are `cavimg-mcp/internal/...`.
- **Go toolchain:** `go 1.26.3` (already pinned in `go.mod`).
- **SDK API (v1.6.1, verified against module cache — do NOT follow older web-guide signatures):**
  - `server := mcp.NewServer(&mcp.Implementation{Name, Version}, nil)`
  - `mcp.AddTool(server, &mcp.Tool{Name, Description}, handler)` — input+output JSON Schema is auto-inferred from the `In`/`Out` struct via `json` and `jsonschema` struct tags; input is auto-validated before the handler runs.
  - Handler signature: `func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error)`.
  - Returning a non-nil `error` is packed into `CallToolResult.Content` with `IsError` set (a **tool** error the agent sees) — use this for rejected paths.
  - Set `CallToolResult.Content = []mcp.Content{&mcp.TextContent{Text: summary}}` for the human summary; the returned `Out` becomes the structured JSON.
  - Stdio: `server.Run(context.Background(), &mcp.StdioTransport{})`.
- **Security:** every `project_path` (and every target file) is confined under `CAVIMG_WORKSPACE_ROOT` (default `/workspace`); `..` and symlink escapes are rejected with no I/O.
- **Safety default:** `install_cavimg` and `apply_cavimg` default `dry_run` to **true**; never mutate unless `dry_run:false` is explicitly passed.
- **Scope:** `apply_cavimg` rewrites only plain `<img>` tags (never `next/image` `<Image>`, never image imports); registration wiring is returned as **guidance text**, never injected into user code.
- **Runtime image:** `node:22-alpine` (NOT distroless — the server shells out to npm/pnpm/yarn), non-root user `app` (uid 10001).
- **No network** except the package-manager registry (used only by a real `install_cavimg`).
- **Commits:** commit message must NOT include a `Co-Authored-By` trailer.

---

### Task 1: Workspace path confinement (`internal/workspace`)

**Files:**
- Create: `internal/workspace/confine.go`
- Test: `internal/workspace/confine_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `const EnvRoot = "CAVIMG_WORKSPACE_ROOT"`
  - `func Root() (string, error)` — resolved (symlink-free) workspace root from `EnvRoot` (default `/workspace`).
  - `func Confine(root, candidate string) (string, error)` — returns the resolved absolute path of `candidate` (relative paths are joined to `root`), or an error if it escapes `root`.

- [ ] **Step 1: Write the failing test**

Create `internal/workspace/confine_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/workspace/`
Expected: FAIL — `undefined: Confine`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/workspace/confine.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/workspace/`
Expected: PASS (the symlink test may print `SKIP` on restricted platforms — acceptable).

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/
git commit -m "feat(mcp): add workspace path-confinement helper"
```

---

### Task 2: Project detection (`internal/detect`)

**Files:**
- Create: `internal/detect/detect.go`
- Test: `internal/detect/detect_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Result struct { PackageManager string; Framework string; TypeScript bool; SourceDirs []string; Evidence map[string]string }`
  - `func Run(dir string) (Result, error)` — detects package manager, framework, TypeScript, and source dirs for the project rooted at `dir`.

- [ ] **Step 1: Write the failing test**

Create `internal/detect/detect_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/detect/`
Expected: FAIL — `undefined: Run`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/detect/detect.go`:

```go
// Package detect inspects a frontend project directory and reports its package
// manager, framework, TypeScript usage, and source directories.
package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Result is the outcome of detecting a project's stack.
type Result struct {
	PackageManager string            `json:"package_manager"`
	Framework      string            `json:"framework"`
	TypeScript     bool              `json:"typescript"`
	SourceDirs     []string          `json:"source_dirs"`
	Evidence       map[string]string `json:"evidence"`
}

// Run detects the stack of the project rooted at dir.
func Run(dir string) (Result, error) {
	r := Result{SourceDirs: []string{}, Evidence: map[string]string{}}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		PackageManager  string            `json:"packageManager"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		_ = json.Unmarshal(data, &pkg)
	}
	dep := func(name string) bool {
		_, a := pkg.Dependencies[name]
		_, b := pkg.DevDependencies[name]
		return a || b
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(dir, rel))
		return err == nil
	}

	// Package manager: lockfile > packageManager field > default npm.
	switch {
	case exists("pnpm-lock.yaml"):
		r.PackageManager, r.Evidence["package_manager"] = "pnpm", "pnpm-lock.yaml"
	case exists("yarn.lock"):
		r.PackageManager, r.Evidence["package_manager"] = "yarn", "yarn.lock"
	case exists("bun.lockb") || exists("bun.lock"):
		r.PackageManager, r.Evidence["package_manager"] = "bun", "bun lockfile"
	case exists("package-lock.json"):
		r.PackageManager, r.Evidence["package_manager"] = "npm", "package-lock.json"
	case pkg.PackageManager != "":
		r.PackageManager = strings.SplitN(pkg.PackageManager, "@", 2)[0]
		r.Evidence["package_manager"] = "packageManager field"
	default:
		r.PackageManager, r.Evidence["package_manager"] = "npm", "default"
	}

	// Framework: order matters (most specific first).
	switch {
	case dep("next") || exists("next.config.js") || exists("next.config.ts") || exists("next.config.mjs"):
		r.Framework, r.Evidence["framework"] = "Next.js", "next"
	case dep("nuxt"):
		r.Framework, r.Evidence["framework"] = "Nuxt", "nuxt"
	case dep("@sveltejs/kit"):
		r.Framework, r.Evidence["framework"] = "SvelteKit", "@sveltejs/kit"
	case dep("svelte"):
		r.Framework, r.Evidence["framework"] = "Svelte", "svelte"
	case dep("@angular/core"):
		r.Framework, r.Evidence["framework"] = "Angular", "@angular/core"
	case dep("vite") && dep("react"):
		r.Framework, r.Evidence["framework"] = "Vite+React", "vite+react"
	case dep("vite"):
		r.Framework, r.Evidence["framework"] = "Vite", "vite"
	case dep("vue"):
		r.Framework, r.Evidence["framework"] = "Vue", "vue"
	case exists("index.html"):
		r.Framework, r.Evidence["framework"] = "Plain HTML", "index.html"
	default:
		r.Framework, r.Evidence["framework"] = "Unknown", "none"
	}

	// TypeScript.
	switch {
	case exists("tsconfig.json"):
		r.TypeScript, r.Evidence["typescript"] = true, "tsconfig.json"
	case dep("typescript"):
		r.TypeScript, r.Evidence["typescript"] = true, "typescript dependency"
	case hasTSFile(dir):
		r.TypeScript, r.Evidence["typescript"] = true, ".ts/.tsx file"
	}

	// Source directories.
	for _, d := range []string{"src", "app", "pages", "components"} {
		if exists(d) {
			r.SourceDirs = append(r.SourceDirs, d)
		}
	}
	if len(r.SourceDirs) == 0 && r.Framework == "Plain HTML" {
		r.SourceDirs = append(r.SourceDirs, ".")
	}

	return r, nil
}

// hasTSFile walks dir (skipping node_modules) looking for a .ts/.tsx file.
func hasTSFile(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ts" || ext == ".tsx" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/detect/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/detect/
git commit -m "feat(mcp): add project stack detection"
```

---

### Task 3: Image-usage scanner (`internal/scan`)

**Files:**
- Create: `internal/scan/scan.go`
- Test: `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Hit struct { File string; Line int; Kind string; Match string }` — `File` is a forward-slash path relative to the scanned dir; `Kind` is `"img-tag"` or `"image-import"`.
  - `func Run(dir, glob string) ([]Hit, error)` — walks `dir` for image usages; `glob` (a `filepath.Match` pattern applied to basenames) overrides the default extension set when non-empty. Results are sorted by file then line.

- [ ] **Step 1: Write the failing test**

Create `internal/scan/scan_test.go`:

```go
package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, files map[string]string) string {
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

func TestScanFindsImgTagsAndImports(t *testing.T) {
	dir := write(t, map[string]string{
		"index.html":       "<h1>x</h1>\n<img src=\"a.png\" alt=\"a\">\n",
		"src/App.tsx":      "import hero from './hero.png';\nexport const A = () => <img src={hero} />;\n",
		"node_modules/x.js": "<img src='ignore'>",
	})
	hits, err := Run(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	var tags, imports int
	for _, h := range hits {
		if h.Kind == "img-tag" {
			tags++
		}
		if h.Kind == "image-import" {
			imports++
		}
		if h.File == "node_modules/x.js" {
			t.Errorf("node_modules must be skipped, got hit in %s", h.File)
		}
	}
	if tags != 2 {
		t.Errorf("img-tag hits = %d, want 2", tags)
	}
	if imports != 1 {
		t.Errorf("image-import hits = %d, want 1", imports)
	}
}

func TestScanReportsLineAndRelPath(t *testing.T) {
	dir := write(t, map[string]string{
		"page.html": "line1\nline2\n<img src=\"x.png\">\n",
	})
	hits, _ := Run(dir, "")
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if hits[0].File != "page.html" {
		t.Errorf("file = %q, want page.html", hits[0].File)
	}
	if hits[0].Line != 3 {
		t.Errorf("line = %d, want 3", hits[0].Line)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scan/`
Expected: FAIL — `undefined: Run`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/scan/scan.go`:

```go
// Package scan finds image usages (<img> tags and image imports) in a project.
package scan

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Hit is a single image usage. File is relative to the scanned dir (forward slashes).
type Hit struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Kind  string `json:"kind"`  // "img-tag" | "image-import"
	Match string `json:"match"`
}

var (
	imgTagRe    = regexp.MustCompile(`(?i)<img\b`)
	imgImportRe = regexp.MustCompile(`(?i)\bimport\b[^\n]*['"][^'"\n]*\.(?:png|jpe?g|gif|webp|avif|svg)['"]`)
	defaultExts = map[string]bool{
		".html": true, ".htm": true, ".jsx": true, ".tsx": true,
		".vue": true, ".svelte": true, ".astro": true,
	}
	skipDirs = map[string]bool{
		"node_modules": true, "dist": true, ".next": true, "build": true, ".git": true,
	}
)

// Run scans dir for image usages. A non-empty glob (filepath.Match on the base
// name) overrides the default extension filter.
func Run(dir, glob string) ([]Hit, error) {
	var hits []Hit
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !selected(d.Name(), glob) {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		fileHits, scanErr := scanFile(path, filepath.ToSlash(rel))
		if scanErr != nil {
			return nil
		}
		hits = append(hits, fileHits...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		return hits[i].Line < hits[j].Line
	})
	return hits, nil
}

func selected(base, glob string) bool {
	if glob != "" {
		ok, _ := filepath.Match(glob, base)
		return ok
	}
	return defaultExts[strings.ToLower(filepath.Ext(base))]
}

func scanFile(path, rel string) ([]Hit, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hits []Hit
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if loc := imgTagRe.FindStringIndex(text); loc != nil {
			hits = append(hits, Hit{File: rel, Line: line, Kind: "img-tag", Match: trim(text)})
			continue
		}
		if imgImportRe.MatchString(text) {
			hits = append(hits, Hit{File: rel, Line: line, Kind: "image-import", Match: trim(text)})
		}
	}
	return hits, sc.Err()
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scan/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/
git commit -m "feat(mcp): add image-usage scanner"
```

---

### Task 4: Unified diff generator (`internal/textdiff`)

**Files:**
- Create: `internal/textdiff/textdiff.go`
- Test: `internal/textdiff/textdiff_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func Unified(path, oldText, newText string) string` — returns a single-hunk unified diff (3 lines of context) with `--- a/<path>` / `+++ b/<path>` headers, or `""` if the texts are identical.

- [ ] **Step 1: Write the failing test**

Create `internal/textdiff/textdiff_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/textdiff/`
Expected: FAIL — `undefined: Unified`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/textdiff/textdiff.go`:

```go
// Package textdiff produces a minimal single-hunk unified diff between two texts.
package textdiff

import (
	"fmt"
	"strings"
)

const context = 3

// Unified returns a unified diff between oldText and newText for the given path,
// or "" if they are identical.
func Unified(path, oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	a := splitLines(oldText)
	b := splitLines(newText)
	ops := diffLines(a, b)

	first, last := -1, -1
	for i, op := range ops {
		if op.tag != ' ' {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	if first == -1 {
		return "" // texts differ only by trailing newline; treat as no change
	}

	start := first - context
	if start < 0 {
		start = 0
	}
	end := last + context
	if end > len(ops)-1 {
		end = len(ops) - 1
	}

	// 1-based start lines = number of preceding lines present in each side + 1.
	aBefore, bBefore := 0, 0
	for _, op := range ops[:start] {
		if op.tag == ' ' || op.tag == '-' {
			aBefore++
		}
		if op.tag == ' ' || op.tag == '+' {
			bBefore++
		}
	}
	aCount, bCount := 0, 0
	var body strings.Builder
	for _, op := range ops[start : end+1] {
		switch op.tag {
		case ' ':
			aCount++
			bCount++
			body.WriteString(" " + op.text + "\n")
		case '-':
			aCount++
			body.WriteString("-" + op.text + "\n")
		case '+':
			bCount++
			body.WriteString("+" + op.text + "\n")
		}
	}
	aStart := aBefore + 1
	if aCount == 0 {
		aStart = aBefore
	}
	bStart := bBefore + 1
	if bCount == 0 {
		bStart = bBefore
	}

	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n", path)
	fmt.Fprintf(&out, "+++ b/%s\n", path)
	fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
	out.WriteString(body.String())
	return out.String()
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

type lineOp struct {
	tag  byte // ' ', '-', '+'
	text string
}

// diffLines computes a line-level diff via a longest-common-subsequence table.
func diffLines(a, b []string) []lineOp {
	n, m := len(a), len(b)
	c := make([][]int, n+1)
	for i := range c {
		c[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				c[i][j] = c[i-1][j-1] + 1
			} else if c[i-1][j] >= c[i][j-1] {
				c[i][j] = c[i-1][j]
			} else {
				c[i][j] = c[i][j-1]
			}
		}
	}
	var out []lineOp
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			out = append(out, lineOp{' ', a[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || c[i][j-1] >= c[i-1][j]):
			out = append(out, lineOp{'+', b[j-1]})
			j--
		default:
			out = append(out, lineOp{'-', a[i-1]})
			i--
		}
	}
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/textdiff/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/textdiff/
git commit -m "feat(mcp): add unified diff generator"
```

---

### Task 5: Image→cav-img rewrite + wiring guidance (`internal/rewrite`)

**Files:**
- Create: `internal/rewrite/rewrite.go`
- Test: `internal/rewrite/rewrite_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func Transform(filename, content string) (string, bool)` — rewrites `<img …>`→`<cav-img …>` preserving attributes; `.jsx`/`.tsx` emit self-closing `<cav-img … />`, other files emit `<cav-img …></cav-img>`. Returns the new content and whether anything changed. Idempotent.
  - `func Wiring(framework string) ([]string, bool)` — registration guidance steps for a framework, and whether wiring is `manual`.

- [ ] **Step 1: Write the failing test**

Create `internal/rewrite/rewrite_test.go`:

```go
package rewrite

import (
	"strings"
	"testing"
)

func TestTransformHTMLImg(t *testing.T) {
	out, changed := Transform("index.html", `<img src="a.png" alt="a">`)
	if !changed {
		t.Fatal("expected changed = true")
	}
	if out != `<cav-img src="a.png" alt="a"></cav-img>` {
		t.Fatalf("got %q", out)
	}
}

func TestTransformJSXSelfClosing(t *testing.T) {
	out, _ := Transform("App.tsx", `<img src={hero} alt="h" />`)
	if out != `<cav-img src={hero} alt="h" />` {
		t.Fatalf("got %q", out)
	}
}

func TestTransformIsIdempotent(t *testing.T) {
	src := `<img src="a.png" alt="a">`
	once, _ := Transform("index.html", src)
	twice, changed := Transform("index.html", once)
	if changed {
		t.Error("second transform should report no change")
	}
	if twice != once {
		t.Errorf("not idempotent: %q != %q", twice, once)
	}
}

func TestTransformIgnoresSvgImageAndCavImg(t *testing.T) {
	src := `<image href="a.png"/><cav-img src="b.png"></cav-img>`
	out, changed := Transform("index.html", src)
	if changed || out != src {
		t.Errorf("should not touch <image> or <cav-img>: %q", out)
	}
}

func TestWiring(t *testing.T) {
	if _, manual := Wiring("Vite+React"); manual {
		t.Error("Vite+React should not be manual")
	}
	steps, manual := Wiring("Vue")
	if !manual {
		t.Error("Vue should be manual")
	}
	if len(steps) == 0 {
		t.Error("expected guidance steps even when manual")
	}
	nextSteps, _ := Wiring("Next.js")
	if !strings.Contains(strings.Join(nextSteps, "\n"), "useEffect") {
		t.Error("Next.js guidance should mention useEffect")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/rewrite/`
Expected: FAIL — `undefined: Transform`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/rewrite/rewrite.go`:

```go
// Package rewrite converts plain <img> tags to the cavimg <cav-img> web component
// and produces framework-specific registration guidance.
package rewrite

import (
	"path/filepath"
	"regexp"
	"strings"
)

// imgTag matches a single <img …> tag (with or without a trailing slash),
// capturing its attributes. It never matches <cav-img> or <image>.
var imgTag = regexp.MustCompile(`(?is)<img\b([^>]*?)\s*/?>`)

// Transform rewrites every <img> in content to <cav-img>, preserving attributes.
// It returns the new content and whether anything changed. Running it again on
// its own output produces no further change (idempotent).
func Transform(filename, content string) (string, bool) {
	jsx := isJSX(filename)
	changed := false
	out := imgTag.ReplaceAllStringFunc(content, func(m string) string {
		changed = true
		sub := imgTag.FindStringSubmatch(m)
		attrs := strings.TrimSpace(sub[1])
		return buildTag(attrs, jsx)
	})
	return out, changed
}

func isJSX(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jsx" || ext == ".tsx"
}

func buildTag(attrs string, jsx bool) string {
	if jsx {
		if attrs == "" {
			return "<cav-img />"
		}
		return "<cav-img " + attrs + " />"
	}
	if attrs == "" {
		return "<cav-img></cav-img>"
	}
	return "<cav-img " + attrs + "></cav-img>"
}

// Wiring returns registration-guidance steps for a framework and whether the
// wiring must be done manually (true for frameworks without a fully specified path).
func Wiring(framework string) ([]string, bool) {
	switch framework {
	case "Plain HTML":
		return []string{
			"Register <cav-img> with a module script in your HTML:",
			`<script type="module">import 'cavimg'</script>`,
			`Or via CDN: <script src="https://cdn.jsdelivr.net/npm/cavimg"></script>`,
		}, false
	case "Vite", "Vite+React":
		return []string{
			"Add a side-effect import in your entry module (e.g. src/main.ts or src/main.tsx):",
			"import 'cavimg';",
			"This auto-registers <cav-img>; no useEffect is needed for a client-only SPA.",
		}, false
	case "Next.js":
		return []string{
			"Web Components are client-only. In a 'use client' component, register on mount:",
			"useEffect(() => { import('cavimg').then(m => m.defineCavImg()); }, []);",
		}, false
	case "Angular":
		return []string{
			"Add CUSTOM_ELEMENTS_SCHEMA to the component and register the element:",
			"import { CUSTOM_ELEMENTS_SCHEMA } from '@angular/core';",
			"import { defineCavImg } from 'cavimg';",
			"// @Component({ ..., schemas: [CUSTOM_ELEMENTS_SCHEMA] })",
			"// in the constructor: defineCavImg();",
		}, false
	default:
		return []string{
			"Manual wiring required for " + framework +
				". See https://github.com/TheeraphatStudent/cavimg for framework guidance.",
		}, true
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/rewrite/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rewrite/
git commit -m "feat(mcp): add img-to-cav-img rewrite and wiring guidance"
```

---

### Task 6: MCP tool handlers + server entry (`internal/tools`, `main.go`)

**Files:**
- Create: `internal/tools/tools.go`
- Create: `main.go`
- Test: `internal/tools/tools_test.go`

**Interfaces:**
- Consumes: `workspace.Root`, `workspace.Confine`, `detect.Run`, `scan.Run`, `scan.Hit`, `rewrite.Transform`, `rewrite.Wiring`, `textdiff.Unified`; SDK `mcp.NewServer`, `mcp.AddTool`, `mcp.Tool`, `mcp.CallToolResult`, `mcp.TextContent`, `mcp.StdioTransport`.
- Produces:
  - `func Register(s *mcp.Server)` — registers all four tools on the server.
  - Handlers (exported for unit testing): `DetectHandler`, `InstallHandler`, `ListHandler`, `ApplyHandler`, each with signature `func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)`.

- [ ] **Step 1: Write the failing test**

Create `internal/tools/tools_test.go`:

```go
package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CAVIMG_WORKSPACE_ROOT", root)
	return root
}

func TestDetectHandler(t *testing.T) {
	setupWorkspace(t, map[string]string{
		"proj/package.json":  `{"dependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
		"proj/tsconfig.json": "{}",
		"proj/src/main.tsx":  "",
	})
	_, out, err := DetectHandler(context.Background(), nil, DetectInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Framework != "Vite+React" {
		t.Errorf("framework = %q, want Vite+React", out.Framework)
	}
}

func TestDetectHandlerRejectsEscape(t *testing.T) {
	setupWorkspace(t, map[string]string{"proj/package.json": "{}"})
	if _, _, err := DetectHandler(context.Background(), nil, DetectInput{ProjectPath: "proj/../.."}); err == nil {
		t.Fatal("expected error for escaping path")
	}
}

func TestInstallHandlerDryRunDoesNotExecute(t *testing.T) {
	setupWorkspace(t, map[string]string{
		"proj/package.json":   `{"devDependencies":{"vite":"^5"}}`,
		"proj/pnpm-lock.yaml": "",
	})
	_, out, err := InstallHandler(context.Background(), nil, InstallInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Command != "pnpm add cavimg" {
		t.Errorf("command = %q, want 'pnpm add cavimg'", out.Command)
	}
	if out.Executed {
		t.Error("dry-run must not execute")
	}
	if out.ExitCode != nil {
		t.Error("dry-run exit code must be nil")
	}
}

func TestApplyDryRunReturnsDiffWithoutWriting(t *testing.T) {
	root := setupWorkspace(t, map[string]string{
		"proj/package.json": `{"dependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
		"proj/src/App.tsx":  "export default () => <img src=\"/a.png\" alt=\"a\" />;\n",
	})
	before, _ := os.ReadFile(filepath.Join(root, "proj", "src", "App.tsx"))
	_, out, err := ApplyHandler(context.Background(), nil, ApplyInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Diff == "" {
		t.Error("expected a non-empty diff on dry-run")
	}
	after, _ := os.ReadFile(filepath.Join(root, "proj", "src", "App.tsx"))
	if string(before) != string(after) {
		t.Error("dry-run must not modify files")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	setupWorkspace(t, map[string]string{
		"proj/package.json": `{"dependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
		"proj/src/App.tsx":  "export default () => <img src=\"/a.png\" alt=\"a\" />;\n",
	})
	no := false
	_, out1, err := ApplyHandler(context.Background(), nil, ApplyInput{ProjectPath: "proj", DryRun: &no})
	if err != nil {
		t.Fatal(err)
	}
	if len(out1.ChangedFiles) == 0 {
		t.Fatal("first apply should change files")
	}
	_, out2, err := ApplyHandler(context.Background(), nil, ApplyInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out2.Diff != "" || len(out2.ChangedFiles) != 0 {
		t.Fatalf("second apply should be empty; diff=%q files=%v", out2.Diff, out2.ChangedFiles)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/`
Expected: FAIL — `undefined: DetectHandler` (etc).

- [ ] **Step 3: Write minimal implementation**

Create `internal/tools/tools.go`:

```go
// Package tools implements the four cavimg-mcp tool handlers and registers them
// on an MCP server. Every handler confines its project_path to the workspace root.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"cavimg-mcp/internal/detect"
	"cavimg-mcp/internal/rewrite"
	"cavimg-mcp/internal/scan"
	"cavimg-mcp/internal/textdiff"
	"cavimg-mcp/internal/workspace"
)

const maxOutput = 8192

// ---- shared ----

func confine(projectPath string) (string, error) {
	root, err := workspace.Root()
	if err != nil {
		return "", fmt.Errorf("workspace root unavailable: %w", err)
	}
	p, err := workspace.Confine(root, projectPath)
	if err != nil {
		return "", fmt.Errorf("project_path rejected: %w", err)
	}
	return p, nil
}

func text(summary string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}
}

// ---- detect_project ----

type DetectInput struct {
	ProjectPath string `json:"project_path" jsonschema:"path to the project root, within the workspace"`
}

type DetectOutput struct {
	PackageManager string            `json:"package_manager"`
	Framework      string            `json:"framework"`
	TypeScript     bool              `json:"typescript"`
	SourceDirs     []string          `json:"source_dirs"`
	Evidence       map[string]string `json:"evidence"`
	Summary        string            `json:"summary"`
}

func DetectHandler(ctx context.Context, req *mcp.CallToolRequest, in DetectInput) (*mcp.CallToolResult, DetectOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, DetectOutput{}, err
	}
	r, err := detect.Run(dir)
	if err != nil {
		return nil, DetectOutput{}, err
	}
	out := DetectOutput{
		PackageManager: r.PackageManager,
		Framework:      r.Framework,
		TypeScript:     r.TypeScript,
		SourceDirs:     r.SourceDirs,
		Evidence:       r.Evidence,
	}
	ts := "no TypeScript"
	if out.TypeScript {
		ts = "TypeScript"
	}
	out.Summary = fmt.Sprintf("%s project using %s (%s); source dirs: %s",
		out.Framework, out.PackageManager, ts, strings.Join(out.SourceDirs, ", "))
	return text(out.Summary), out, nil
}

// ---- install_cavimg ----

type InstallInput struct {
	ProjectPath    string `json:"project_path" jsonschema:"path to the project root, within the workspace"`
	Version        string `json:"version,omitempty" jsonschema:"cavimg version (e.g. 1.0.1); defaults to latest"`
	PackageManager string `json:"package_manager,omitempty" jsonschema:"npm|pnpm|yarn|bun; auto-detected if omitted"`
	DryRun         *bool  `json:"dry_run,omitempty" jsonschema:"if true (default) report the command without running it"`
}

type InstallOutput struct {
	Command   string `json:"command"`
	Executed  bool   `json:"executed"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
	Summary   string `json:"summary"`
}

func InstallHandler(ctx context.Context, req *mcp.CallToolRequest, in InstallInput) (*mcp.CallToolResult, InstallOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, InstallOutput{}, err
	}
	pm := in.PackageManager
	if pm == "" {
		r, derr := detect.Run(dir)
		if derr != nil {
			return nil, InstallOutput{}, derr
		}
		pm = r.PackageManager
	}
	name, args := installCmd(pm, in.Version)
	cmdStr := name + " " + strings.Join(args, " ")

	dry := in.DryRun == nil || *in.DryRun
	if dry {
		out := InstallOutput{
			Command:  cmdStr,
			Executed: false,
			Summary:  "dry-run: would run `" + cmdStr + "` in " + in.ProjectPath,
		}
		return text(out.Summary), out, nil
	}

	if _, lookErr := exec.LookPath(name); lookErr != nil {
		out := InstallOutput{
			Command:  cmdStr,
			Executed: false,
			Stderr:   name + " is not available in this environment",
			Summary:  name + " not found; nothing was installed",
		}
		return text(out.Summary), out, nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	runErr := cmd.Run()

	code := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
			se.WriteString("\n" + runErr.Error())
		}
	}
	stdout, tr1 := truncate(so.String())
	stderr, tr2 := truncate(se.String())
	out := InstallOutput{
		Command:   cmdStr,
		Executed:  true,
		ExitCode:  &code,
		Stdout:    stdout,
		Stderr:    stderr,
		Truncated: tr1 || tr2,
		Summary:   fmt.Sprintf("ran `%s` (exit %d)", cmdStr, code),
	}
	return text(out.Summary), out, nil
}

func installCmd(pm, version string) (string, []string) {
	spec := "cavimg"
	if version != "" {
		spec = "cavimg@" + version
	}
	switch pm {
	case "pnpm":
		return "pnpm", []string{"add", spec}
	case "yarn":
		return "yarn", []string{"add", spec}
	case "bun":
		return "bun", []string{"add", spec}
	default:
		return "npm", []string{"install", spec}
	}
}

func truncate(s string) (string, bool) {
	if len(s) > maxOutput {
		return s[:maxOutput], true
	}
	return s, false
}

// ---- list_image_usages ----

type ListInput struct {
	ProjectPath string `json:"project_path" jsonschema:"path to the project root, within the workspace"`
	Glob        string `json:"glob,omitempty" jsonschema:"optional filename glob (filepath.Match on base names)"`
}

type ListOutput struct {
	Hits    []scan.Hit `json:"hits"`
	Count   int        `json:"count"`
	Summary string     `json:"summary"`
}

func ListHandler(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, ListOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, ListOutput{}, err
	}
	hits, err := scan.Run(dir, in.Glob)
	if err != nil {
		return nil, ListOutput{}, err
	}
	if hits == nil {
		hits = []scan.Hit{}
	}
	out := ListOutput{
		Hits:    hits,
		Count:   len(hits),
		Summary: fmt.Sprintf("found %d image usage(s)", len(hits)),
	}
	return text(out.Summary), out, nil
}

// ---- apply_cavimg ----

type ApplyInput struct {
	ProjectPath string   `json:"project_path" jsonschema:"path to the project root, within the workspace"`
	Files       []string `json:"files,omitempty" jsonschema:"specific files to rewrite; defaults to all detected"`
	DryRun      *bool    `json:"dry_run,omitempty" jsonschema:"if true (default) return a diff and change nothing on disk"`
}

type WiringInfo struct {
	Framework string   `json:"framework"`
	Steps     []string `json:"steps"`
	Manual    bool     `json:"manual"`
}

type ApplyOutput struct {
	DryRun       bool       `json:"dry_run"`
	Diff         string     `json:"diff"`
	ChangedFiles []string   `json:"changed_files"`
	Hunks        int        `json:"hunks"`
	Wiring       WiringInfo `json:"wiring"`
	Summary      string     `json:"summary"`
}

func ApplyHandler(ctx context.Context, req *mcp.CallToolRequest, in ApplyInput) (*mcp.CallToolResult, ApplyOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, ApplyOutput{}, err
	}
	det, err := detect.Run(dir)
	if err != nil {
		return nil, ApplyOutput{}, err
	}

	files := in.Files
	if len(files) == 0 {
		hits, serr := scan.Run(dir, "")
		if serr != nil {
			return nil, ApplyOutput{}, serr
		}
		seen := map[string]bool{}
		for _, h := range hits {
			if h.Kind == "img-tag" && !seen[h.File] {
				seen[h.File] = true
				files = append(files, h.File)
			}
		}
		sort.Strings(files)
	}

	dry := in.DryRun == nil || *in.DryRun
	var diffs []string
	changed := []string{}
	hunks := 0
	for _, f := range files {
		abs, cerr := workspace.Confine(dir, f)
		if cerr != nil {
			return nil, ApplyOutput{}, fmt.Errorf("file rejected: %w", cerr)
		}
		content, rerr := os.ReadFile(abs)
		if rerr != nil {
			continue
		}
		newContent, didChange := rewrite.Transform(abs, string(content))
		if !didChange {
			continue
		}
		rel, _ := filepath.Rel(dir, abs)
		relSlash := filepath.ToSlash(rel)
		d := textdiff.Unified(relSlash, string(content), newContent)
		if d == "" {
			continue
		}
		diffs = append(diffs, d)
		hunks += strings.Count(d, "@@ -")
		changed = append(changed, relSlash)
		if !dry {
			if werr := os.WriteFile(abs, []byte(newContent), 0o644); werr != nil {
				return nil, ApplyOutput{}, werr
			}
		}
	}

	steps, manual := rewrite.Wiring(det.Framework)
	verb := "would rewrite"
	if !dry {
		verb = "rewrote"
	}
	out := ApplyOutput{
		DryRun:       dry,
		Diff:         strings.Join(diffs, "\n"),
		ChangedFiles: changed,
		Hunks:        hunks,
		Wiring:       WiringInfo{Framework: det.Framework, Steps: steps, Manual: manual},
		Summary:      fmt.Sprintf("%s %d file(s) to <cav-img>; see wiring for %s registration", verb, len(changed), det.Framework),
	}
	return text(out.Summary), out, nil
}

// ---- registration ----

// Register adds all four cavimg tools to the server.
func Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "detect_project",
		Description: "Detect a frontend project's package manager, framework, TypeScript usage, and source dirs.",
	}, DetectHandler)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "install_cavimg",
		Description: "Install the cavimg npm package with the project's package manager. Defaults to dry_run.",
	}, InstallHandler)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_image_usages",
		Description: "List <img> tags and image imports in a project.",
	}, ListHandler)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "apply_cavimg",
		Description: "Rewrite <img> tags to cavimg's <cav-img>. Defaults to dry_run (returns a diff, changes nothing).",
	}, ApplyHandler)
}
```

Create `main.go`:

```go
// Command cavimg-mcp is a stdio MCP server that helps an AI agent adopt the
// cavimg npm package into a frontend project.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"cavimg-mcp/internal/tools"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cavimg-mcp",
		Version: "0.1.0",
	}, nil)

	tools.Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("cavimg-mcp: %v", err)
	}
}
```

- [ ] **Step 4: Run tests and build to verify they pass**

Run: `go test ./... && go build -o cavimg-mcp .`
Expected: all packages PASS; `cavimg-mcp` binary builds with no errors.

Note: `go.mod` currently marks the SDK modules `// indirect`. A default `-mod=readonly`
build tolerates stale `indirect` annotations, so this should just work. If the build
*does* complain about the module graph, run `go mod tidy` once (all deps are already
in the local cache — no network needed) and re-run; this is a one-time fixup, not a
plan change.

- [ ] **Step 5: Commit**

```bash
git add main.go internal/tools/
git commit -m "feat(mcp): add four tool handlers and stdio server entry"
```

---

### Task 7: Protocol smoke verification (`scripts/smoke.sh`, `scripts/smoke.ps1`)

**Files:**
- Create: `scripts/smoke.sh`
- Create: `scripts/smoke.ps1`

**Interfaces:**
- Consumes: the built server (via `go run .`).
- Produces: a pass/fail check that piping `initialize` + `notifications/initialized` + `tools/list` over stdin yields all four tool names on stdout. Verifies acceptance criterion #1 locally without a container.

- [ ] **Step 1: Write the smoke script (bash)**

Create `scripts/smoke.sh`:

```bash
#!/usr/bin/env bash
# Smoke-test the cavimg-mcp stdio protocol without a container.
# Pipes initialize + initialized + tools/list, asserts all four tools appear.
set -euo pipefail

cd "$(dirname "$0")/.."

req=$(cat <<'JSON'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
JSON
)

out=$(printf '%s\n' "$req" | go run .)

fail=0
for tool in detect_project install_cavimg list_image_usages apply_cavimg; do
  if ! grep -q "\"$tool\"" <<<"$out"; then
    echo "MISSING tool: $tool"
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  echo "SMOKE FAILED"
  echo "$out"
  exit 1
fi
echo "SMOKE OK: all four tools listed"
```

- [ ] **Step 2: Write the smoke script (PowerShell, for this Windows dev box)**

Create `scripts/smoke.ps1`:

```powershell
# Smoke-test the cavimg-mcp stdio protocol without a container.
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$req = @'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
'@

$out = $req | go run .

$fail = $false
foreach ($tool in "detect_project","install_cavimg","list_image_usages","apply_cavimg") {
  if ($out -notmatch [regex]::Escape("`"$tool`"")) {
    Write-Host "MISSING tool: $tool"
    $fail = $true
  }
}
if ($fail) {
  Write-Host "SMOKE FAILED"
  Write-Host $out
  exit 1
}
Write-Host "SMOKE OK: all four tools listed"
```

- [ ] **Step 3: Run the smoke test**

Run (Windows): `powershell -File scripts/smoke.ps1`
(Or on a POSIX box: `bash scripts/smoke.sh`.)
Expected: `SMOKE OK: all four tools listed`.

- [ ] **Step 4: Commit**

```bash
git add scripts/
git commit -m "test(mcp): add stdio protocol smoke scripts"
```

---

### Task 8: Container image + Makefile (`Containerfile`, `Makefile`)

**Files:**
- Create: `Containerfile`
- Create: `Makefile`
- Create: `.dockerignore`

**Interfaces:**
- Consumes: the buildable Go module (Task 6).
- Produces: a runnable `cavimg-mcp` image and `make build|run|test|smoke` targets.

- [ ] **Step 1: Write the Containerfile**

Create `Containerfile`:

```dockerfile
# syntax=docker/dockerfile:1

# ---- build stage: static Go binary ----
# Pin the exact patch to match the go.mod `go 1.26.3` directive (a lagging floating
# tag would otherwise trigger a toolchain re-download at build).
FROM golang:1.26.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cavimg-mcp .

# ---- runtime stage: node (for npm/pnpm/yarn) + the binary, non-root ----
FROM node:22-alpine
RUN corepack enable
RUN addgroup -S app && adduser -S -G app -u 10001 app
COPY --from=build /out/cavimg-mcp /usr/local/bin/cavimg-mcp
USER app
WORKDIR /workspace
ENV CAVIMG_WORKSPACE_ROOT=/workspace
ENTRYPOINT ["/usr/local/bin/cavimg-mcp"]
```

- [ ] **Step 2: Write the .dockerignore**

Create `.dockerignore`:

```
cavimg-mcp
*.exe
.git
docs
scripts/*.ps1
```

- [ ] **Step 3: Write the Makefile**

Create `Makefile`:

```makefile
IMAGE ?= cavimg-mcp
WORKSPACE ?= $(CURDIR)

.PHONY: build run test smoke

build:
	podman build -t $(IMAGE) -f Containerfile .

run:
	podman run -i --rm --userns=keep-id \
		-v "$(WORKSPACE)":/workspace:Z \
		-e CAVIMG_WORKSPACE_ROOT=/workspace \
		$(IMAGE)

test:
	go test ./...

smoke:
	bash scripts/smoke.sh
```

- [ ] **Step 4: Verify Go tests still pass (podman steps are environment-dependent)**

Run: `make test`
Expected: all Go packages PASS.

If `podman` is available on this machine, also run: `make build` then verify the image was created with `podman images cavimg-mcp`. If podman is NOT available, record that `make build`/`make run` are provided as a checklist (see Task 9), not executed here.

- [ ] **Step 5: Commit**

```bash
git add Containerfile Makefile .dockerignore
git commit -m "build(mcp): add multi-stage Containerfile and Makefile"
```

---

### Task 9: README — tool contracts, podman, Codex config, verification checklist (`README.md`)

**Files:**
- Create: `README.md`

**Interfaces:**
- Consumes: everything above.
- Produces: user-facing docs, the exact podman commands, the Codex config snippet, and the verification checklist.

- [ ] **Step 1: Write the README**

Create `README.md`:

````markdown
# cavimg-mcp

A stdio [MCP](https://modelcontextprotocol.io) server that helps an AI coding agent
adopt the [`cavimg`](https://github.com/TheeraphatStudent/cavimg) npm package —
which renders an `<img>` into a `<canvas>` (`<cav-img>`) so images are undraggable
and harder to copy — into any frontend project.

Built with the official Go SDK (`github.com/modelcontextprotocol/go-sdk` v1.6.1).

## Tools

| Tool | Input | Output |
|------|-------|--------|
| `detect_project` | `project_path` | package manager, framework, TypeScript, source dirs, evidence |
| `install_cavimg` | `project_path`, `version?`, `package_manager?`, `dry_run=true` | command, executed, exit_code, stdout/stderr (truncated) |
| `list_image_usages` | `project_path`, `glob?` | hits: `{file, line, kind, match}` |
| `apply_cavimg` | `project_path`, `files?`, `dry_run=true` | unified diff, changed_files, wiring guidance |

Each tool returns structured JSON **and** a one-line human summary. `install_cavimg`
and `apply_cavimg` default to `dry_run: true` and never mutate unless you pass
`dry_run: false`. `apply_cavimg` rewrites only plain `<img>` tags (not `next/image`)
and returns framework registration steps as guidance — it never edits your app code
to inject them.

## Security

Every path is confined to `CAVIMG_WORKSPACE_ROOT` (default `/workspace`). Paths that
escape via `..`, absolute paths, or symlinks are rejected with no filesystem access.

## Build & run (Podman)

```bash
# Build the image
podman build -t cavimg-mcp -f Containerfile .

# Run the server, mounting your project workspace
podman run -i --rm --userns=keep-id \
  -v "$PWD":/workspace:Z \
  -e CAVIMG_WORKSPACE_ROOT=/workspace \
  cavimg-mcp
```

- `:Z` relabels the mount for SELinux systems.
- `--userns=keep-id` maps the non-root container user (`app`, uid 10001) to your host
  user so the mounted workspace stays writable.
- The container needs registry network access only when you run `install_cavimg`
  with `dry_run: false`.

## Codex config (`~/.codex/config.toml`)

```toml
[mcp_servers.cavimg]
command = "podman"
args = [
  "run", "-i", "--rm", "--userns=keep-id",
  "-v", "/abs/path/to/workspace:/workspace:Z",
  "-e", "CAVIMG_WORKSPACE_ROOT=/workspace",
  "cavimg-mcp",
]
```

Use an absolute host path for the volume (Codex does not shell-expand `$PWD`).
Reload Codex; `tools/list` should show all four tools.

## Verification checklist

1. **Unit + idempotency tests** — `make test` (or `go test ./...`). Covers path
   confinement, stack detection, image scanning, and the apply dry-run + empty-diff-
   on-re-run guarantee.
2. **Protocol smoke** — `bash scripts/smoke.sh` (or `powershell -File scripts/smoke.ps1`
   on Windows). Pipes `initialize` + `tools/list` and asserts all four tools appear.
3. **Container** — `make build`, then pipe the same requests:
   ```bash
   printf '%s\n' \
     '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"c","version":"0"}}}' \
     '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
     '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
   | podman run -i --rm cavimg-mcp
   ```
   Expect a `tools/list` response naming all four tools.
4. **End-to-end (Vite+React)** — in a scratch Vite+React app under the workspace:
   `detect_project` → `Vite+React`; `install_cavimg` with `dry_run:false` adds
   `cavimg` to `package.json`; `apply_cavimg` with `dry_run:true` returns a diff and
   changes nothing; re-running `apply_cavimg` after applying returns an empty diff.
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs(mcp): add README with tool contracts, podman, and verification"
```

---

## Self-Review

**1. Spec coverage** (against `docs/superpowers/specs/2026-07-23-cavimg-mcp-server-design.md`):
- §2 SDK API → Global Constraints + Task 6. ✓
- §3 A1–A7 assumptions → A1 Task 8; A2/A3 Task 5 `Wiring`; A4 Task 5 regex; A5 Task 6 `installCmd`/`LookPath`; A6 Task 6 dry-run branch; A7 Task 1. ✓
- §4 repo layout → all tasks (note: fixtures are synthesized in tests, so no committed `testdata/` — a deliberate simplification). ✓
- §5.1–5.4 tool contracts → Tasks 2/6, 6, 3/6, 5/6. ✓
- §6 confinement → Task 1. ✓
- §7 container/ops → Tasks 8, 9. ✓
- §8 testing/honesty split → Tasks 6, 7; Task 8 Step 4 + Task 9 record podman as checklist when unavailable. ✓
- §9 acceptance criteria → #1 Task 7; #3/#4 Task 6 (`TestApplyDryRunReturnsDiffWithoutWriting`, `TestApplyIsIdempotent`) + Task 9 checklist; #2 Task 9 config. ✓

**2. Placeholder scan:** no TBD/TODO; every code step contains complete code. The only intentional user-supplied token is the absolute volume path in the Codex snippet, explained in prose. ✓

**3. Type consistency:** `detect.Result` fields ↔ `DetectOutput` copy in Task 6 match; `scan.Hit` reused directly in `ListOutput`; `rewrite.Transform(filename, content) (string, bool)` and `rewrite.Wiring(framework) ([]string, bool)` called with matching signatures in Task 6; `textdiff.Unified(path, old, new) string` called with three strings; `workspace.Confine(root, candidate)` used in Tasks 1 and 6 identically; handler signatures match the SDK `ToolHandlerFor` shape. ✓

---

## Execution deviations (discovered while implementing)

Two things in the plan-as-written did not survive contact with the real SDK/runtime.
Both were fixed; the shipped code differs from the Task 6/7 listings above as follows.

**D1 — `main.go` treats client disconnect as a clean exit (not `log.Fatalf`).**
On stdin EOF the SDK's `Server.Run` returns an error wrapping the internal jsonrpc2
`ErrServerClosing` ("server is closing: EOF") — a *normal* shutdown for a per-session
stdio server. The plan's `log.Fatalf` turned that into exit code 1, which (a) is wrong
and (b) breaks any `set -e`/`pipefail` verification. The shipped `main.go` routes the
Run error through an `isCleanShutdown(err)` helper: it returns 0 for `nil`,
`errors.Is(err, io.EOF)`, `errors.Is(err, context.Canceled)`, or an error whose message
contains `"server is closing"`/`"client is closing"` (the value lives in an internal
package that cannot be imported, so its stable message is matched). Only genuine errors
`log.Printf` + `os.Exit(1)`.

**D2 — smoke scripts build the binary, hold stdin open, and force BOM-less UTF-8.**
The plan's simple `printf ... | go run .` pipe fails for two reasons found at runtime:
- **Flush race:** the SDK tears the connection down on read-EOF, which can beat the
  flushing of in-flight responses; blasting the requests and immediately closing stdin
  yields **zero** output. Real MCP clients keep stdin open for the whole session. The
  shipped `smoke.sh` uses `{ printf …; sleep 1; } | ./cavimg-mcp` (build first, hold
  open past flush). `smoke.ps1` uses a `System.Diagnostics.Process` with a
  `Start-Sleep` before `StandardInput.Close()`.
- **Windows BOM:** Windows PowerShell 5.1 prepends a UTF-8 BOM to a child's stdin,
  which the JSON decoder rejects ("invalid character 'ï'"). `smoke.ps1` sets
  `[Console]::InputEncoding = New-Object System.Text.UTF8Encoding($false)` before
  starting the process. (`ProcessStartInfo.StandardInputEncoding` is not surfaced on
  this host, and writing raw bytes to `BaseStream` did not suppress the preamble.)

**Also observed (no code change needed):** MCP tool calls are dispatched
**concurrently** by the SDK. A batched pipe that sends `apply(dry_run:false)` then
`apply(dry_run:true)` back-to-back races on the same file; the responses can even
return out of order. Idempotency was therefore verified with the write settled before
the re-check (and is proven deterministically by `TestApplyIsIdempotent`). The README
notes that agents should await each response before the next when order matters.

**Verification actually performed on this machine (Windows + podman 5.6.2 WSL):**
- `go test ./...` — all packages pass, including `TestApplyIsIdempotent` and the
  dry-run-no-write test.
- `bash scripts/smoke.sh` and `powershell -File scripts/smoke.ps1` — both print
  `SMOKE OK`, exit 0.
- `podman build` — image `localhost/cavimg-mcp:latest` built.
- `podman run -i --rm cavimg-mcp` with piped `initialize`+`tools/list` — 3919 bytes,
  all four tools listed (**acceptance #1, container level**).
- `podman run … -v <scratch>:/workspace cavimg-mcp` against a scratch Vite+React app —
  `detect_project` → `Vite+React`; `apply_cavimg` dry-run returned a 184-byte diff;
  `apply_cavimg` `dry_run:false` rewrote `src/App.tsx` to `<cav-img … />`; a settled
  re-run returned an empty diff (**acceptance #3 apply + #4**).
- `install_cavimg` `dry_run:false` against a minimal app — ran `npm install cavimg`
  (exit 0); `package.json` gained `"dependencies":{"cavimg":"^1.0.1"}` (**acceptance #3
  install**).
- **#2 (Codex lists the tools after config reload)** — not verified here; requires a
  Codex install. The config snippet is in the README.
