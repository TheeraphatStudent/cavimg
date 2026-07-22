import { describe, it, expect } from 'vitest';
import { VERSION } from '../src/index';

describe('smoke', () => {
  it('runs in a happy-dom environment', () => {
    expect(typeof document).toBe('object');
    expect(typeof HTMLCanvasElement).toBe('function');
  });

  it('exports VERSION', () => {
    expect(VERSION).toBe('1.0.0');
  });
});
