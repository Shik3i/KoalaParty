import { describe, expect, it, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  ambilight,
  DEFAULT_AMBILIGHT,
  generatePaletteFromSeed,
  initialAmbilight,
  resetAmbilight,
  updateAmbilight,
} from './ambilight';

describe('ambilight state & persistence', () => {
  beforeEach(() => {
    localStorage.clear();
    resetAmbilight();
  });

  it('loads default config when storage is empty', () => {
    const config = initialAmbilight();
    expect(config).toEqual(DEFAULT_AMBILIGHT);
  });

  it('safely handles corrupt storage data', () => {
    localStorage.setItem('koalaparty.ambilight', '{invalid json');
    const config = initialAmbilight();
    expect(config).toEqual(DEFAULT_AMBILIGHT);
  });

  it('updates state and persists to localStorage', () => {
    updateAmbilight({ mode: 'static', color: '#a855f7', intensity: 0.9, blur: 40 });
    const current = get(ambilight);
    expect(current.mode).toBe('static');
    expect(current.color).toBe('#a855f7');
    expect(current.intensity).toBe(0.9);
    expect(current.blur).toBe(40);

    const stored = JSON.parse(localStorage.getItem('koalaparty.ambilight') || '{}');
    expect(stored.mode).toBe('static');
    expect(stored.color).toBe('#a855f7');
  });

  it('generates consistent harmonious 4-color palettes from seed', () => {
    const p1 = generatePaletteFromSeed('dQw4w9WgXcQ');
    const p2 = generatePaletteFromSeed('dQw4w9WgXcQ');
    expect(p1).toEqual(p2);
    expect(p1).toHaveLength(4);
    expect(p1[0]).toMatch(/^hsl\(\d+,\s*82%,\s*58%\)$/);
  });
});
