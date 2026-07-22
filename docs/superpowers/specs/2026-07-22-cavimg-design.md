# cavimg — Design Spec

- **Date:** 2026-07-22
- **Status:** Approved (brainstorming complete)
- **Author:** th33raphat / Claude Code

## 1. Summary

`cavimg` is a lightweight, framework-agnostic package that renders images into a
`<canvas>` via a Web Component (`<cav-img src="…">`) to make images **harder to
casually copy**: it blocks drag-to-desktop and the right-click "Save image as…"
menu, and it keeps the image out of the DOM as an `<img>` tag / `src` attribute.

The package ships with a QA/validation layer: an exhaustive client-side
copy-vector threat matrix, real-browser E2E across Next.js / Vite / Angular,
before/after screenshots committed to `/images`, and a README performance +
protection analysis.

## 2. Honest threat model (scope: casual deterrence)

Canvas rendering is **not** true protection. It raises the bar against casual
copying only. The design and docs must state this plainly rather than overpromise.

- **What it stops:** right-click "Save image as…", drag-to-desktop, the `<img>`
  tag + URL appearing in the Elements panel.
- **What it only slows:** copy-image, extension-based grabbers, view-source.
- **What it CANNOT stop:** OS/browser **screenshots** & screen recording, the
  **Network tab** (the raw file is still fetched over the wire), `canvas.toDataURL()`
  / `toBlob()` pixel export, printing, and headless scraping.

The full block/partial/can't table for ~15 vectors lives in
`docs/threat-matrix.md`; the README carries a summary.

## 3. Non-goals

- No anti-screenshot / anti-`toDataURL` measures.
- No Network-tab / over-the-wire URL hiding (impossible client-side).
- No backend, tokenized/expiring delivery, DRM, or watermarking.
- Example apps are dev-only and are **not** published to npm.

## 4. Repository layout (pnpm monorepo, modular / "structural")

```
cavimg/
├─ pnpm-workspace.yaml, tsconfig.base.json, .gitignore, README.md
├─ images/                        # COMMITTED before/after + perf screenshots
├─ docs/
│  ├─ superpowers/specs/2026-07-22-cavimg-design.md
│  └─ threat-matrix.md            # full QA vector table
├─ packages/cavimg/               # the ONLY published package
│  ├─ package.json, tsup.config.ts
│  ├─ src/
│  │  ├─ fit.ts                   # pure geometry: contain/cover/fill → rect
│  │  ├─ loader.ts                # in-memory Image → ImageBitmap, scrub url/attr
│  │  ├─ render.ts                # size canvas × DPR, drawImage
│  │  ├─ protect.ts               # drag/contextmenu blocking + CSS
│  │  ├─ cav-img-element.ts       # <cav-img> custom element (glue + ResizeObserver)
│  │  ├─ index.ts                 # defineCavImg(), auto-register, re-exports
│  │  └─ types.ts
│  └─ test/                       # Vitest + happy-dom unit tests
├─ examples/{vite,next,angular}/  # dev-only smoke/demo apps
└─ e2e/                           # Playwright: config (3 projects), tests/, fixtures/large/
```

## 5. Module boundaries (each one job, independently testable)

- **`fit.ts`** — pure function, no DOM: `(imgW, imgH, boxW, boxH, mode) → {dx,dy,dw,dh}`.
- **`loader.ts`** — `new Image()` → `crossOrigin = 'anonymous'` → `onload` →
  `createImageBitmap` → returns the bitmap; nulls the URL string and removes the
  host `src` attribute. No canvas dependency.
- **`render.ts`** — sizes the canvas backing store to
  `element box × devicePixelRatio`, applies the fit rect, calls `drawImage`.
- **`protect.ts`** — `draggable = false`, prevents `dragstart` and `contextmenu`,
  sets `user-select: none` / `-webkit-user-drag: none` CSS.
- **`cav-img-element.ts`** — `CavImgElement extends HTMLElement`; **open shadow
  root** holding one `<canvas>`; `observedAttributes = ['src','fit','alt']`;
  sets `role="img"` + `aria-label` from `alt`. Owns a private `#url` field, a
  cached `ImageBitmap`, and the `ResizeObserver`. **Both `#url` and the bitmap
  are retained across disconnect/reconnect** (the bitmap is not closed in
  `disconnectedCallback`) so a DOM move — React reconciliation, route change,
  list reorder — reconnects and redraws from the cache without a refetch. On
  load it sets the host `style.aspectRatio` from the bitmap's natural size.
- **`index.ts`** — idempotent `defineCavImg()`, auto-registers on import,
  re-exports `renderToCanvas` (functional escape hatch) + types.

## 6. Public API

- **Element:** `<cav-img src fit alt>` (registered as `cav-img`).
- **Attributes:** `src` (scrubbed from DOM after load), `fit = contain | cover |
  fill` (**default `contain`**), `alt` → `aria-label`.
