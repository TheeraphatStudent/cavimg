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
