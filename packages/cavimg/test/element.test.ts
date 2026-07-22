import { describe, it, expect, beforeAll, afterEach } from 'vitest';
import { CavImgElement } from '../src/cav-img-element';

beforeAll(() => {
  if (!customElements.get('cav-img')) customElements.define('cav-img', CavImgElement);
});
afterEach(() => {
  document.body.innerHTML = '';
});

function mount(attrs: Record<string, string> = {}): CavImgElement {
  const el = document.createElement('cav-img') as CavImgElement;
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  document.body.append(el);
  return el;
}
function once(el: EventTarget, type: string): Promise<void> {
  return new Promise((resolve) => el.addEventListener(type, () => resolve(), { once: true }));
}
const tick = (): Promise<void> =>
  new Promise((r) => queueMicrotask(() => queueMicrotask(() => r())));

function bitmapCalls(): number {
  return (globalThis.createImageBitmap as unknown as { mock: { calls: unknown[] } }).mock.calls.length;
}

describe('CavImgElement', () => {
  it('renders a canvas in the shadow root and mounts no <img>', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    expect(el.shadowRoot?.querySelector('canvas')).toBeTruthy();
    expect(el.shadowRoot?.querySelector('img')).toBeNull();
    expect(el.querySelector('img')).toBeNull();
  });

  it('scrubs the src attribute but keeps get src()', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    expect(el.hasAttribute('src')).toBe(false);
    expect(el.src).toBe('https://x/a.png');
  });

  it('sets role=img and aria-label from alt', async () => {
    const el = mount({ src: 'https://x/a.png', alt: 'a kitten' });
    await once(el, 'cav-load');
    expect(el.getAttribute('role')).toBe('img');
    expect(el.getAttribute('aria-label')).toBe('a kitten');
  });

  it('sets host aspect-ratio from the bitmap natural size', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    expect(el.style.aspectRatio.replace(/\s+/g, '')).toBe('200/100');
  });

  it('emits cav-error on a failed load', async () => {
    const el = mount();
    const errored = once(el, 'cav-error');
    el.setAttribute('src', 'https://x/bad.png');
    await errored;
    expect(el.shadowRoot?.querySelector('canvas')).toBeTruthy();
  });

  it('loads via the src property setter without touching the DOM attribute', async () => {
    const el = document.createElement('cav-img') as CavImgElement;
    document.body.append(el);
    el.src = 'https://x/b.png';
    await once(el, 'cav-load');
    expect(el.hasAttribute('src')).toBe(false);
    expect(el.src).toBe('https://x/b.png');
  });

  it('survives disconnect/reconnect without refetching', async () => {
    const el = mount({ src: 'https://x/a.png' });
    await once(el, 'cav-load');
    const before = bitmapCalls();
    el.remove();
    document.body.append(el);
    await tick();
    expect(bitmapCalls()).toBe(before);
    expect(el.shadowRoot?.querySelector('canvas')).toBeTruthy();
  });

  it('does not refetch when src is configured before the element is connected', async () => {
    const before = bitmapCalls();
    const el = document.createElement('cav-img') as CavImgElement;
    el.src = 'https://x/c.png'; // load starts while disconnected
    document.body.append(el); // connectedCallback must NOT start a second load
    await once(el, 'cav-load');
    await tick();
    expect(bitmapCalls() - before).toBe(1);
  });

  it('does not emit cav-error when a failed load has been superseded by a successful one', async () => {
    const el = mount();
    let errored = false;
    el.addEventListener('cav-error', () => {
      errored = true;
    });
    el.setAttribute('src', 'https://x/bad.png'); // load #1 will reject
    el.setAttribute('src', 'https://x/good.png'); // load #2 supersedes and resolves
    await once(el, 'cav-load');
    await tick();
    expect(errored).toBe(false);
  });
});
