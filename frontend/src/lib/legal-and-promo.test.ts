import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = (path: string) => readFileSync(resolve(process.cwd(), path), 'utf8');

describe('legal and KoalaSync cross-promotion', () => {
  it('documents the controller, retention and YouTube data flow', () => {
    const privacy = source('src/routes/privacy/+page.svelte');
    expect(privacy).toContain('Timo Schmidt');
    expect(privacy).toContain('admin@koalastuff.net');
    expect(privacy).toContain('Room activity is limited to 200 visible events per room and 30 days');
    expect(privacy).toContain('youtube-nocookie.com');
    expect(privacy).toContain('oEmbed');
  });

  it('keeps platform artwork local and links to KoalaSync', () => {
    const promo = source('src/lib/KoalaSyncPromo.svelte');
    expect(promo).toContain('https://sync.koalastuff.net/');
    expect(promo).toContain('/assets/platforms/netflix.svg');
    expect(promo).toContain('/assets/platforms/disney-plus.svg');
    expect(promo).not.toMatch(/<img[^>]+src=["']https?:/);
  });

  it('serves explicit crawler policy instead of the SPA fallback', () => {
    expect(source('static/robots.txt')).toBe('User-agent: *\nAllow: /\n');
  });

  it('points the retired imprint route at the central legal notice', () => {
    const imprint = source('src/routes/imprint/+page.svelte');
    expect(imprint).toContain('https://koalastuff.net/legal');
    expect(imprint).toContain('rel="canonical"');
    expect(source('../LICENSE')).toContain('Permission is hereby granted, free of charge');
  });
});
