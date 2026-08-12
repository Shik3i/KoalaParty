import { getIdentity, randomUUID, replaceWithAnonymousIdentity } from './identity';
export interface Principal {
  identityId: string;
  accountId?: string;
  displayName: string;
  csrfToken: string;
  isAdmin?: boolean;
}
// Carries the HTTP status so callers can tell a genuine client error (4xx) from
// a transient server/proxy blip (5xx) — e.g. a Bad Gateway during a deploy.
export class ApiError extends Error {
  status: number;
  code?: string;
  requestId?: string;
  constructor(status: number, message: string, code?: string, requestId?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}
let principal: Principal | null = null;
export function getPrincipal() {
  return principal;
}
let establishing: Promise<Principal> | null = null;
async function currentPrincipal(): Promise<Principal | null> {
  const requestId = randomUUID();
  const response = await fetch('/api/me', { headers: { 'X-Request-ID': requestId } });
  if (response.status === 204 || response.status === 401) return null;
  if (!response.ok) {
    const details = await problemDetails(response);
    throw new ApiError(
      response.status,
      details.message,
      details.code,
      response.headers.get('X-Request-ID') ?? requestId,
    );
  }
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
    const exchange = (i: ReturnType<typeof getIdentity>) =>
      fetch('/api/identity/exchange', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Request-ID': randomUUID() },
        body: JSON.stringify({ id: i.id, secret: i.secret, displayName: i.displayName }),
      });
    let r = await exchange(getIdentity());
    if (r.status === 401) {
      // Account-linked device secrets identify a browser but cannot recreate an
      // authenticated account session after logout/revocation. Continue with a
      // fresh anonymous identity; the account remains available through Log in.
      r = await exchange(replaceWithAnonymousIdentity());
    }
    if (!r.ok) {
      const details = await problemDetails(r);
      throw new ApiError(r.status, details.message, details.code, r.headers.get('X-Request-ID') ?? undefined);
    }
    principal = (await r.json()) as Principal;
    return principal;
  })().finally(() => {
    establishing = null;
  });
  return establishing;
}
async function problemDetails(r: Response): Promise<{ code?: string; message: string }> {
  try {
    const body = (await r.json()) as { code?: string; message?: string };
    return { code: body.code, message: body.message ?? r.statusText };
  } catch {
    return { message: r.statusText };
  }
}
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  let p = await establish();
  const logicalRequestId = randomUUID();
  const request = () => {
    const headers = new Headers(init.headers);
    headers.set('X-Request-ID', logicalRequestId);
    if (init.body) headers.set('Content-Type', 'application/json');
    if (init.method && init.method !== 'GET') headers.set('X-CSRF-Token', p.csrfToken);
    return fetch(path, { ...init, headers });
  };
  let r = await request();
  if (r.status === 401) {
    principal = null;
    p = await establish();
    r = await request();
  }
  if (r.status === 403) {
    const problem = await problemDetails(r);
    if (problem.code !== 'csrf_failed') {
      throw new ApiError(403, problem.message, problem.code, r.headers.get('X-Request-ID') ?? logicalRequestId);
    }
    principal = null;
    p = await establish();
    r = await request();
  }
  if (!r.ok) {
    const problem = await problemDetails(r);
    throw new ApiError(r.status, problem.message, problem.code, r.headers.get('X-Request-ID') ?? logicalRequestId);
  }
  if (r.status === 204) return undefined as T;
  return (await r.json()) as T;
}
export function websocketURL(path: string) {
  const u = new URL(path, location.href);
  u.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return u.toString();
}
