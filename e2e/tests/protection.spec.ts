import { test, expect } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const APP = process.env.CAVIMG_E2E_APP ?? 'vite';
const IMAGES = fileURLToPath(new URL('../../images/', import.meta.url));

test('protected image renders to canvas, hides the URL, and blocks drag/context menu', async ({ page }) => {
  mkdirSync(IMAGES, { recursive: true });
  await page.goto('/');

  const cav = page.locator('cav-img[data-test="protected"]');
  const plain = page.locator('img[data-test="plain"]');
  await expect(cav).toBeVisible();
  await expect(plain).toBeVisible();

  // canvas drawn in the shadow root
  await page.waitForFunction(() => {
    const el = document.querySelector('cav-img[data-test="protected"]');
    const c = el?.shadowRoot?.querySelector('canvas');
    return !!c && c.width > 0;
  }, { timeout: 30_000 });

  // no <img> anywhere inside cav-img (light or shadow)
  const imgInsideCav = await cav.evaluate(
    (el) => (el.shadowRoot?.querySelectorAll('img').length ?? 0) + el.querySelectorAll('img').length,
  );
  expect(imgInsideCav).toBe(0);

  // src attribute scrubbed from the DOM after load
  await expect.poll(() => cav.evaluate((el) => el.hasAttribute('src'))).toBe(false);

  // drag + context menu prevented on the canvas
  const blocked = await cav.evaluate((el) => {
    const c = el.shadowRoot.querySelector('canvas');
    const ctx = new Event('contextmenu', { cancelable: true, bubbles: true });
    const drag = new Event('dragstart', { cancelable: true, bubbles: true });
    c.dispatchEvent(ctx);
    c.dispatchEvent(drag);
    return { ctx: ctx.defaultPrevented, drag: drag.defaultPrevented };
  });
  expect(blocked.ctx).toBe(true);
  expect(blocked.drag).toBe(true);

  // DOM evidence for the README (the whole point: identical pixels, different DOM)
  const dom = await page.evaluate(() => {
    const p = document.querySelector('img[data-test="plain"]');
    const c = document.querySelector('cav-img[data-test="protected"]');
    return {
      plainOuter: p.outerHTML,
      protectedOuter: c.outerHTML,
      protectedHasImg: !!c.querySelector('img') || !!c.shadowRoot?.querySelector('img'),
      protectedHasSrcAttr: c.hasAttribute('src'),
    };
  });
  writeFileSync(`${IMAGES}dom-${APP}.json`, JSON.stringify(dom, null, 2));

  // before/after screenshots (visually identical — that is the point)
  await plain.screenshot({ path: `${IMAGES}before-${APP}.png` });
  await cav.screenshot({ path: `${IMAGES}after-${APP}.png` });
});
