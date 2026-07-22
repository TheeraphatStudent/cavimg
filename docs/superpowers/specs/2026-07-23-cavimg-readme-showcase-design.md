# cavimg README Showcase GIF — Design Spec

- **Date:** 2026-07-23
- **Status:** Approved (brainstorming complete)
- **Branch:** feature/examples-e2e-and-docs (continues Plan 2)

## 1. Goal

Add an animated GIF hero to the top of `README.md` that demonstrates cavimg's
protection *in motion*, since the protection is structural (a plain `<img>` and a
`<cav-img>` render pixel-identical output, so a static render shows nothing).

## 2. Why a scripted GIF (not a live capture)

A browser's native right-click "Save image as…" menu is OS/browser chrome rendered
**outside** the page — Playwright and live-browser recorders cannot screenshot it.
So the GIF is a **scripted in-page demo** that faithfully re-enacts the real,
E2E-verified behavior (plain img → menu opens; cav-img → blocked). The README labels
it as an illustration of the actual behavior. The right panel uses a **real
`<cav-img>`** (genuinely canvas-rendered) so the demo isn't a fabricated mockup of
the product — only the cursor/menu/chips overlay is stylized.

## 3. Storyboard (~3s loop, ~24 frames)

Two labeled panels side by side, both showing the same appealing image:
1. **Left = "Plain `<img>`", Right = "`<cav-img>`"** (title cards). Both display the
   showcase SVG.
2. A faux cursor glides to the left image and "right-clicks": a styled
   *"Save image as… / Copy image"* popover appears; a red ❌ tag reads
   *"saveable · draggable · URL visible"*. Hold ~1s.
3. Cursor glides to the right image and "right-clicks": the popover appears
   struck-through / a **🚫 blocked** chip pops; a green ✅ tag reads
   *"no `<img>` · no `src` · save & drag blocked"*. Hold ~1s.
4. Loop.

## 4. The showcase image

A crisp, hand-authored **SVG scene** (stylized sunset over mountains) instead of the
gradient PNG fixture — it reads as real artwork worth protecting. Text-authored, no
encoder needed; `<cav-img>` renders SVG (spike-proven). Lives at
`demo/showcase-art.svg`, served same-origin by the demo page. The E2E fixture
(`fixture.png`) is unchanged.

## 5. Components

- **`demo/showcase.html`** — the demo stage. Real `<img>` (left) and real `<cav-img>`
  (right) both `src` the SVG. An overlay layer (absolutely-positioned faux cursor,
  popover, ❌/✅ tags, 🚫 chip) plus a small inline script exposing a **deterministic**
  `window.__frame(n)` function that sets the scene to frame `n` (cursor position,
  which popover is visible, which tags show). No wall-clock timers — the frame is a
  pure function of `n`, so capture is reproducible. Registers cavimg via
  `import 'cavimg'` (bundled/served by a tiny static setup; see plan).
- **`scripts/make-showcase-gif.mjs`** — a Playwright (chromium) script: opens
  `showcase.html`, waits for `cav-load`, loops `n = 0..FRAMES-1` calling
  `window.__frame(n)` + `page.screenshot({clip})` per frame, encodes the frames to
  `images/showcase.gif` with **`gifenc`** (pure-JS, dev-dep), quantized palette,
  per-frame delay to total ~3s, infinite loop.
- **`images/showcase.gif`** — committed output (~880×360, target ≤ ~500 KB).

## 6. Dependencies & scripts

- Add **`gifenc`** (GIF encode) and **`pngjs`** (decode Playwright's PNG frames to the
  RGBA `gifenc` wants) as dev dependencies in the `e2e` workspace package (it already
  has Playwright + chromium). Both are pure-JS (reliable on Windows). The demo page is
  served by reusing the Vite example's dev server (which resolves `import 'cavimg'`)
  or an import map — the plan pins the exact mechanism.
- Add a root/e2e script `showcase` → runs `make-showcase-gif.mjs` so the GIF is
  regenerable: `pnpm ... showcase`.
- The showcase pipeline needs the library built first (`pnpm --filter cavimg build`),
  same as the E2E tasks.

## 7. README changes

- Insert the GIF hero directly under the title + tagline:
  `![cavimg — plain img vs cav-img protection demo](images/showcase.gif)` with a
  one-line caption noting the menus illustrate the real, E2E-verified behavior.
- Keep all existing sections. Reframe the existing **Before / after** table as
  "static proof: pixel-identical output, structurally different DOM."

## 8. Folded-in fix (from Plan 2 final review)

`examples/angular/src/app/app.spec.ts` asserts the removed scaffold title
(`'Hello, angular'`); the real template renders `cavimg — before / after`. Update the
assertion (or delete the stale spec) so `ng test` isn't red.

## 9. Non-goals

- No live-browser recording; no attempt to capture the native OS context menu.
- No change to the E2E fixture, specs, or the three example apps (beyond the Angular
  test fix).
- No new runtime dependency in the published `packages/cavimg` (gifenc is dev-only,
  in the demo/e2e tooling, never in the library).
- Convenience `pnpm e2e:*` scripts are out of scope (tangential to the showcase).

## 10. Honesty

The demo overlay (cursor, menu, chips) is a stylized illustration; the README says
so. The underlying behavior it depicts is real and proven by `e2e/tests/protection.spec.ts`
(canvas rendered, no `<img>`, `src` scrubbed, contextmenu + dragstart prevented).
