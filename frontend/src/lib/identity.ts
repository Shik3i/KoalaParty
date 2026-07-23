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
    Array.from(identity.displayName).length <= 32 &&
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
// crypto.randomUUID() only exists in secure contexts (HTTPS / localhost). Over a
// plain-HTTP LAN address — common when self-hosting a watch party — it is
// undefined and would throw. crypto.getRandomValues() works everywhere, so we
// build a v4 UUID from it as a fallback.
export function randomUUID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    try {
      return crypto.randomUUID();
    } catch {
      // Fall through to manual implementation.
    }
  }
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    const b = crypto.getRandomValues(new Uint8Array(16));
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    const h = Array.from(b, (x) => x.toString(16).padStart(2, '0'));
    return `${h[0]}${h[1]}${h[2]}${h[3]}-${h[4]}${h[5]}-${h[6]}${h[7]}-${h[8]}${h[9]}-${h[10]}${h[11]}${h[12]}${h[13]}${h[14]}${h[15]}`;
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}
// Playful anonymous names. nameEmojis and nameAnimals are index-aligned so the
// emoji always matches the animal. Kept in sync with the backend room-label
// pools (backend/internal/app/names.go).
// prettier-ignore
const nameEmojis = [
  '🐨', '🦘', '🦊', '🦉', '🐼', '🦦', '🦔', '🐧', '🦩', '🦢',
  '🐢', '🐸', '🦎', '🦇', '🦫', '🦥', '🦡', '🐹', '🐰', '🦋',
  '🐝', '🐙', '🦈', '🐳', '🦭', '🦜', '🦚', '🐿️', '🦆', '🦌',
  '🐺', '🐬',
];
// prettier-ignore
const nameAnimals = [
  'Koala', 'Kangaroo', 'Fox', 'Owl', 'Panda', 'Otter', 'Hedgehog', 'Penguin', 'Flamingo', 'Swan',
  'Turtle', 'Frog', 'Gecko', 'Bat', 'Beaver', 'Sloth', 'Badger', 'Hamster', 'Rabbit', 'Butterfly',
  'Bee', 'Octopus', 'Shark', 'Whale', 'Seal', 'Parrot', 'Peacock', 'Squirrel', 'Duck', 'Deer',
  'Wolf', 'Dolphin',
];
// prettier-ignore
const nameAdjectives = [
  'Calm', 'Gentle', 'Mossy', 'Quiet', 'Sunny', 'Cozy', 'Bamboo', 'Forest',
  'Bouncy', 'Sleepy', 'Clever', 'Fuzzy', 'Happy', 'Brave', 'Swift', 'Wandering',
  'Cheerful', 'Curious', 'Mellow', 'Nimble', 'Plucky', 'Jolly', 'Breezy', 'Dapper',
  'Snug', 'Wild',
];
function pick<T>(items: T[]): T {
  return items[Math.floor(Math.random() * items.length)];
}
function randomDisplayName(): string {
  const i = Math.floor(Math.random() * nameAnimals.length);
  const emoji = nameEmojis[i];
  const name = `${emoji} ${pick(nameAdjectives)} ${nameAnimals[i]}`;
  // The server caps display names at 32 bytes; drop the adjective if we overrun.
  return new TextEncoder().encode(name).length <= 32 ? name : `${emoji} ${nameAnimals[i]}`;
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
    id: randomUUID(),
    secret: randomSecret(),
    displayName: randomDisplayName(),
    avatarSeed: randomSecret().slice(0, 12),
  };
  persist(value);
  return value;
}
export function updateDisplayName(displayName: string) {
  const value = getIdentity();
  const normalized = Array.from(displayName.trim()).slice(0, 32).join('');
  if (normalized) value.displayName = normalized;
  persist(value);
  return value;
}

export function resetIdentityCacheForTests() {
  fallback = null;
}