- **JS properties:** `el.src` (get/set), `el.fit`, and an `el.load(url)` alias —
  attribute↔property reflection, mirroring native `<img>`. `get src` returns the
  retained `#url` (honest read-back even after the attribute is scrubbed). Lets
  security-conscious devs set the URL purely in code so it never appears in markup.
- **Events:** `cav-load` on success, `cav-error` on load failure.
- **Functional escape hatch:** `renderToCanvas(canvasEl, url, opts)`.

## 7. Data flow

`src set` → `loader.load()` → decode to `ImageBitmap`, cache in the private
`#bitmap` field, store the URL in the private `#url` field, **remove the `src`
attribute from the DOM**, set host `aspect-ratio` → `render.draw()` →
`protect.harden()` (once) → on `ResizeObserver` fire, `render.draw()` again
**reusing the cached bitmap**. Disconnect keeps `#url` + `#bitmap`; reconnect
redraws from the cache (no refetch). Load failure → `cav-error` event.

**Key resolution:** "hide the URL from the DOM inspector but stay responsive and
survive remounts" is satisfied by removing the URL from the *DOM* (`src`
attribute + no `<img>` tag) while retaining it in a non-enumerable private field
plus the decoded pixels as an `ImageBitmap`. The URL is never in the DOM; redraws
and reconnects never refetch. (Honest limit: the URL still lives in JS memory and
the Network tab — consistent with the casual-deterrence scope.)

## 8. Framework integration

- **Vite** — `import 'cavimg'`; use `<cav-img>` directly.
- **Next.js** — Web Components are client-only; the example registers via a
  `'use client'` component with `useEffect`/dynamic import to avoid SSR
  `HTMLElement is not defined`.
- **Angular** — the example adds `CUSTOM_ELEMENTS_SCHEMA` and imports `cavimg`
  in `main.ts`.

## 9. Build & distribution

- **tsup**, zero runtime deps. Outputs: ESM (`import 'cavimg'` auto-registers),
  a minified **IIFE global** for `<script>`/CDN, and `.d.ts` types.
- Proper `exports` map, `sideEffects` for the auto-register entry, `files`
  whitelist so only `dist/` ships.

## 10. Testing

- **Unit (Vitest + happy-dom):** fit math (all 3 modes; portrait/landscape;
  upscaling; zero-size edge cases), loader scrubbing (attribute removed, URL not
  retained, `crossOrigin` set), attribute↔property reflection, event blocking.
  Canvas ctx / `Image` / `createImageBitmap` / `ResizeObserver` mocked.
- **E2E (Playwright, 3 projects — vite/next/angular, each with its own
  `webServer`):**
  - `protection.spec` — assert the shadow root has a `<canvas>`, there is **no
    `<img>`** in light or shadow DOM, and the host has **no `src`** after load;
    `contextmenu` and `dragstart` are prevented. Screenshot the rendered output →
    `/images/after-<fw>.png`; a plain-`<img>` control page → `/images/before-<fw>.png`.
  - `perf.spec` — `performance` marks around fetch→decode→draw, first paint, and
    JS heap (Chromium, best-effort); write a metrics JSON + a rendered comparison
    to `/images/perf-<fw>.png`.
- **Execution order:** build package → unit → E2E against built output → emit
  `/images` + perf JSON → generate the README before/after section.

## 11. Deliverables

- `docs/threat-matrix.md` — full vector table.
- `/images/*.png` — before/after protection screenshots + perf comparison (committed).
- **README** — what-it-is (honest one-liner) → install/quickstart per framework →
  API → **Before/After (protection screenshots + perf table)** → threat-matrix
  summary → limitations.

## 12. `.gitignore`

Ignore `node_modules`, `dist`, `.next`, Angular `dist`, `playwright-report`,
`test-results`, `coverage`; huge images `e2e/**/fixtures/large/**`, `*.psd`,
`*.tiff`, `*.raw`, `*.heic`. Keep `/images` tracked (add `!images/` guard if a
broad image glob is used).

## 13. Resolved sub-decisions

- Fit default: **`contain`**.
- Shadow DOM: **open** (closed adds no real security, hurts a11y/testing).
- Host sizing: `:host{display:block}` + `aspect-ratio` set from the bitmap's
  natural size on load, so the element is responsive (fills its container's
  width, height from ratio) instead of collapsing to the canvas default
  300×150. Consumers can override with explicit CSS width/height.
- URL handling: retained in a private `#url` field (not the DOM); attribute
  scrubbed after load; bitmap + URL survive disconnect/reconnect.
- Next.js example: **`useEffect` client-side registration**.
- Responsiveness: **`ResizeObserver`** (not `window.resize`) + `devicePixelRatio`.
- Packaging: **tsup** → ESM + IIFE global + types.

## 14. Implementation phasing

1. Package core (`src` modules) + Vitest unit tests.
2. Build config (tsup) producing `dist/`.
3. Example apps (vite, next, angular).
4. Playwright E2E + screenshot/perf capture → `/images`.
5. `docs/threat-matrix.md` + README before/after + perf analysis.
