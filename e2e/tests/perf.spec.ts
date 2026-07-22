import { test, expect } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const APP = process.env.CAVIMG_E2E_APP ?? 'vite';
const IMAGES = fileURLToPath(new URL('../../images/', import.meta.url));

test('captures load-time and memory impact of cav-img vs a plain img', async ({ page }) => {
  mkdirSync(IMAGES, { recursive: true });
  await page.goto('/');

  // the app exposes window.__perf = { plainMs, cavMs, heap? } once both have loaded
  const perf = await page.waitForFunction(() => {
    const p = window.__perf;
    return p && typeof p.plainMs === 'number' && typeof p.cavMs === 'number' ? p : null;
  }, { timeout: 30_000 }).then((h) => h.jsonValue());

  expect(perf.plainMs).toBeGreaterThanOrEqual(0);
  expect(perf.cavMs).toBeGreaterThanOrEqual(0);

  const out = {
    app: APP,
    plainMs: Math.round(perf.plainMs),
    cavMs: Math.round(perf.cavMs),
    overheadMs: Math.round(perf.cavMs - perf.plainMs),
    heapBytes: perf.heap ?? null,
  };
  writeFileSync(`${IMAGES}perf-${APP}.json`, JSON.stringify(out, null, 2));
});
