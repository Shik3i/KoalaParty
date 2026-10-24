import { writable } from 'svelte/store';

const volumeKey = 'koalaparty.volume';
const muteKey = 'koalaparty.muted';

function initialVolume(): number {
  if (typeof localStorage === 'undefined') return 100;
  try {
    const raw = localStorage.getItem(volumeKey);
    if (!raw) return 100;
    const v = Number(raw);
    return Number.isFinite(v) && v >= 0 && v <= 100 ? v : 100;
  } catch {
    return 100;
  }
}

function initialMuted(): boolean {
  if (typeof localStorage === 'undefined') return false;
  try {
    return localStorage.getItem(muteKey) === '1';
  } catch {
    return false;
  }
}

export const volume = writable<number>(initialVolume());
export const muted = writable<boolean>(initialMuted());

export function setVolume(v: number) {
  const clamped = Math.max(0, Math.min(100, Math.round(v)));
  volume.set(clamped);
  if (clamped > 0) muted.set(false);
  try {
    localStorage.setItem(volumeKey, String(clamped));
    if (clamped > 0) localStorage.setItem(muteKey, '0');
  } catch {}
}

export function toggleMute() {
  muted.update((curr) => {
    const next = !curr;
    try {
      localStorage.setItem(muteKey, next ? '1' : '0');
    } catch {}
    return next;
  });
}
