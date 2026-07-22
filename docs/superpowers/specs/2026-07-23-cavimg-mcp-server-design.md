# cavimg-mcp — MCP Server Design Spec

- **Date:** 2026-07-23
- **Status:** Approved (brainstorming complete)
- **Branch:** feature/mcp-tools
- **Location:** `packages/cavimg-mcp/`

## 1. Goal

Build, containerize, and verify a stdio MCP server named `cavimg-mcp` that lets an
AI coding agent adopt the `cavimg` npm package into any frontend project. It
exposes four tools — `detect_project`, `install_cavimg`, `list_image_usages`,
`apply_cavimg` — following the official Go SDK
(`github.com/modelcontextprotocol/go-sdk`, pinned at **v1.6.1** in the existing
`go.mod`).

`cavimg` renders an `<img>` into a `<canvas>` (`<cav-img>` web component) so the
image can't be dragged or trivially copied. The server automates detecting a
project's stack, installing the package, finding image usages, and rewriting them
to `<cav-img>`.

## 2. SDK API (pinned to the installed v1.6.1, not the web guide)

Verified against the module cache — the current SDK uses generic typed tools, which
differs from older versions of the official guide:

- `mcp.NewServer(impl *mcp.Implementation, opts *mcp.ServerOptions) *mcp.Server`
- `mcp.AddTool[In, Out any](s *mcp.Server, t *mcp.Tool, h ToolHandlerFor[In, Out])`
- `ToolHandlerFor[In, Out] = func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)`
- `server.Run(ctx, &mcp.StdioTransport{})` for stdio.

The typed `Out` struct is auto-serialized as the tool's **structured JSON** result;
we additionally set `CallToolResult.Content` to a one-line **human-readable
summary**. This satisfies "structured JSON plus a short human-readable summary"
natively, per tool.

## 3. Reconciling assumptions (spec-vs-reality calls, user-approved)

- **A1 — Runtime is `node:alpine`, not distroless.** Distroless has no shell or
  package manager, but `install_cavimg` shells out to npm/pnpm/yarn. "distroless/
  alpine" therefore resolves to **alpine-with-Node**. Multi-stage: Go builder →
  `node:22-alpine` runtime.
- **A2 — Registration wiring is NOT auto-injected.** `apply_cavimg` rewrites tags on
  disk and returns the framework-specific wiring snippet as **guidance text**, not by
  editing arbitrary user code. (Chosen to keep idempotency trivial and avoid
  corrupting hand-written files.)
- **A3 — Tag rewrite is framework-agnostic; wiring guidance is framework-specific.**
  Wiring is fully specified for plain-HTML/Vite, Next/React, and Angular (working
  examples exist for all three in `examples/`). Vue/Nuxt/Svelte/SvelteKit: tags are
  still rewritten, but wiring is flagged **`manual: true`** ("see cavimg docs")
  rather than half-implemented.
- **A4 — `next/image` `<Image>` is out of scope.** Only plain `<img>` tags are
  matched and rewritten.
- **A5 — bun is detected but not executed.** The runtime image ships npm + corepack
  (pnpm/yarn) per the runtime spec. If a project's PM is bun, `install_cavimg`
  reports the command it *would* run plus a note that bun isn't in the image, rather
  than failing silently.
- **A6 — `dry_run` reports the command without executing it** (no reliance on
  `npm --dry-run` semantics), guaranteeing "change nothing on dry run."
- **A7 — Workspace root via `CAVIMG_WORKSPACE_ROOT`** (default `/workspace`). All
  four tools confine every path to it.

## 4. Repo layout

```
packages/cavimg-mcp/
├── go.mod / go.sum              # already present (SDK v1.6.1 pinned)
├── main.go                      # wire server + stdio transport, register 4 tools
├── internal/
│   ├── workspace/confine.go     # path-confinement helper (shared) + tests
│   ├── detect/detect.go         # package-manager + framework + TS + source-dir detection + tests
│   ├── scan/scan.go             # list_image_usages: img-tag + image-import line scanner + tests
│   ├── rewrite/rewrite.go       # apply_cavimg: tag transform + unified diff + idempotency + tests
│   └── tools/tools.go           # 4 typed handlers: In/Out structs, summary text, error mapping
├── testdata/                    # fixture mini-projects (next, vite-react, angular, vue, plain-html)
├── Containerfile                # multi-stage: golang:alpine → node:alpine, non-root
├── Makefile                     # build / run / test
├── scripts/smoke.sh             # pipe initialize + tools/list over stdin, assert 4 tools
└── README.md                    # tool contracts + podman + Codex config + verification checklist
```

## 5. Tool contracts

Each handler returns a typed `Out` struct (structured JSON) **and** sets `Content`
to a one-line summary. Path-confinement failure returns an `IsError` result with a
clear message and performs **no** mutation.

