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
