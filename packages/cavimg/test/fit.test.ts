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
