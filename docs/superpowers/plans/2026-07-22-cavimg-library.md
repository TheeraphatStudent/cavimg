# cavimg Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the publishable `cavimg` package — a `<cav-img>` Web Component that renders images to a `<canvas>` to make them harder to casually copy — with full unit tests and a tsup build.

**Architecture:** A pnpm monorepo whose only published package is `packages/cavimg`. The package is split into single-responsibility modules — `fit` (pure geometry), `loader` (in-memory decode), `render` (canvas draw), `protect` (anti-copy wiring), `cav-img-element` (the custom element), `index` (registration + exports). Pure logic is unit-tested directly; the element is tested in happy-dom with stubbed browser globals.

**Tech Stack:** TypeScript (strict), tsup (build), Vitest + happy-dom (test), pnpm workspaces. Zero runtime dependencies.

**Scope note:** This is Plan 1 of 2. Plan 2 (Vite/Next/Angular example apps, Playwright E2E, `/images` screenshots, README perf analysis, `docs/threat-matrix.md`) is written separately after this library is built and installable. Full design: `docs/superpowers/specs/2026-07-22-cavimg-design.md`.

## Global Constraints

Every task's requirements implicitly include these. Copy exact values verbatim.

- **Published package name:** `cavimg`, version `1.0.0`, license `MIT`, author `th33raphat`. Zero runtime dependencies.
- **TypeScript strict mode** on. Target/lib `ES2022` (native `#private` fields).
- **Element tag:** `cav-img`. `observedAttributes` is exactly `['src', 'fit', 'alt']`.
- **Fit modes:** exactly `'contain' | 'cover' | 'fill'`. Default `'contain'`.
- **Events:** `cav-load` (success), `cav-error` (load failure), dispatched as `CustomEvent`.
- **CORS:** the loader must set `img.crossOrigin = 'anonymous'` before assigning `src`.
- **URL hiding:** no `<img>` element is ever mounted; the `src` attribute is removed from the host after load. The URL is retained only in a private `#url` field (never in the DOM).
- **Reconnect:** `#url` and the decoded `ImageBitmap` are retained across `disconnectedCallback`; reconnect redraws from the cache with no refetch.
- **Host sizing:** shadow CSS `:host{display:block}`; on load set `host.style.aspectRatio` from the bitmap's natural size.
- **Shadow root:** mode `open`.
- **Error messages must not contain the image URL.**
- **Build (tsup):** ESM at `dist/index.js`, minified IIFE at `dist/index.global.js` (global name `Cavimg`), plus `dist/index.d.ts` types.
- **Test:** Vitest with `environment: 'happy-dom'`.

## File Structure

```
cavimg/                         (repo root → becomes workspace root)
├─ pnpm-workspace.yaml          (create)
├─ tsconfig.base.json           (create)
├─ package.json                 (modify: turn into private workspace root)
├─ .gitignore                   (create)
└─ packages/cavimg/
   ├─ package.json              (create: the real published package)
   ├─ tsconfig.json             (create)
   ├─ tsup.config.ts            (create)
   ├─ vitest.config.ts          (create)
   ├─ src/
   │  ├─ types.ts               Task 2 — FitMode, FitRect
   │  ├─ fit.ts                 Task 2 — computeFit()
   │  ├─ loader.ts              Task 3 — loadImageBitmap()
   │  ├─ render.ts              Task 4 — drawBitmap()
   │  ├─ protect.ts             Task 5 — harden()
   │  ├─ cav-img-element.ts     Task 6 — CavImgElement
   │  └─ index.ts               Task 1 stub → Task 7 real (defineCavImg, renderToCanvas, exports)
   └─ test/
      ├─ setup.ts               Task 1 — global stubs
      ├─ smoke.test.ts          Task 1
      ├─ fit.test.ts            Task 2
      ├─ loader.test.ts         Task 3
      ├─ render.test.ts         Task 4
      ├─ protect.test.ts        Task 5
      ├─ element.test.ts        Task 6
      └─ index.test.ts          Task 7
```

**Existing state:** the repo root currently has a `package.json` named `cavimg` (the original, with keywords/author/repo). Task 1 repurposes root as the private workspace and moves the real package identity into `packages/cavimg/package.json`, carrying over keywords/author/license/repo.

---

### Task 1: Monorepo + package scaffolding & tooling

**Files:**
- Create: `pnpm-workspace.yaml`, `tsconfig.base.json`, `.gitignore`
- Modify: `package.json` (root → private workspace)
- Create: `packages/cavimg/package.json`, `packages/cavimg/tsconfig.json`, `packages/cavimg/tsup.config.ts`, `packages/cavimg/vitest.config.ts`
- Create: `packages/cavimg/src/index.ts` (stub), `packages/cavimg/test/setup.ts`, `packages/cavimg/test/smoke.test.ts`

**Interfaces:**
- Produces: workspace with package `cavimg`; `pnpm --filter cavimg test` and `pnpm --filter cavimg build` both work; `src/index.ts` exports `const VERSION = '1.0.0'`; `test/setup.ts` installs global stubs (`ResizeObserver`, `Image`, `createImageBitmap`, canvas `getContext`/`getBoundingClientRect`) and exposes created images on `globalThis.__cavCreatedImages`.

