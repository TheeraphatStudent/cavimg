import { describe, it, expect } from 'vitest';
import { harden } from '../src/protect';

function fire(el: EventTarget, type: string): Event {
  const ev = new Event(type, { cancelable: true, bubbles: true });
  el.dispatchEvent(ev);
  return ev;
}

describe('harden', () => {
  it('marks the canvas non-draggable', () => {
    const canvas = document.createElement('canvas');
    harden(canvas);
    expect(canvas.getAttribute('draggable')).toBe('false');
  });

  it('prevents contextmenu and dragstart', () => {
    const canvas = document.createElement('canvas');
    harden(canvas);
    expect(fire(canvas, 'contextmenu').defaultPrevented).toBe(true);
    expect(fire(canvas, 'dragstart').defaultPrevented).toBe(true);
  });

  it('cleanup removes the listeners', () => {
    const canvas = document.createElement('canvas');
    const cleanup = harden(canvas);
    cleanup();
    expect(fire(canvas, 'contextmenu').defaultPrevented).toBe(false);
    expect(fire(canvas, 'dragstart').defaultPrevented).toBe(false);
  });
});
