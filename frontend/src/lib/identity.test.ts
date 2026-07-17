import { beforeEach, describe, expect, it } from 'vitest';
import { getIdentity, resetIdentityCacheForTests, updateDisplayName } from './identity';

describe('persistent anonymous identity', () => {
  beforeEach(() => {
    localStorage.clear();
    resetIdentityCacheForTests();
  });
  it('creates credentials once and restores them', () => {
    const first = getIdentity();
    const second = getIdentity();
    expect(first.id).toMatch(/^[0-9a-f-]{36}$/);
    expect(first.secret.length).toBeGreaterThanOrEqual(40);
    expect(second).toEqual(first);
  });
  it('persists a bounded display name', () => {
    const updated = updateDisplayName('Forest Friend'.padEnd(80, '!'));
    expect(updated.displayName).toHaveLength(32);
    expect(getIdentity().displayName).toBe(updated.displayName);
  });
  it('regenerates structurally invalid stored credentials', () => {
    localStorage.setItem('koalaparty.identity.v1', JSON.stringify({ id: 'broken', secret: 'short' }));
    const identity = getIdentity();
    expect(identity.id).toMatch(/^[0-9a-f-]{36}$/);
    expect(identity.secret.length).toBeGreaterThanOrEqual(40);
  });
  it('does not replace the display name with whitespace', () => {
    const original = getIdentity();
    expect(updateDisplayName('   ').displayName).toBe(original.displayName);
  });
});