- [ ] **Step 1: Create `pnpm-workspace.yaml`**

```yaml
packages:
  - packages/*
  - examples/*
```

- [ ] **Step 2: Rewrite root `package.json` as the private workspace root**

```json
{
  "name": "cavimg-workspace",
  "private": true,
  "version": "0.0.0",
  "description": "Workspace for the cavimg package",
  "license": "MIT",
  "author": "th33raphat",
  "scripts": {
    "build": "pnpm --filter cavimg build",
    "test": "pnpm --filter cavimg test",
    "typecheck": "pnpm --filter cavimg typecheck"
  }
}
```

- [ ] **Step 3: Create `tsconfig.base.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "strict": true,
    "declaration": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "useDefineForClassFields": true,
    "verbatimModuleSyntax": true
  }
}
```

- [ ] **Step 4: Create `.gitignore`**

```gitignore
node_modules/
dist/
coverage/
*.log
.DS_Store

# framework build outputs (Plan 2)
.next/
examples/*/dist/
playwright-report/
test-results/

# huge images — never commit raw/source assets
e2e/**/fixtures/large/**
*.psd
*.tiff
*.tif
*.raw
*.heic

# keep committed comparison outputs
!images/
```

- [ ] **Step 5: Create `packages/cavimg/package.json`** (the real published package; devDependencies are filled by Step 8)

```json
{
  "name": "cavimg",
  "version": "1.0.0",
  "description": "An covering image: render an image to <canvas> to make it undraggable and harder to copy",
  "type": "module",
  "main": "./dist/index.js",
  "module": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js"
    }
  },
  "unpkg": "./dist/index.global.js",
  "jsdelivr": "./dist/index.global.js",
  "sideEffects": ["./dist/index.js", "./dist/index.global.js"],
  "files": ["dist"],
  "scripts": {
    "build": "tsup",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit"
  },
  "keywords": [
    "image-to-canvas", "canvas", "image", "image-protection", "copy-protection",
    "anti-copy", "prevent-download", "disable-right-click", "no-drag",
    "undraggable", "image-security", "browser", "web-component"
  ],
  "author": "th33raphat",
  "license": "MIT",
  "repository": { "type": "git", "url": "git+https://github.com/TheeraphatStudent/cavimg.git" },
  "bugs": { "url": "https://github.com/TheeraphatStudent/cavimg/issues" },
  "homepage": "https://github.com/TheeraphatStudent/cavimg#readme"
}
```

- [ ] **Step 6: Create `packages/cavimg/tsconfig.json`**

```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "outDir": "dist",
    "rootDir": "src"
  },
  "include": ["src"]
}
```

- [ ] **Step 7: Create `packages/cavimg/tsup.config.ts`**

```ts
import { defineConfig } from 'tsup';

export default defineConfig({
  entry: { index: 'src/index.ts' },
  format: ['esm', 'iife'],
  globalName: 'Cavimg',
  dts: true,
  clean: true,
  minify: true,
  sourcemap: true,
  target: 'es2022',
});
```

- [ ] **Step 8: Install dev dependencies (populates `packages/cavimg/package.json` devDependencies)**

Run from repo root:
```bash
pnpm --filter cavimg add -D typescript tsup vitest happy-dom
```
Expected: pnpm resolves current versions and writes them into `packages/cavimg/package.json`; a root `pnpm-lock.yaml` and `node_modules` are created.

- [ ] **Step 9: Create `packages/cavimg/vitest.config.ts`**

```ts
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'happy-dom',
    setupFiles: ['./test/setup.ts'],
    include: ['test/**/*.test.ts'],
  },
});
```

- [ ] **Step 10: Create `packages/cavimg/test/setup.ts`** (shared global stubs for all test files)

```ts
import { vi } from 'vitest';

// ResizeObserver is not needed to fire in unit tests — a no-op stub is enough.
class ResizeObserverStub {
  constructor(_cb: ResizeObserverCallback) {}
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverStub);

// Track every Image the code under test creates so loader tests can assert on it.
interface TrackedImage {
  crossOrigin: string;
  decoding: string;
  onload: (() => void) | null;
  onerror: (() => void) | null;
  src: string;
}
const createdImages: TrackedImage[] = [];
(globalThis as unknown as { __cavCreatedImages: TrackedImage[] }).__cavCreatedImages = createdImages;

class ImageStub implements TrackedImage {
  crossOrigin = '';
  decoding = '';
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  private _src = '';
  constructor() {
    createdImages.push(this);
  }
  set src(value: string) {
    this._src = value;
    if (!value) return; // empty assignment = discard, do not "load"
    queueMicrotask(() => {
      if (value.includes('bad')) this.onerror?.();
      else this.onload?.();
    });
  }
  get src(): string {
    return this._src;
  }
}
vi.stubGlobal('Image', ImageStub);

vi.stubGlobal(
  'createImageBitmap',
  vi.fn(async () => ({ width: 200, height: 100, close: vi.fn() }) as unknown as ImageBitmap),
);

// happy-dom has no real 2D canvas context; stub getContext + layout box.
const ctxStub = { clearRect: vi.fn(), drawImage: vi.fn() };
HTMLCanvasElement.prototype.getContext = vi.fn(
  () => ctxStub,
) as unknown as HTMLCanvasElement['getContext'];
HTMLCanvasElement.prototype.getBoundingClientRect = function (): DOMRect {
  return {
    width: 200, height: 100, top: 0, left: 0, right: 200, bottom: 100, x: 0, y: 0,
    toJSON: () => ({}),
  } as DOMRect;
};
```

