import { writable } from 'svelte/store';

export type SleepTimerMode = 'off' | 'after-video' | 15 | 30 | 45 | 60;

export interface SleepTimerState {
  mode: SleepTimerMode;
  targetTimestamp: number | null; // epoch ms when timer fires
}

export const sleepTimer = writable<SleepTimerState>({
  mode: 'off',
  targetTimestamp: null,
});

let sleepTimeout: ReturnType<typeof setTimeout> | null = null;

export function setSleepTimer(mode: SleepTimerMode, onTrigger: () => void) {
  if (sleepTimeout) {
    clearTimeout(sleepTimeout);
    sleepTimeout = null;
  }

  if (mode === 'off') {
    sleepTimer.set({ mode: 'off', targetTimestamp: null });
    return;
  }

  if (mode === 'after-video') {
    sleepTimer.set({ mode: 'after-video', targetTimestamp: null });
    return;
  }

  const durationMs = mode * 60 * 1000;
  const targetTimestamp = Date.now() + durationMs;
  sleepTimer.set({ mode, targetTimestamp });

  sleepTimeout = setTimeout(() => {
    sleepTimer.set({ mode: 'off', targetTimestamp: null });
    sleepTimeout = null;
    onTrigger();
  }, durationMs);
}

export function clearSleepTimer() {
  if (sleepTimeout) {
    clearTimeout(sleepTimeout);
    sleepTimeout = null;
  }
  sleepTimer.set({ mode: 'off', targetTimestamp: null });
}
