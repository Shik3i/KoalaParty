import { describe, expect, it, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { chatSound, toggleChatSound, playNotificationChime } from './audio';

describe('audio & chat sound', () => {
  beforeEach(() => {
    localStorage.clear();
    chatSound.set(true);
  });

  it('defaults to true and toggles properly', () => {
    expect(get(chatSound)).toBe(true);
    toggleChatSound();
    expect(get(chatSound)).toBe(false);
    expect(localStorage.getItem('koalaparty.chatSound')).toBe('0');
  });

  it('runs playNotificationChime safely in test/node environment', () => {
    expect(() => playNotificationChime()).not.toThrow();
  });
});
