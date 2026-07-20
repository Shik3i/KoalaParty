export type Theme = 'system' | 'light' | 'dark';
const key = 'koalaparty.theme';
export function applyTheme(theme: Theme) {
  if (typeof document === 'undefined') return;
  if (theme === 'system') document.documentElement.removeAttribute('data-theme');
  else document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem(key, theme);
  } catch {
    // Theme changes still apply when browser storage is unavailable.
  }
}
export function initialTheme(): Theme {
  if (typeof localStorage === 'undefined') return 'system';
  try {
    const value = localStorage.getItem(key);
    return value === 'light' || value === 'dark' ? value : 'system';
  } catch {
    return 'system';
  }
}

// The design picks the colour palette; the theme (light/dark/system) picks the
// mode within it. The two are independent and each persists on its own.
export type Design = 'eucalyptus' | 'ocean' | 'amber' | 'grape' | 'rose';
export const designs: { value: Design; label: string; swatch: string }[] = [
  { value: 'eucalyptus', label: 'Eucalyptus', swatch: '#286846' },
  { value: 'ocean', label: 'Ocean', swatch: '#1f6f8b' },
  { value: 'amber', label: 'Amber', swatch: '#9a6a15' },
  { value: 'grape', label: 'Grape', swatch: '#6b3fa0' },
  { value: 'rose', label: 'Rose', swatch: '#a83a5b' },
];
const designKey = 'koalaparty.design';
const isDesign = (value: string | null): value is Design => designs.some((d) => d.value === value);
export function applyDesign(design: Design) {
  if (typeof document === 'undefined') return;
  // Eucalyptus is the default palette on the bare :root, so it needs no attribute.
  if (design === 'eucalyptus') document.documentElement.removeAttribute('data-design');
  else document.documentElement.dataset.design = design;
  try {
    localStorage.setItem(designKey, design);
  } catch {
    // Design changes still apply when browser storage is unavailable.
  }
}
export function initialDesign(): Design {
  if (typeof localStorage === 'undefined') return 'eucalyptus';
  try {
    const value = localStorage.getItem(designKey);
    return isDesign(value) ? value : 'eucalyptus';
  } catch {
    return 'eucalyptus';
  }
}
