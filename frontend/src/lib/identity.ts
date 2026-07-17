export interface LocalIdentity {
  id: string;
  secret: string;
  displayName: string;
  avatarSeed: string;
}
const key = 'koalaparty.identity.v1';
function randomSecret() {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  return btoa(String.fromCharCode(...bytes))
    .replaceAll('+', '-')
    .replaceAll('/', '_')
    .replaceAll('=', '');
}
export function getIdentity(): LocalIdentity {
  const raw = localStorage.getItem(key);
  if (raw) {
    try {
      return JSON.parse(raw) as LocalIdentity;
    } catch {
      localStorage.removeItem(key);
    }
  }
  const value = {
    id: crypto.randomUUID(),
    secret: randomSecret(),
    displayName: `Koala ${Math.floor(100 + Math.random() * 900)}`,
    avatarSeed: randomSecret().slice(0, 12),
  };
  localStorage.setItem(key, JSON.stringify(value));
  return value;
}
export function updateDisplayName(displayName: string) {
  const value = getIdentity();
  value.displayName = displayName.trim().slice(0, 32);
  localStorage.setItem(key, JSON.stringify(value));
  return value;
}
