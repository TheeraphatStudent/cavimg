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
   on Windows). Builds the binary, pipes `initialize` + `tools/list` (holding stdin
   open until responses flush), and asserts all four tools appear.
3. **Container** — `make build`, then pipe the same requests. Because a stdio server
   closes on stdin EOF, hold the pipe open briefly so responses flush:
   ```bash
   { printf '%s\n' \
     '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"c","version":"0"}}}' \
     '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
     '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'; sleep 1; } \
   | podman run -i --rm cavimg-mcp
   ```
   Expect a `tools/list` response naming all four tools.
4. **End-to-end (Vite+React)** — in a scratch Vite+React app under the workspace:
   `detect_project` → `Vite+React`; `install_cavimg` with `dry_run:false` adds
   `cavimg` to `package.json`; `apply_cavimg` with `dry_run:true` returns a diff and
   changes nothing; `apply_cavimg` with `dry_run:false` writes; a subsequent
   `apply_cavimg` (`dry_run:true`) returns an empty diff.
   > Note: MCP tool calls are dispatched concurrently, so send each call and await its
   > response before the next when order matters (e.g. write then re-check).

## Notes on stdio behavior

A stdio MCP server is launched per session and shuts down when the client closes
stdin (EOF) — this server treats that as a clean exit, not a failure. When scripting
verification (rather than using a real MCP client that keeps the connection open),
hold stdin open until the responses have been written, as the smoke scripts do.
