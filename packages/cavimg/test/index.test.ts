import { describe, it, expect } from 'vitest';
import { defineCavImg, renderToCanvas, CavImgElement } from '../src/index';

describe('index', () => {
  it('auto-registers cav-img on import', () => {
    expect(customElements.get('cav-img')).toBe(CavImgElement);
  });

  it('defineCavImg is idempotent', () => {
    expect(() => {
      defineCavImg();
      defineCavImg();
    }).not.toThrow();
  });

  it('renderToCanvas draws a decoded image into the given canvas', async () => {
    const canvas = document.createElement('canvas');
    await expect(renderToCanvas(canvas, 'https://x/a.png')).resolves.toBeUndefined();
  });
});