- [ ] **Step 11: Create `packages/cavimg/src/index.ts`** (stub — replaced in Task 7)

```ts
export const VERSION = '1.0.0';
```

- [ ] **Step 12: Create `packages/cavimg/test/smoke.test.ts`**

```ts
import { describe, it, expect } from 'vitest';
import { VERSION } from '../src/index';

describe('smoke', () => {
  it('runs in a happy-dom environment', () => {
    expect(typeof document).toBe('object');
    expect(typeof HTMLCanvasElement).toBe('function');
  });

  it('exports VERSION', () => {
    expect(VERSION).toBe('1.0.0');
  });
});
```

- [ ] **Step 13: Run the smoke test**

Run: `pnpm --filter cavimg test`
Expected: PASS (2 tests).

- [ ] **Step 14: Verify the build produces the required artifacts**

Run: `pnpm --filter cavimg build`
Expected: `packages/cavimg/dist/` contains `index.js`, `index.global.js`, and `index.d.ts` with no errors.

- [ ] **Step 15: Commit**

```bash
git add -A
git commit -m "chore: scaffold cavimg monorepo, tooling, and test harness"
```

---

### Task 2: `fit.ts` — pure fit geometry

**Files:**
- Create: `packages/cavimg/src/types.ts`, `packages/cavimg/src/fit.ts`
- Test: `packages/cavimg/test/fit.test.ts`

**Interfaces:**
- Produces: `type FitMode = 'contain' | 'cover' | 'fill'`; `interface FitRect { dx: number; dy: number; dw: number; dh: number }`; `computeFit(imgW, imgH, boxW, boxH, mode?: FitMode): FitRect` (mode defaults to `'contain'`).

- [ ] **Step 1: Write the failing test — `packages/cavimg/test/fit.test.ts`**

```ts
import { describe, it, expect } from 'vitest';
import { computeFit } from '../src/fit';

describe('computeFit', () => {
  it('contain: letterboxes a wide image into a square box', () => {
    // 200x100 into 100x100 → scale 0.5 → 100x50 centered vertically
    expect(computeFit(200, 100, 100, 100, 'contain')).toEqual({ dx: 0, dy: 25, dw: 100, dh: 50 });
  });

  it('cover: fills the box and overflows (negative offset)', () => {
    // 200x100 into 100x100 → scale 1 → 200x100, dx -50
    expect(computeFit(200, 100, 100, 100, 'cover')).toEqual({ dx: -50, dy: 0, dw: 200, dh: 100 });
  });

  it('fill: stretches exactly to the box', () => {
    expect(computeFit(200, 100, 100, 100, 'fill')).toEqual({ dx: 0, dy: 0, dw: 100, dh: 100 });
  });

  it('contain upscales a small image', () => {
    expect(computeFit(50, 50, 100, 100, 'contain')).toEqual({ dx: 0, dy: 0, dw: 100, dh: 100 });
  });

  it('defaults to contain', () => {
    expect(computeFit(200, 100, 100, 100)).toEqual(computeFit(200, 100, 100, 100, 'contain'));
  });

  it('returns a zero rect for non-positive dimensions', () => {
    expect(computeFit(0, 100, 100, 100)).toEqual({ dx: 0, dy: 0, dw: 0, dh: 0 });
    expect(computeFit(200, 100, 0, 50)).toEqual({ dx: 0, dy: 0, dw: 0, dh: 0 });
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm --filter cavimg exec vitest run fit`
Expected: FAIL — cannot resolve `../src/fit`.

- [ ] **Step 3: Create `packages/cavimg/src/types.ts`**

```ts
export type FitMode = 'contain' | 'cover' | 'fill';

export interface FitRect {
  dx: number;
  dy: number;
  dw: number;
  dh: number;
}
```

- [ ] **Step 4: Create `packages/cavimg/src/fit.ts`**

