import { getIdentity } from './identity';
export interface Principal {
  identityId: string;
  accountId?: string;
  displayName: string;
  csrfToken: string;
  isAdmin?: boolean;
}
let principal: Principal | null = null;
export function getPrincipal() {
  return principal;
}
let establishing: Promise<Principal> | null = null;
async function currentPrincipal(): Promise<Principal | null> {
  const response = await fetch('/api/me');
  if (response.status === 401) return null;
  if (!response.ok) throw new Error(await message(response));
  return (await response.json()) as Principal;
}
export async function establish(): Promise<Principal> {
  if (principal) return principal;
  if (establishing) return establishing;
  establishing = (async () => {
    const current = await currentPrincipal();
    if (current) {
      principal = current;
      return current;
    }
    const i = getIdentity();
    const r = await fetch('/api/identity/exchange', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: i.id, secret: i.secret, displayName: i.displayName }),
    });
    if (!r.ok) throw new Error(await message(r));
    principal = (await r.json()) as Principal;
    return principal;
  })().finally(() => {
    establishing = null;
  });
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
  let p = await establish();
  const request = () => {
    const headers = new Headers(init.headers);
    if (init.body) headers.set('Content-Type', 'application/json');
    if (init.method && init.method !== 'GET') headers.set('X-CSRF-Token', p.csrfToken);
    return fetch(path, { ...init, headers });
  };
  let r = await request();
  if (r.status === 403) {
    const problem = (await r.json()) as { code?: string; message?: string };
    if (problem.code !== 'csrf_failed') throw new Error(problem.message ?? r.statusText);
    const current = await currentPrincipal();
    if (!current) throw new Error('Session expired. Reload and try again.');
    principal = p = current;
    r = await request();
  }
  if (!r.ok) throw new Error(await message(r));
  if (r.status === 204) return undefined as T;
  return (await r.json()) as T;
}
export function websocketURL(path: string) {
  const u = new URL(path, location.href);
  u.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return u.toString();
}
