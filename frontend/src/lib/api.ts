import { getIdentity } from './identity';
export interface Principal {
  identityId: string;
  accountId?: string;
  displayName: string;
  csrfToken: string;
}
let principal: Principal | null = null;
let establishing: Promise<Principal> | null = null;
export async function establish(): Promise<Principal> {
  if (principal) return principal;
  if (establishing) return establishing;
  establishing = (async () => {
    const i = getIdentity();
    const r = await fetch('/api/identity/exchange', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: i.id, secret: i.secret, displayName: i.displayName }),
    });
    if (!r.ok) throw new Error(await message(r));
    principal = (await r.json()) as Principal;
    return principal;
  })();
  return establishing;
}
async function message(r: Response) {
  try {
    return ((await r.json()) as { message?: string }).message ?? r.statusText;
  } catch {
    return r.statusText;
  }
}
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const p = await establish();
  const headers = new Headers(init.headers);
  if (init.body) headers.set('Content-Type', 'application/json');
  if (init.method && init.method !== 'GET') headers.set('X-CSRF-Token', p.csrfToken);
  const r = await fetch(path, { ...init, headers });
  if (!r.ok) throw new Error(await message(r));
  if (r.status === 204) return undefined as T;
  return (await r.json()) as T;
}
export function websocketURL(path: string) {
  const u = new URL(path, location.href);
  u.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return u.toString();
}
