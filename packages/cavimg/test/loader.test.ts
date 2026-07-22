import { describe, it, expect, beforeEach } from 'vitest';
import { loadImageBitmap } from '../src/loader';

interface TrackedImage {
  crossOrigin: string;
  src: string;
}
function created(): TrackedImage[] {
  return (globalThis as unknown as { __cavCreatedImages: TrackedImage[] }).__cavCreatedImages;
}

describe('loadImageBitmap', () => {
  beforeEach(() => {
    created().length = 0;
  });

  it('sets crossOrigin=anonymous and resolves with the decoded bitmap', async () => {
    const bmp = await loadImageBitmap('https://cdn.example/a.png');
    expect(created()[0].crossOrigin).toBe('anonymous');
    expect(bmp).toMatchObject({ width: 200, height: 100 });
  });

  it('discards the src after decoding', async () => {
    await loadImageBitmap('https://cdn.example/a.png');
    expect(created()[0].src).toBe('');
  });

  it('rejects on load error without leaking the url in the message', async () => {
    const err = await loadImageBitmap('https://secret.example/bad.png').catch((e: Error) => e);
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).not.toContain('secret.example');
    expect((err as Error).message).not.toContain('bad.png');
  });
});
