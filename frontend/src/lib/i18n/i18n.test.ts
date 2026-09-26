import { describe, expect, it } from 'vitest';
import { en } from './en';
import { de } from './de';
import { detectLocale, errorText, translate } from './index';

describe('i18n', () => {
  it('prefers a stored choice, then the browser languages, then English', () => {
    expect(detectLocale('de', ['en-US'])).toBe('de');
    expect(detectLocale(null, ['fr-FR', 'de-AT', 'en'])).toBe('de');
    expect(detectLocale(null, ['en-GB', 'de'])).toBe('en');
    expect(detectLocale('xx', ['fr'])).toBe('en');
  });

  it('fills placeholders and leaves unknown ones visible', () => {
    expect(translate('de', 'room.watching', { count: 3 })).toBe('3 schauen zu');
    expect(translate('en', 'room.waitingFor', {})).toBe('Waiting for {names}');
  });

  it('keeps both dictionaries complete and placeholder-compatible', () => {
    const placeholders = (text: string) => [...text.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
    for (const key of Object.keys(en) as Array<keyof typeof en>) {
      expect(de[key], key).toBeTruthy();
      expect(placeholders(de[key]), key).toEqual(placeholders(en[key]));
    }
    expect(Object.keys(de).sort()).toEqual(Object.keys(en).sort());
  });

  it('maps known server error codes and keeps other server messages', () => {
    expect(errorText(Object.assign(new Error('x'), { code: 'already_queued' }))).toBe(en['error.already_queued']);
    expect(errorText(Object.assign(new Error('Server says no'), { code: 'weird' }))).toBe('Server says no');
    expect(errorText('nope')).toBe(en['error.generic']);
  });
});
