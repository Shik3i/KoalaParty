import { describe, expect, it, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { chatBubblesEnabled, suppressCaptionsEnabled, setChatBubbles, setSuppressCaptions } from './preferences';

describe('preferences store', () => {
  beforeEach(() => {
    localStorage.clear();
    setChatBubbles(true);
    setSuppressCaptions(true);
  });

  it('defaults to true', () => {
    expect(get(chatBubblesEnabled)).toBe(true);
    expect(get(suppressCaptionsEnabled)).toBe(true);
  });

  it('updates chat bubbles preference and persists to localStorage', () => {
    setChatBubbles(false);
    expect(get(chatBubblesEnabled)).toBe(false);
    expect(localStorage.getItem('koalaparty.chatBubbles')).toBe('0');
  });

  it('updates suppress captions preference and persists to localStorage', () => {
    setSuppressCaptions(false);
    expect(get(suppressCaptionsEnabled)).toBe(false);
    expect(localStorage.getItem('koalaparty.suppressCaptions')).toBe('0');
  });
});
