export type Theme = 'system' | 'light' | 'dark';
const key = 'koalaparty.theme';
export function applyTheme(theme: Theme) {
  if (typeof document === 'undefined') return;
  if (theme === 'system') document.documentElement.removeAttribute('data-theme');
  else document.documentElement.dataset.theme = theme;
  localStorage.setItem(key, theme);
}
export function initialTheme(): Theme {
  if (typeof localStorage === 'undefined') return 'system';
  const value = localStorage.getItem(key);
  return value === 'light' || value === 'dark' ? value : 'system';
}