```ts
import type { FitMode, FitRect } from './types';

/**
 * Compute the destination rect for drawing an image of size (imgW×imgH) into a
 * box of size (boxW×boxH) under the given fit mode. Pure — no DOM.
 */
export function computeFit(
  imgW: number,
  imgH: number,
  boxW: number,
  boxH: number,
  mode: FitMode = 'contain',
): FitRect {
  if (imgW <= 0 || imgH <= 0 || boxW <= 0 || boxH <= 0) {
    return { dx: 0, dy: 0, dw: 0, dh: 0 };
  }
  if (mode === 'fill') {
    return { dx: 0, dy: 0, dw: boxW, dh: boxH };
  }
  const scale =
    mode === 'cover'
      ? Math.max(boxW / imgW, boxH / imgH)
      : Math.min(boxW / imgW, boxH / imgH);
  const dw = imgW * scale;
  const dh = imgH * scale;
  return { dx: (boxW - dw) / 2, dy: (boxH - dh) / 2, dw, dh };
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `pnpm --filter cavimg exec vitest run fit`
Expected: PASS (6 tests).

- [ ] **Step 6: Commit**

```bash
git add packages/cavimg/src/types.ts packages/cavimg/src/fit.ts packages/cavimg/test/fit.test.ts
git commit -m "feat: add computeFit geometry"
```

---

### Task 3: `loader.ts` — in-memory image decode

**Files:**
- Create: `packages/cavimg/src/loader.ts`
- Test: `packages/cavimg/test/loader.test.ts`

**Interfaces:**
- Consumes: global `Image`, `createImageBitmap` (stubbed in tests via `test/setup.ts`).
- Produces: `loadImageBitmap(url: string): Promise<ImageBitmap>` — sets `crossOrigin='anonymous'`, resolves with the decoded bitmap, discards the URL (`img.src = ''`), and rejects with an Error whose message contains no URL.

- [ ] **Step 1: Write the failing test — `packages/cavimg/test/loader.test.ts`**

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { loadImageBitmap } from '../src/loader';

interface TrackedImage {
  crossOrigin: string;
  src: string;
}
function created(): TrackedImage[] {
  return (globalThis as unknown as { __cavCreatedImages: TrackedImage[] }).__cavCreatedImages;
}

describe('loadImageBitmap', () => {
  beforeEach(() => {
    created().length = 0;
  });

  it('sets crossOrigin=anonymous and resolves with the decoded bitmap', async () => {
    const bmp = await loadImageBitmap('https://cdn.example/a.png');
    expect(created()[0].crossOrigin).toBe('anonymous');
    expect(bmp).toMatchObject({ width: 200, height: 100 });
  });

  it('discards the src after decoding', async () => {
    await loadImageBitmap('https://cdn.example/a.png');
    expect(created()[0].src).toBe('');
  });

  it('rejects on load error without leaking the url in the message', async () => {
    const err = await loadImageBitmap('https://secret.example/bad.png').catch((e: Error) => e);
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).not.toContain('secret.example');
    expect((err as Error).message).not.toContain('bad.png');
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm --filter cavimg exec vitest run loader`
Expected: FAIL — cannot resolve `../src/loader`.

- [ ] **Step 3: Create `packages/cavimg/src/loader.ts`**

```ts
/**
 * Load an image entirely in memory (no <img> mounted into the DOM), decode it
 * to an ImageBitmap, then discard the URL. The url is a local parameter and is
 * never stored. Error messages deliberately omit the url.
 */
export function loadImageBitmap(url: string): Promise<ImageBitmap> {
  return new Promise<ImageBitmap>((resolve, reject) => {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.decoding = 'async';

    const cleanup = (): void => {
      img.onload = null;
      img.onerror = null;
    };

    img.onload = (): void => {
      createImageBitmap(img)
        .then((bitmap) => {
          cleanup();
          img.src = ''; // discard the source
          resolve(bitmap);
        })
        .catch(() => {
          cleanup();
          reject(new Error('cavimg: failed to decode image'));
        });
    };

    img.onerror = (): void => {
      cleanup();
      reject(new Error('cavimg: failed to load image'));
    };

    img.src = url;
  });
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `pnpm --filter cavimg exec vitest run loader`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add packages/cavimg/src/loader.ts packages/cavimg/test/loader.test.ts
git commit -m "feat: add in-memory image loader"
```

---

### Task 4: `render.ts` — draw bitmap to canvas

**Files:**
- Create: `packages/cavimg/src/render.ts`
- Test: `packages/cavimg/test/render.test.ts`

**Interfaces:**
- Consumes: `computeFit` from `./fit`; `FitMode` from `./types`.
- Produces: `interface DrawParams { canvas: HTMLCanvasElement; bitmap: ImageBitmap; fit?: FitMode; dpr?: number }`; `drawBitmap(params: DrawParams): void` — sizes the backing store to `cssBox × dpr`, clears, and draws with the computed fit rect.

- [ ] **Step 1: Write the failing test — `packages/cavimg/test/render.test.ts`**

