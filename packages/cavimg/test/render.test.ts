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
