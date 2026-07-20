import { beforeEach, describe, expect, it } from 'vitest';
import { applyTheme, initialTheme, applyDesign, initialDesign } from './theme';

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

describe('design preference', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-design');
  });
  it('defaults to eucalyptus and persists a chosen design', () => {
    expect(initialDesign()).toBe('eucalyptus');
    applyDesign('ocean');
    expect(document.documentElement.dataset.design).toBe('ocean');
    expect(initialDesign()).toBe('ocean');
  });
  it('drops the attribute for the default eucalyptus design', () => {
    applyDesign('grape');
    applyDesign('eucalyptus');
    expect(document.documentElement.hasAttribute('data-design')).toBe(false);
  });
  it('ignores an unknown stored design', () => {
    localStorage.setItem('koalaparty.design', 'chartreuse');
    expect(initialDesign()).toBe('eucalyptus');
  });
});