```ts
import { describe, it, expect, vi } from 'vitest';
import { drawBitmap } from '../src/render';

function makeCanvas(cssW: number, cssH: number) {
  const canvas = document.createElement('canvas');
  const ctx = { clearRect: vi.fn(), drawImage: vi.fn() };
  // Instance overrides shadow the prototype stubs from setup.ts.
  canvas.getContext = vi.fn(() => ctx) as unknown as HTMLCanvasElement['getContext'];
  canvas.getBoundingClientRect = (() => ({
    width: cssW, height: cssH, top: 0, left: 0, right: cssW, bottom: cssH, x: 0, y: 0,
    toJSON: () => ({}),
  })) as unknown as HTMLCanvasElement['getBoundingClientRect'];
  return { canvas, ctx };
}

const bitmap = { width: 200, height: 100 } as ImageBitmap;

describe('drawBitmap', () => {
  it('sizes the backing store to cssBox × dpr', () => {
    const { canvas } = makeCanvas(100, 50);
    drawBitmap({ canvas, bitmap, fit: 'contain', dpr: 2 });
    expect(canvas.width).toBe(200);
    expect(canvas.height).toBe(100);
  });

  it('clears then draws with the contain fit rect', () => {
    const { canvas, ctx } = makeCanvas(200, 100);
    drawBitmap({ canvas, bitmap, fit: 'contain', dpr: 1 });
    expect(ctx.clearRect).toHaveBeenCalledWith(0, 0, 200, 100);
    // 200x100 bitmap into a 200x100 box → 1:1
    expect(ctx.drawImage).toHaveBeenCalledWith(bitmap, 0, 0, 200, 100);
  });

  it('does not throw when getContext returns null', () => {
    const { canvas } = makeCanvas(100, 100);
    canvas.getContext = (() => null) as unknown as HTMLCanvasElement['getContext'];
    expect(() => drawBitmap({ canvas, bitmap })).not.toThrow();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm --filter cavimg exec vitest run render`
Expected: FAIL — cannot resolve `../src/render`.

- [ ] **Step 3: Create `packages/cavimg/src/render.ts`**

```ts
import { computeFit } from './fit';
import type { FitMode } from './types';

export interface DrawParams {
  canvas: HTMLCanvasElement;
  bitmap: ImageBitmap;
  fit?: FitMode;
  dpr?: number;
}

export function drawBitmap(params: DrawParams): void {
  const { canvas, bitmap, fit = 'contain', dpr = 1 } = params;

  const rect = canvas.getBoundingClientRect();
  const cssW = rect.width || bitmap.width;
  const cssH = rect.height || bitmap.height;
  const pixelW = Math.max(1, Math.round(cssW * dpr));
  const pixelH = Math.max(1, Math.round(cssH * dpr));

  if (canvas.width !== pixelW) canvas.width = pixelW;
  if (canvas.height !== pixelH) canvas.height = pixelH;

  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  ctx.clearRect(0, 0, pixelW, pixelH);
  const f = computeFit(bitmap.width, bitmap.height, pixelW, pixelH, fit);
  ctx.drawImage(bitmap, f.dx, f.dy, f.dw, f.dh);
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `pnpm --filter cavimg exec vitest run render`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add packages/cavimg/src/render.ts packages/cavimg/test/render.test.ts
git commit -m "feat: add canvas draw with DPR + fit"
```

---

### Task 5: `protect.ts` — anti-copy wiring

**Files:**
- Create: `packages/cavimg/src/protect.ts`
- Test: `packages/cavimg/test/protect.test.ts`

**Interfaces:**
- Produces: `harden(canvas: HTMLCanvasElement): () => void` — sets `draggable=false`, prevents `dragstart` and `contextmenu`, applies no-select/no-drag CSS, and returns a cleanup function that removes the listeners.

- [ ] **Step 1: Write the failing test — `packages/cavimg/test/protect.test.ts`**

```ts
import { describe, it, expect } from 'vitest';
import { harden } from '../src/protect';

function fire(el: EventTarget, type: string): Event {
  const ev = new Event(type, { cancelable: true, bubbles: true });
  el.dispatchEvent(ev);
  return ev;
}

describe('harden', () => {
  it('marks the canvas non-draggable', () => {
    const canvas = document.createElement('canvas');
    harden(canvas);
    expect(canvas.getAttribute('draggable')).toBe('false');
  });

  it('prevents contextmenu and dragstart', () => {
    const canvas = document.createElement('canvas');
    harden(canvas);
    expect(fire(canvas, 'contextmenu').defaultPrevented).toBe(true);
    expect(fire(canvas, 'dragstart').defaultPrevented).toBe(true);
  });

  it('cleanup removes the listeners', () => {
    const canvas = document.createElement('canvas');
    const cleanup = harden(canvas);
    cleanup();
    expect(fire(canvas, 'contextmenu').defaultPrevented).toBe(false);
    expect(fire(canvas, 'dragstart').defaultPrevented).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm --filter cavimg exec vitest run protect`
Expected: FAIL — cannot resolve `../src/protect`.

- [ ] **Step 3: Create `packages/cavimg/src/protect.ts`**

```ts
/**
 * Apply casual anti-copy hardening to the canvas: block drag-to-desktop and the
 * right-click "Save image as…" menu, and disable text/image selection. Returns
 * a cleanup that detaches the listeners.
 */
export function harden(canvas: HTMLCanvasElement): () => void {
  canvas.draggable = false;
  canvas.setAttribute('draggable', 'false');

  const prevent = (event: Event): void => event.preventDefault();
  canvas.addEventListener('dragstart', prevent);
  canvas.addEventListener('contextmenu', prevent);

  const style = canvas.style;
  style.setProperty('user-select', 'none');
  style.setProperty('-webkit-user-select', 'none');
  style.setProperty('-webkit-user-drag', 'none');
  style.setProperty('-webkit-touch-callout', 'none');

  return (): void => {
    canvas.removeEventListener('dragstart', prevent);
    canvas.removeEventListener('contextmenu', prevent);
  };
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `pnpm --filter cavimg exec vitest run protect`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add packages/cavimg/src/protect.ts packages/cavimg/test/protect.test.ts
git commit -m "feat: add anti-copy hardening"
```

