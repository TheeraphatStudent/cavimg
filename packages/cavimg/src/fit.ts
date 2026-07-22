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
