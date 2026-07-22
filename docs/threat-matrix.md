# cavimg — Client-Side Copy Threat Matrix

`cavimg` renders an image to a `<canvas>` to make it harder to **casually** copy. It
is not real protection. This table is the honest, exhaustive accounting of how a
user can still obtain the image, and what cavimg does about each vector.

Legend: **Blocks** = cavimg stops the casual path · **Raises bar** = harder but
doable · **Cannot** = out of reach of any client-side canvas approach.

| # | Copy vector | cavimg | Notes |
|---|-------------|--------|-------|
| 1 | Right-click "Save image as…" | **Blocks** | `contextmenu` preventDefault on the canvas; there is no `<img>` to save. |
| 2 | Drag image to desktop/another app | **Blocks** | `draggable=false` + `dragstart` preventDefault. |
| 3 | `<img>` src / URL in the Elements panel | **Blocks** | No `<img>` is mounted; the `src` attribute is scrubbed after load. |
| 4 | Copy image (browser context action) | **Blocks** | Same as #1 — no context menu, no `<img>`. |
| 5 | View Source / "Save page as" | **Raises bar** | The URL isn't in the served HTML, but it is fetched at runtime (see #6). |
| 6 | **Network tab** (raw file over the wire) | **Cannot** | The browser must fetch the bytes; the URL and response are visible. Fundamental. |
| 7 | `canvas.toDataURL()` / `toBlob()` in the console | **Cannot** | The pixels are in the canvas by design; any script can read them (image is same-origin/CORS-clean). |
| 8 | OS / browser **screenshot** & screen recording | **Cannot** | Outside the page's control entirely. |
| 9 | Print / "Save as PDF" | **Raises bar** | The canvas prints; quality/reflow varies, but the pixels are obtainable. |
| 10 | Image-downloader browser extensions | **Raises bar** | Extensions that scan `<img>` miss it; ones that read canvas or the Network tab do not. |
| 11 | JavaScript disabled | **Raises bar** | With JS off, nothing renders (no image shown at all) — content is gone, not stolen. |
| 12 | Accessibility tree / screen readers | **Blocks (as image)** | Exposed as `role="img"` + `aria-label`; no pixel data via a11y APIs. |
| 13 | CSS `background-image` sniffing | **Blocks** | cavimg uses no CSS background image. |
| 14 | Headless scraping / bots | **Cannot** | A headless browser fetches the URL (#6) or reads the canvas (#7) like any script. |
| 15 | `getContext('2d').getImageData()` pixel read | **Cannot** | The canvas is not tainted (CORS-clean by design), so pixels are readable. |

## Bottom line
cavimg reliably defeats the **casual** paths (#1–4, #12–13) and adds friction to a
few more (#5, #9–11). It **cannot** defeat the Network tab, canvas export,
screenshots, or headless scraping (#6–8, #14–15) — no client-side canvas library
can. Use it to discourage casual reuse, never to protect genuinely sensitive images.

## CORS caveat
cavimg sets `crossOrigin="anonymous"` so the canvas stays exportable/untainted and
CORS-clean. A consequence: images served from hosts **without** permissive CORS
headers fail to load and emit `cav-error`. Host your images with CORS enabled (or
same-origin).
