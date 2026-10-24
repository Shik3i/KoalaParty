import { describe, expect, it, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { volume, muted, setVolume, toggleMute } from './volume';

describe('volume store', () => {
  beforeEach(() => {
    localStorage.clear();
    setVolume(100);
    muted.set(false);
  });

  it('updates volume and persists to storage', () => {
    setVolume(75);
    expect(get(volume)).toBe(75);
    expect(localStorage.getItem('koalaparty.volume')).toBe('75');
  });

  it('clamps volume between 0 and 100', () => {
    setVolume(150);
    expect(get(volume)).toBe(100);
    setVolume(-20);
    expect(get(volume)).toBe(0);
  });

  it('toggles mute correctly', () => {
    expect(get(muted)).toBe(false);
    toggleMute();
    expect(get(muted)).toBe(true);
    expect(localStorage.getItem('koalaparty.muted')).toBe('1');
    toggleMute();
    expect(get(muted)).toBe(false);
  });
});