---

### Task 6: `cav-img-element.ts` — the `<cav-img>` custom element

**Files:**
- Create: `packages/cavimg/src/cav-img-element.ts`
- Test: `packages/cavimg/test/element.test.ts`

**Interfaces:**
- Consumes: `loadImageBitmap` (`./loader`), `drawBitmap` (`./render`), `harden` (`./protect`), `FitMode` (`./types`).
- Produces: `class CavImgElement extends HTMLElement` with `static observedAttributes` = `['src','fit','alt']`; `get/set src` (getter returns retained `#url`), `get/set fit`, `load(url): void`. Dispatches `cav-load`/`cav-error`. Retains `#url` + bitmap across disconnect. Scrubs the `src` attribute after load. Sets host `aspect-ratio` on load.

- [ ] **Step 1: Write the failing test — `packages/cavimg/test/element.test.ts`**

```ts
import { describe, it, expect, beforeAll, afterEach } from 'vitest';
import { CavImgElement } from '../src/cav-img-element';

beforeAll(() => {
  if (!customElements.get('cav-img')) customElements.define('cav-img', CavImgElement);
});
afterEach(() => {
  document.body.innerHTML = '';
});

function mount(attrs: Record<string, string> = {}): CavImgElement {
  const el = document.createElement('cav-img') as CavImgElement;
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  document.body.append(el);
  return el;
}
function once(el: EventTarget, type: string): Promise<void> {
  return new Promise((resolve) => el.addEventListener(type, () => resolve(), { once: true }));
}
const tick = (): Promise<void> =>
  new Promise((r) => queueMicrotask(() => queueMicrotask(() => r())));

function bitmapCalls(): number {
  return (globalThis.createImageBitmap as unknown as { mock: { calls: unknown[] } }).mock.calls.length;
}

describe('CavImgElement', () => {
  it('renders a canvas in the shadow root and mounts no <img>', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    expect(el.shadowRoot?.querySelector('canvas')).toBeTruthy();
    expect(el.shadowRoot?.querySelector('img')).toBeNull();
    expect(el.querySelector('img')).toBeNull();
  });

  it('scrubs the src attribute but keeps get src()', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    expect(el.hasAttribute('src')).toBe(false);
    expect(el.src).toBe('https://x/a.png');
  });

  it('sets role=img and aria-label from alt', async () => {
    const el = mount({ src: 'https://x/a.png', alt: 'a kitten' });
    await once(el, 'cav-load');
    expect(el.getAttribute('role')).toBe('img');
    expect(el.getAttribute('aria-label')).toBe('a kitten');
  });

  it('sets host aspect-ratio from the bitmap natural size', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    expect(el.style.aspectRatio.replace(/\s+/g, '')).toBe('200/100');
  });

  it('emits cav-error on a failed load', async () => {
    const el = mount();
    const errored = once(el, 'cav-error');
    el.setAttribute('src', 'https://x/bad.png');
    await errored;
    expect(el.shadowRoot?.querySelector('canvas')).toBeTruthy();
  });

  it('loads via the src property setter without touching the DOM attribute', async () => {
    const el = document.createElement('cav-img') as CavImgElement;
    document.body.append(el);
    el.src = 'https://x/b.png';
    await once(el, 'cav-load');
    expect(el.hasAttribute('src')).toBe(false);
    expect(el.src).toBe('https://x/b.png');
  });

  it('survives disconnect/reconnect without refetching', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    const before = bitmapCalls();
    el.remove();
    document.body.append(el);
    await tick();
    expect(bitmapCalls()).toBe(before);
    expect(el.shadowRoot?.querySelector('canvas')).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm --filter cavimg exec vitest run element`
Expected: FAIL — cannot resolve `../src/cav-img-element`.

- [ ] **Step 3: Create `packages/cavimg/src/cav-img-element.ts`**

