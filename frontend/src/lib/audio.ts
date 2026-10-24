import { writable } from 'svelte/store';

const soundKey = 'koalaparty.chatSound';

function initialSound(): boolean {
  if (typeof localStorage === 'undefined') return true;
  try {
    const val = localStorage.getItem(soundKey);
    return val === null ? true : val === '1';
  } catch {
    return true;
  }
}

export const chatSound = writable<boolean>(initialSound());

export function toggleChatSound() {
  chatSound.update((curr) => {
    const next = !curr;
    try {
      localStorage.setItem(soundKey, next ? '1' : '0');
    } catch {}
    return next;
  });
}

let audioCtx: AudioContext | null = null;

/**
 * Plays a soft, pleasant notification chime using Web Audio API synthesis.
 * Requires no external audio files or network requests.
 */
export function playNotificationChime() {
  if (typeof window === 'undefined') return;
  try {
    const AudioCtx =
      window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    if (!AudioCtx) return;
    if (!audioCtx) audioCtx = new AudioCtx();
    if (audioCtx.state === 'suspended') void audioCtx.resume();

    const now = audioCtx.currentTime;
    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();

    osc.type = 'sine';
    // Two-tone bell chime: D5 (587.33 Hz) jumping to A5 (880 Hz)
    osc.frequency.setValueAtTime(587.33, now);
    osc.frequency.setValueAtTime(880, now + 0.08);

    gain.gain.setValueAtTime(0.08, now);
    gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.28);

    osc.connect(gain);
    gain.connect(audioCtx.destination);

    osc.start(now);
    osc.stop(now + 0.28);
  } catch {
    /* AudioContext not allowed yet or browser muted; harmless */
  }
}
