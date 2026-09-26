import { derived, get, writable } from 'svelte/store';
import { en, type MessageKey } from './en';
import { de } from './de';

// Lightweight i18n: flat dictionaries, `{name}` placeholders, English fallback.
// English is the source of truth; the German dictionary is type-checked against
// it so a missing translation fails the build instead of shipping a raw key.

export type Locale = 'en' | 'de';
export type Translate = (key: MessageKey, vars?: Record<string, string | number>) => string;

export const LOCALES: Array<{ value: Locale; label: string }> = [
  { value: 'en', label: 'English' },
  { value: 'de', label: 'Deutsch' },
];

const dictionaries: Record<Locale, Record<MessageKey, string>> = { en, de };
const storageKey = 'koalaparty.locale';

export function detectLocale(stored: string | null, languages: readonly string[]): Locale {
  if (stored === 'en' || stored === 'de') return stored;
  for (const language of languages) {
    const base = language.toLowerCase().split('-')[0];
    if (base === 'de') return 'de';
    if (base === 'en') return 'en';
  }
  return 'en';
}

function initialLocale(): Locale {
  if (typeof window === 'undefined') return 'en';
  let stored: string | null = null;
  try {
    stored = localStorage.getItem(storageKey);
  } catch {
    /* storage unavailable */
  }
  return detectLocale(stored, navigator.languages ?? [navigator.language]);
}

export function translate(locale: Locale, key: MessageKey, vars?: Record<string, string | number>): string {
  const template = dictionaries[locale][key] ?? en[key] ?? key;
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match));
}

export const locale = writable<Locale>(initialLocale());
export const t = derived(
  locale,
  ($locale): Translate =>
    (key, vars) =>
      translate($locale, key, vars),
);

locale.subscribe((value) => {
  if (typeof document !== 'undefined') document.documentElement.lang = value;
});

export function setLocale(value: Locale) {
  locale.set(value);
  try {
    localStorage.setItem(storageKey, value);
  } catch {
    /* storage unavailable */
  }
}

/** Non-reactive access for plain TypeScript helpers. */
export function tNow(key: MessageKey, vars?: Record<string, string | number>) {
  return translate(get(locale), key, vars);
}

/** Localized message for a server error code, falling back to the server text. */
export function errorText(error: unknown, fallback: MessageKey = 'error.generic'): string {
  const code = typeof error === 'object' && error && 'code' in error ? String((error as { code?: string }).code) : '';
  const key = `error.${code}` as MessageKey;
  if (code && key in en) return tNow(key);
  if (error instanceof Error && error.message) return error.message;
  return tNow(fallback);
}

export type { MessageKey };