```ts
import { loadImageBitmap } from './loader';
import { drawBitmap } from './render';
import { harden } from './protect';
import type { FitMode } from './types';

const FIT_MODES: readonly FitMode[] = ['contain', 'cover', 'fill'];
const SHADOW_CSS = ':host{display:block}canvas{display:block;width:100%;height:100%}';

export class CavImgElement extends HTMLElement {
  static get observedAttributes(): string[] {
    return ['src', 'fit', 'alt'];
  }

  #canvas: HTMLCanvasElement;
  #url: string | null = null;
  #bitmap: ImageBitmap | null = null;
  #ro: ResizeObserver | null = null;
  #unharden: (() => void) | null = null;
  #connected = false;
  #token = 0;

  constructor() {
    super();
    const root = this.attachShadow({ mode: 'open' });
    const style = document.createElement('style');
    style.textContent = SHADOW_CSS;
    this.#canvas = document.createElement('canvas');
    root.append(style, this.#canvas);
    this.setAttribute('role', 'img');
  }

  get src(): string | null {
    return this.#url ?? this.getAttribute('src');
  }
  set src(value: string | null) {
    if (value == null || value === '') {
      this.#url = null;
      this.removeAttribute('src');
      return;
    }
    this.#loadFrom(value);
  }

  get fit(): FitMode {
    const value = this.getAttribute('fit');
    return FIT_MODES.includes(value as FitMode) ? (value as FitMode) : 'contain';
  }
  set fit(value: FitMode) {
    this.setAttribute('fit', value);
  }

  load(url: string): void {
    this.#loadFrom(url);
  }

  connectedCallback(): void {
    this.#connected = true;
    this.#unharden = harden(this.#canvas);
    this.#ro = new ResizeObserver(() => this.#redraw());
    this.#ro.observe(this);

    const alt = this.getAttribute('alt');
    if (alt != null) this.setAttribute('aria-label', alt);

    if (this.#bitmap) {
      this.#redraw();
    } else if (this.#url) {
      this.#loadFrom(this.#url);
    } else {
      const attr = this.getAttribute('src');
      if (attr) this.#loadFrom(attr);
    }
  }

  disconnectedCallback(): void {
    this.#connected = false;
    this.#ro?.disconnect();
    this.#ro = null;
    this.#unharden?.();
    this.#unharden = null;
    // Intentionally retain #url and #bitmap so reconnect redraws without a refetch.
  }

  attributeChangedCallback(name: string, _old: string | null, value: string | null): void {
    if (!this.#connected) return; // ignore initial-parse attribute sets; connectedCallback handles them
    if (name === 'src') {
      if (value) this.#loadFrom(value);
    } else if (name === 'fit') {
      this.#redraw();
    } else if (name === 'alt') {
      this.setAttribute('aria-label', value ?? '');
    }
  }

  #loadFrom(url: string): void {
    this.#url = url;
    const token = ++this.#token;
    loadImageBitmap(url)
      .then((bitmap) => {
        if (token !== this.#token) {
          bitmap.close?.();
          return;
        }
        this.#bitmap = bitmap;
        this.removeAttribute('src'); // scrub the URL from the DOM; #url still holds it
        this.style.aspectRatio = `${bitmap.width} / ${bitmap.height}`;
        this.#redraw();
        this.dispatchEvent(new CustomEvent('cav-load'));
      })
      .catch(() => {
        this.dispatchEvent(new CustomEvent('cav-error'));
      });
  }

  #redraw(): void {
    if (!this.#bitmap) return;
    drawBitmap({
      canvas: this.#canvas,
      bitmap: this.#bitmap,
      fit: this.fit,
      dpr: (typeof window !== 'undefined' && window.devicePixelRatio) || 1,
    });
  }
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `pnpm --filter cavimg exec vitest run element`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add packages/cavimg/src/cav-img-element.ts packages/cavimg/test/element.test.ts
git commit -m "feat: add cav-img custom element"
```

---

### Task 7: `index.ts` — registration, functional API, exports

**Files:**
- Modify: `packages/cavimg/src/index.ts` (replace the Task 1 stub)
- Test: `packages/cavimg/test/index.test.ts`

**Interfaces:**
- Consumes: `CavImgElement`, `loadImageBitmap`, `drawBitmap`, `computeFit`, `harden`, types.
- Produces: `defineCavImg(tag?: string): void` (idempotent, SSR-guarded); `renderToCanvas(canvas, url, opts?): Promise<void>`; re-exports `CavImgElement`, `computeFit`, `drawBitmap`, `loadImageBitmap`, `harden`, `FitMode`, `FitRect`. Auto-registers `cav-img` on import.

- [ ] **Step 1: Write the failing test — `packages/cavimg/test/index.test.ts`**

```ts
import { describe, it, expect } from 'vitest';
import { defineCavImg, renderToCanvas, CavImgElement } from '../src/index';

describe('index', () => {
  it('auto-registers cav-img on import', () => {
    expect(customElements.get('cav-img')).toBe(CavImgElement);
  });

  it('defineCavImg is idempotent', () => {
    expect(() => {
      defineCavImg();
      defineCavImg();
    }).not.toThrow();
  });

  it('renderToCanvas draws a decoded image into the given canvas', async () => {
    const canvas = document.createElement('canvas');
    await expect(renderToCanvas(canvas, 'https://x/a.png')).resolves.toBeUndefined();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm --filter cavimg exec vitest run test/index.test.ts`
Expected: FAIL — `defineCavImg`/`renderToCanvas` are not exported (stub only exports `VERSION`).

- [ ] **Step 3: Replace `packages/cavimg/src/index.ts`**