### 5.1 `detect_project`
- **In:** `{ project_path string }`
- **Out:** `{ package_manager, framework, typescript bool, source_dirs []string, evidence map, summary string }`
- **Package manager:** lockfile first (`pnpm-lock.yaml`→pnpm, `yarn.lock`→yarn,
  `bun.lockb`/`bun.lock`→bun, `package-lock.json`→npm), then the `packageManager`
  field in `package.json` (corepack), else default `npm`.
- **Framework:** deps + config files — `next` dep or `next.config.*`→Next.js;
  `nuxt`→Nuxt; `@sveltejs/kit`→SvelteKit; `svelte`→Svelte; `vite`+`react`→Vite+React;
  `vite` alone→Vite; `vue` (no nuxt)→Vue; `@angular/core`→Angular; none + `index.html`
  present→plain HTML.
- **TypeScript:** `tsconfig.json` present OR `typescript` dep OR any `.ts/.tsx` file.
- **Source dirs:** report which of `src`, `app`, `pages`, `components` exist (plus
  project root for plain HTML).

### 5.2 `install_cavimg`
- **In:** `{ project_path string, version? string, package_manager? string, dry_run bool (default true) }`
- **Out:** `{ command string, executed bool, exit_code *int, stdout string, stderr string, truncated bool, summary string }`
- **Command:** `npm install cavimg@<v|latest>` / `pnpm add cavimg@…` /
  `yarn add cavimg@…` / `bun add cavimg@…`, PM from arg or `detect_project`.
- **dry_run=true:** return the command, `executed=false`, `exit_code=null`, **no exec**.
- **dry_run=false:** exec in `project_path`, capture stdout/stderr (truncate each to
  ~8 KB, set `truncated`), return real exit code.
- **bun (A5):** if PM is bun and bun isn't on PATH, `executed=false` with an
  explanatory stderr note.

### 5.3 `list_image_usages`
- **In:** `{ project_path string, glob? string }`
- **Out:** `{ hits []{ file, line, kind, match }, count int, summary string }`
- **Default glob:** `**/*.{html,jsx,tsx,vue,svelte,astro}`, skipping
  `node_modules`, `dist`, `.next`, `build`, `.git`. `glob` overrides.
- **Matches:** `<img …>` (kind `img-tag`) and `import x from '….(png|jpe?g|gif|webp|avif|svg)'`
  (kind `image-import`). `next/image` `<Image>` is intentionally not matched (A4).

### 5.4 `apply_cavimg`
- **In:** `{ project_path string, files? []string, dry_run bool (default true) }`
- **Out:** `{ dry_run bool, diff string, changed_files []string, hunks int, wiring { framework, steps []string, manual bool }, summary string }`
- **Transform:** rewrite `<img …>` → `<cav-img …>` preserving all attributes.
  Extension-aware closing: `.jsx`/`.tsx` keep `<cav-img … />` self-closing; HTML-ish
  (`.html`/`.vue`/`.svelte`) emit `<cav-img …></cav-img>`. Only `img-tag` hits are
  rewritten — never image imports.
- **Target files:** the `files` arg if given (each re-confined under `project_path`),
  else all `img-tag` hits from the scanner.
