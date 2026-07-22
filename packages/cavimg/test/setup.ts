import { vi } from 'vitest';

// ResizeObserver is not needed to fire in unit tests — a no-op stub is enough.
class ResizeObserverStub {
  constructor(_cb: ResizeObserverCallback) {}
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverStub);

// Track every Image the code under test creates so loader tests can assert on it.
interface TrackedImage {
  crossOrigin: string;
  decoding: string;
  onload: (() => void) | null;
  onerror: (() => void) | null;
  src: string;
}
const createdImages: TrackedImage[] = [];
(globalThis as unknown as { __cavCreatedImages: TrackedImage[] }).__cavCreatedImages = createdImages;

class ImageStub implements TrackedImage {
  crossOrigin = '';
  decoding = '';
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  private _src = '';
  constructor() {
    createdImages.push(this);
  }
  set src(value: string) {
    this._src = value;
    if (!value) return; // empty assignment = discard, do not "load"
    queueMicrotask(() => {
      if (value.includes('bad')) this.onerror?.();
      else this.onload?.();
    });
  }
  get src(): string {
    return this._src;
  }
}
vi.stubGlobal('Image', ImageStub);

vi.stubGlobal(
  'createImageBitmap',
  vi.fn(async () => ({ width: 200, height: 100, close: vi.fn() }) as unknown as ImageBitmap),
);

// happy-dom has no real 2D canvas context; stub getContext + layout box.
const ctxStub = { clearRect: vi.fn(), drawImage: vi.fn() };
HTMLCanvasElement.prototype.getContext = vi.fn(
  () => ctxStub,
) as unknown as HTMLCanvasElement['getContext'];
HTMLCanvasElement.prototype.getBoundingClientRect = function (): DOMRect {
  return {
    width: 200, height: 100, top: 0, left: 0, right: 200, bottom: 100, x: 0, y: 0,
    toJSON: () => ({}),
  } as DOMRect;
};