```ts
import { CavImgElement } from './cav-img-element';
import { loadImageBitmap } from './loader';
import { drawBitmap } from './render';
import type { FitMode } from './types';

export { CavImgElement } from './cav-img-element';
export { computeFit } from './fit';
export { drawBitmap } from './render';
export { loadImageBitmap } from './loader';
export { harden } from './protect';
export type { FitMode, FitRect } from './types';

export const VERSION = '1.0.0';

/** Register the <cav-img> element. Idempotent and safe on the server (no-ops when customElements is absent). */
export function defineCavImg(tag = 'cav-img'): void {
  if (typeof customElements === 'undefined') return;
  if (customElements.get(tag)) return;
  try {
    customElements.define(tag, CavImgElement);
  } catch {
    // The class is already registered under another tag; ignore.
  }
}

/** Functional escape hatch: render a URL into a caller-owned canvas. */
export async function renderToCanvas(
  canvas: HTMLCanvasElement,
  url: string,
  opts: { fit?: FitMode } = {},
): Promise<void> {
  const bitmap = await loadImageBitmap(url);
  drawBitmap({
    canvas,
    bitmap,
    fit: opts.fit ?? 'contain',
    dpr: (typeof window !== 'undefined' && window.devicePixelRatio) || 1,
  });
}

// Auto-register on import (guarded for SSR).
defineCavImg();
```

- [ ] **Step 4: Run the index test**

Run: `pnpm --filter cavimg exec vitest run test/index.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Run the full suite + typecheck + build**

Run: `pnpm --filter cavimg test && pnpm --filter cavimg typecheck && pnpm --filter cavimg build`
Expected: all unit tests pass; `tsc --noEmit` clean; `dist/` has `index.js`, `index.global.js`, `index.d.ts`.

- [ ] **Step 6: Verify the built artifacts expose the API**

Run: `node -e "import('./packages/cavimg/dist/index.js').then(m => console.log(typeof m.defineCavImg, typeof m.renderToCanvas, typeof m.CavImgElement))"`
Expected: `function function function`

- [ ] **Step 7: Commit**

```bash
git add packages/cavimg/src/index.ts packages/cavimg/test/index.test.ts
git commit -m "feat: add registration, renderToCanvas, and public exports"
```

---

## Self-Review

**Spec coverage:**
- §4 layout → Task 1 (monorepo, packages/cavimg). ✅
- §5 modules fit/loader/render/protect/element/index → Tasks 2–7. ✅
- §6 API (`<cav-img src fit alt>`, `el.src/fit/load`, `cav-load`/`cav-error`, `renderToCanvas`) → Tasks 6–7. ✅
- §7 data flow (retain #url + bitmap, scrub attribute, aspect-ratio, no refetch on reconnect) → Task 6 tests. ✅
- §9 build (tsup ESM + IIFE `Cavimg` + dts) → Task 1 config, Task 7 verify. ✅
- §10 unit tests (fit math incl. edge cases, loader scrub + crossOrigin, reflection, event blocking) → Tasks 2–6. ✅
- §13 decisions (contain default, open shadow, host aspect-ratio, `#url`, ResizeObserver, tsup) → Tasks 1/6. ✅
- Deferred to Plan 2: example apps, Playwright E2E, `/images`, README, threat-matrix. Explicitly out of scope here.

**Placeholder scan:** No TBD/TODO; every code step contains full code; every command has expected output.

**Type consistency:** `computeFit(imgW,imgH,boxW,boxH,mode?)→FitRect`, `loadImageBitmap(url)→Promise<ImageBitmap>`, `drawBitmap(DrawParams)→void`, `harden(canvas)→()=>void`, `CavImgElement`, `defineCavImg(tag?)→void`, `renderToCanvas(canvas,url,opts?)→Promise<void>` — names/signatures match across the tasks that produce and consume them. Event names `cav-load`/`cav-error` consistent between Task 6 and its tests.

---

## Implementation deviations (recorded during execution)

Corrections applied during subagent-driven execution; the shipped code is authoritative, not the reference blocks above.

1. **Toolchain (Task 1):** `typescript` pinned to `^5.9.3`. `typescript@7.x` (native rewrite) crashes tsup's bundled `rollup-plugin-dts`; `6.x` errors on the deprecated `baseUrl` it injects. `5.9.3` is the last stable line that builds cleanly.
2. **Element robustness (Task 6, fix commit `34f1d27`):** the reference `#loadFrom`/`connectedCallback` had two real defects, fixed with regression tests:
   - The `.catch()` path lacked the stale-token guard, so a superseded-then-failed load fired a spurious `cav-error`. Added `if (token !== this.#token) return;` in `.catch()`.
   - Setting `.src` before the element is appended started a load, then `connectedCallback` (seeing `#url` set but no bitmap) started a second — a duplicate fetch under the common "configure then append" pattern. Added a `#pending` flag; `connectedCallback` skips the redundant load while one is in flight.
   - Minor: `connectedCallback` now clears a stale `aria-label` when `alt` is absent.
3. **Task 7 Step 6 verification:** the bare-`node` dynamic-import check cannot run — `class CavImgElement extends HTMLElement` can't be evaluated in Node (no `HTMLElement` global). The module is browser-only by design (SSR handled via dynamic import per §8). Replaced with an export-surface check on the built `dist/index.d.ts` + IIFE `Cavimg` global; runtime correctness is covered by the happy-dom `index.test.ts`. **Plan 2 note:** if universal SSR-safe top-level import is wanted, guard the base class (`const Base = typeof HTMLElement !== 'undefined' ? HTMLElement : class{}`) — deferred, to be validated against the real Next.js example.