- **Idempotency (acceptance #4):** only `<img` is matched, so a second run finds
  nothing to convert → **empty diff**. This is a first-class Go test.
- **dry_run=true (default):** compute a unified diff, write **nothing** to disk.
- **dry_run=false:** write the files, return the diff of applied changes +
  `changed_files`.
- **Wiring (A2/A3):** `wiring.steps` carries a registration snippet keyed on the
  **detected framework** (not on "is it React"):
  - **plain HTML** → `<script type="module">import 'cavimg'</script>` (or the CDN tag).
  - **Vite / Vite+React** (client-only SPA) → side-effect `import 'cavimg'` in the
    entry module (e.g. `main.tsx`); auto-registers, no `useEffect` needed.
  - **Next.js** (SSR, web components are client-only) →
    `useEffect(() => { import('cavimg').then(m => m.defineCavImg()); }, [])` in a
    `'use client'` component.
  - **Angular** → add `CUSTOM_ELEMENTS_SCHEMA` to the component `schemas` and call
    `defineCavImg()`.
  - **Vue / Nuxt / Svelte / SvelteKit** → `manual: true` with a cavimg-docs pointer.

## 6. Security model — path confinement

A single `workspace.Confine(root, candidate)` helper, used by all four tools:

1. Resolve `root` via `filepath.EvalSymlinks` once at startup (from
   `CAVIMG_WORKSPACE_ROOT`, default `/workspace`).
2. `filepath.Abs` + `filepath.Clean` the candidate; reject if it escapes `root`
   after cleaning (handles `..`).
3. `EvalSymlinks` the real candidate and assert the real path equals `root` or is
   under `root` + separator (handles symlinks pointing outside).
4. In `apply_cavimg`, each target file is independently re-confined under
   `project_path`.
5. Any escape → structured error, **no I/O performed**.

## 7. Container & ops

- **Containerfile (multi-stage):**
  - `FROM golang:1.26-alpine AS build` → `CGO_ENABLED=0 go build -o /out/cavimg-mcp`.
  - `FROM node:22-alpine` runtime → copy the static binary, `corepack enable`
    (pnpm+yarn; npm ships with Node), create non-root user `app` (uid 10001),
    `USER app`, `WORKDIR /workspace`, `ENV CAVIMG_WORKSPACE_ROOT=/workspace`,
    `ENTRYPOINT ["/usr/local/bin/cavimg-mcp"]`.
- **Build:** `podman build -t cavimg-mcp -f Containerfile .`
- **Run:** `podman run -i --rm --userns=keep-id -v "$PWD":/workspace:Z -e CAVIMG_WORKSPACE_ROOT=/workspace cavimg-mcp`
  - `:Z` = SELinux relabel of the mount; `--userns=keep-id` maps the non-root
    container user to the host user so the mounted workspace stays writable.
- **Network:** default podman networking is retained so the package-manager registry
  is reachable (the one allowed network exception). No other outbound calls.
- **Codex `~/.codex/config.toml`:**
  ```toml
  [mcp_servers.cavimg]
  command = "podman"
  args = ["run", "-i", "--rm", "--userns=keep-id",
          "-v", "<ABS_WORKSPACE>:/workspace:Z",
          "-e", "CAVIMG_WORKSPACE_ROOT=/workspace",
          "cavimg-mcp"]
  ```
  (`<ABS_WORKSPACE>` is an absolute host path; Codex does not shell-expand `$PWD`.)
- **Makefile:** `build` → podman build; `run` → the podman run above; `test` →
  `go test ./...` (no podman needed, runs on Windows).

## 8. Testing & verification (honesty split)

- **Go unit tests (`make test` / `go test ./...`, runs locally on Windows, no
  podman):**
  - Confinement: escape via `..`, absolute-path-outside, and symlink-outside (symlink
    case guarded where the platform disallows creating them).
  - `detect_project` across the five `testdata` fixtures (next, vite-react, angular,
    vue, plain-html).
  - `list_image_usages` hit extraction.
  - `apply_cavimg`: dry-run diff matches expected; **apply-then-rerun yields an empty
    diff** (acceptance #3/#4 as pure Go tests).
- **Protocol smoke (`go run`, no container):** `scripts/smoke.sh` (and a Windows
  equivalent) pipes an `initialize` + `tools/list` JSON-RPC exchange to the binary's
  stdin and asserts all four tool names appear (acceptance #1, minus podman).
- **Container / Codex checklist:** exact `podman build`/`run` and config-reload steps
  are documented. During implementation we probe whether `podman` exists on the build
  machine: if present, we run build/run and report real output; if absent, those
  steps are labeled **"provided as checklist, not executed here"** — never implied to
  have run.

## 9. Acceptance criteria (from the brief)

1. `podman run -i --rm cavimg-mcp` answers `initialize` and `tools/list` piped over
   stdin. *(Protocol layer proven locally via `go run` where podman is unavailable.)*
2. Codex lists all four tools after config reload.
3. Scratch Vite+React app: `detect_project` reports the stack; `install_cavimg`
   `dry_run:false` adds cavimg to `package.json`; `apply_cavimg` `dry_run:true`
   returns a diff and modifies nothing.
4. Re-running `apply_cavimg` after applying produces an empty diff.

## 10. Non-goals

- No auto-injection of registration/import code into user files (A2).
- No `next/image` `<Image>` rewriting (A4).
- No fully-specified Vue/Nuxt/Svelte/SvelteKit wiring — detection + tag rewrite only,
  wiring flagged manual (A3).
- No bun execution inside the image (detection only) (A5).
- No network calls beyond the package-manager registry.
- No changes to the published `packages/cavimg` library or the `examples/` apps.
- No non-stdio transport (HTTP/SSE out of scope).

## 11. Deliverables (produced during implementation, after this spec)

Full file contents with paths, then a numbered "Run and verify" section with
copy-pasteable commands and expected output: (1) repo layout, (2) `go.mod` + Go
source for the four tools, (3) `Containerfile`, (4) `Makefile`, (5) exact
`podman build`/`run` commands with `:Z` and `--userns=keep-id`, (6) the Codex
`config.toml` snippet, (7) the verification script/checklist.
