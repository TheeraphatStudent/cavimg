# cavimg

Render an image to a `<canvas>` so it is harder to **casually** copy — no `<img>` to
right-click-save, no drag-to-desktop, and the image URL kept out of the DOM
inspector. Framework-agnostic Web Component (`<cav-img>`). **Casual deterrence only.**

![cavimg — a plain <img> can be saved and dragged; a <cav-img> can't](https://raw.githubusercontent.com/TheeraphatStudent/cavimg/main/images/showcase.gif)

**A plain `<img>` exposes its URL in DevTools; `<cav-img>` shows only a canvas with no `src`:**

| Plain `<img>` — URL exposed | `<cav-img>` — URL hidden |
|:---:|:---:|
| ![](https://raw.githubusercontent.com/TheeraphatStudent/cavimg/main/images/usage-img.png) | ![](https://raw.githubusercontent.com/TheeraphatStudent/cavimg/main/images/usage-cavimg.png) |

## Install
```bash
npm i cavimg
```
CDN: `<script src="https://cdn.jsdelivr.net/npm/cavimg"></script>` (registers `<cav-img>`).

## Quick start
- **HTML / Vite:** `import 'cavimg'` then `<cav-img src="…" alt="…"></cav-img>`
- **Next.js:** `useEffect(() => { import('cavimg').then(m => m.defineCavImg()); }, [])` (Web Components are client-only).
- **Angular:** add `CUSTOM_ELEMENTS_SCHEMA`, call `defineCavImg()`.

## API
- **Attributes:** `src`, `fit` = `contain` (default) \| `cover` \| `fill`, `alt`.
- **Properties:** `el.src` (get/set), `el.fit`, `el.load(url)`.
- **Events:** `cav-load`, `cav-error`.
- **Functional:** `renderToCanvas(canvas, url, { fit })`, `defineCavImg(tag?)`.

## What it stops (and what it doesn't)
Blocks right-click *Save image as…*, drag-to-desktop, and hides the URL from the DOM
inspector. It does **not** stop the Network tab, `canvas.toDataURL()`, screenshots, or
headless scraping — no client-side canvas library can. Use it to discourage casual
reuse, not to protect genuinely sensitive images.

## 📖 Full documentation
Live demo, per-framework guides, performance analysis, and the complete client-side
copy-vector **threat matrix**:

**https://github.com/TheeraphatStudent/cavimg**

## License
MIT © th33raphat
