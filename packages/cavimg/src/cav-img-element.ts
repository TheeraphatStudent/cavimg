import { loadImageBitmap } from './loader';
import { drawBitmap } from './render';
import { harden } from './protect';
import type { FitMode } from './types';

const FIT_MODES: readonly FitMode[] = ['contain', 'cover', 'fill'];
const SHADOW_CSS = ':host{display:block}canvas{display:block;width:100%;height:100%}';

export class CavImgElement extends HTMLElement {
  static get observedAttributes(): string[] {
    return ['src', 'fit', 'alt'];
  }

  #canvas: HTMLCanvasElement;
  #url: string | null = null;
  #bitmap: ImageBitmap | null = null;
  #ro: ResizeObserver | null = null;
  #unharden: (() => void) | null = null;
  #connected = false;
  #token = 0;

  constructor() {
    super();
    const root = this.attachShadow({ mode: 'open' });
    const style = document.createElement('style');
    style.textContent = SHADOW_CSS;
    this.#canvas = document.createElement('canvas');
    root.append(style, this.#canvas);
    this.setAttribute('role', 'img');
  }

  get src(): string | null {
    return this.#url ?? this.getAttribute('src');
  }
  set src(value: string | null) {
    if (value == null || value === '') {
      this.#url = null;
      this.removeAttribute('src');
      return;
    }
    this.#loadFrom(value);
  }

  get fit(): FitMode {
    const value = this.getAttribute('fit');
    return FIT_MODES.includes(value as FitMode) ? (value as FitMode) : 'contain';
  }
  set fit(value: FitMode) {
    this.setAttribute('fit', value);
  }

  load(url: string): void {
    this.#loadFrom(url);
  }

  connectedCallback(): void {
    this.#connected = true;
    this.#unharden = harden(this.#canvas);
    this.#ro = new ResizeObserver(() => this.#redraw());
    this.#ro.observe(this);

    const alt = this.getAttribute('alt');
    if (alt != null) this.setAttribute('aria-label', alt);

    if (this.#bitmap) {
      this.#redraw();
    } else if (this.#url) {
      this.#loadFrom(this.#url);
    } else {
      const attr = this.getAttribute('src');
      if (attr) this.#loadFrom(attr);
    }
  }

  disconnectedCallback(): void {
    this.#connected = false;
    this.#ro?.disconnect();
    this.#ro = null;
    this.#unharden?.();
    this.#unharden = null;
    // Intentionally retain #url and #bitmap so reconnect redraws without a refetch.
  }

  attributeChangedCallback(name: string, _old: string | null, value: string | null): void {
    if (!this.#connected) return; // ignore initial-parse attribute sets; connectedCallback handles them
    if (name === 'src') {
      if (value) this.#loadFrom(value);
    } else if (name === 'fit') {
      this.#redraw();
    } else if (name === 'alt') {
      this.setAttribute('aria-label', value ?? '');
    }
  }

  #loadFrom(url: string): void {
    this.#url = url;
    const token = ++this.#token;
    loadImageBitmap(url)
      .then((bitmap) => {
        if (token !== this.#token) {
          bitmap.close?.();
          return;
        }
        this.#bitmap = bitmap;
        this.removeAttribute('src'); // scrub the URL from the DOM; #url still holds it
        this.style.aspectRatio = `${bitmap.width} / ${bitmap.height}`;
        this.#redraw();
        this.dispatchEvent(new CustomEvent('cav-load'));
      })
      .catch(() => {
        this.dispatchEvent(new CustomEvent('cav-error'));
      });
  }

  #redraw(): void {
    if (!this.#bitmap) return;
    drawBitmap({
      canvas: this.#canvas,
      bitmap: this.#bitmap,
      fit: this.fit,
      dpr: (typeof window !== 'undefined' && window.devicePixelRatio) || 1,
    });
  }
}
