export interface LocalIdentity {
  id: string;
  secret: string;
  displayName: string;
  avatarSeed: string;
}
const key = 'koalaparty.identity.v1';
let fallback: LocalIdentity | null = null;
function valid(value: unknown): value is LocalIdentity {
  if (!value || typeof value !== 'object') return false;
  const identity = value as Partial<LocalIdentity>;
  return (
    typeof identity.id === 'string' &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(identity.id) &&
    typeof identity.secret === 'string' &&
    identity.secret.length >= 40 &&
    identity.secret.length <= 128 &&
    typeof identity.displayName === 'string' &&
    identity.displayName.trim().length > 0 &&
    identity.displayName.length <= 32 &&
    typeof identity.avatarSeed === 'string' &&
    identity.avatarSeed.length > 0
  );
}
function read() {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}
function persist(value: LocalIdentity) {
  fallback = value;
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // The identity remains stable for this page when storage is unavailable.
  }
}
function randomSecret() {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  return btoa(String.fromCharCode(...bytes))
    .replaceAll('+', '-')
    .replaceAll('/', '_')
    .replaceAll('=', '');
}
export function getIdentity(): LocalIdentity {
  if (fallback) return fallback;
  const raw = read();
  if (raw) {
    try {
      const parsed: unknown = JSON.parse(raw);
      if (valid(parsed)) {
        fallback = parsed;
        return parsed;
      }
    } catch {
      // Regenerate invalid or truncated credentials below.
    }
    try {
      localStorage.removeItem(key);
    } catch {
      // Storage may be disabled.
    }
  }
  const value = {
    id: crypto.randomUUID(),
    secret: randomSecret(),
    displayName: `Koala ${Math.floor(100 + Math.random() * 900)}`,
    avatarSeed: randomSecret().slice(0, 12),
  };
  persist(value);
  return value;
}
export function updateDisplayName(displayName: string) {
  const value = getIdentity();
  const normalized = displayName.trim().slice(0, 32);
  if (normalized) value.displayName = normalized;
  persist(value);
  return value;
}

export function resetIdentityCacheForTests() {
  fallback = null;
}
