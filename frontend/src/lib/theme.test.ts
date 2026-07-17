import { beforeEach, describe, expect, it } from 'vitest';
import { applyTheme, initialTheme } from './theme';

describe('theme preference', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
  });
  it('defaults to system and persists a manual theme', () => {
    expect(initialTheme()).toBe('system');
    applyTheme('dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(initialTheme()).toBe('dark');
  });
  it('removes the override for system theme', () => {
    applyTheme('light');
    applyTheme('system');
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
  });
});
