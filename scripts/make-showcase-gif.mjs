import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';

// gifenc, pngjs, and @playwright/test are devDependencies of the `e2e`
// workspace package, not of this `scripts/` directory or the repo root, so
// under pnpm's (non-hoisted) node_modules layout a plain `import '...'`
// from this file's own path cannot find them. Anchor module resolution at
// the e2e package directory instead, where pnpm actually links them.
const require = createRequire(new URL('../e2e/package.json', import.meta.url));
const { chromium } = require('@playwright/test');
const { GIFEncoder, quantize, applyPalette } = require('gifenc');
const { PNG } = require('pngjs');

// Named PAGE_URL (not URL) so it doesn't shadow the global URL constructor
// used above (for createRequire) and below (for OUT) in this same module scope.
const PAGE_URL = 'http://localhost:5173/showcase.html';
const OUT = fileURLToPath(new URL('../images/showcase.gif', import.meta.url));
const FRAMES = 24;
const CLIP = { x: 0, y: 0, width: 880, height: 360 };

function waitForServer(url, timeoutMs = 60000) {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    const tick = async () => {
      try {
        const r = await fetch(url);
        if (r.ok) return resolve();
      } catch { /* not up yet */ }
      if (Date.now() - start > timeoutMs) return reject(new Error('vite dev server did not start'));
      setTimeout(tick, 500);
    };
    tick();
  });
}

const server = spawn('pnpm', ['--filter', '@cavimg/example-vite', 'dev'], {
  stdio: 'ignore', shell: process.platform === 'win32',
});
try {
  await waitForServer(PAGE_URL);
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 900, height: 380 }, deviceScaleFactor: 1 });
  await page.goto(PAGE_URL, { waitUntil: 'load' });
  await page.waitForFunction(() => window.__ready === true, { timeout: 30000 });

  const gif = GIFEncoder();
  for (let n = 0; n < FRAMES; n++) {
    await page.evaluate((i) => window.__frame(i), n);
    await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
    const buf = await page.screenshot({ clip: CLIP });
    const png = PNG.sync.read(buf);
    const rgba = new Uint8Array(png.data.buffer, png.data.byteOffset, png.data.length);
    const palette = quantize(rgba, 256);
    const index = applyPalette(rgba, palette);
    gif.writeFrame(index, png.width, png.height, { palette, delay: Math.round(3000 / FRAMES), repeat: n === 0 ? 0 : undefined });
  }
  gif.finish();
  writeFileSync(OUT, Buffer.from(gif.bytes()));
  console.log(`wrote ${OUT} (${gif.bytes().length} bytes, ${FRAMES} frames)`);
  await browser.close();
} finally {
  server.kill();
}
