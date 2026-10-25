import { describe, expect, it, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { sleepTimer, setSleepTimer, clearSleepTimer } from './sleepTimer';

describe('sleepTimer', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    clearSleepTimer();
  });

  it('starts in off state', () => {
    expect(get(sleepTimer)).toEqual({ mode: 'off', targetTimestamp: null });
  });

  it('sets after-video mode', () => {
    const cb = vi.fn();
    setSleepTimer('after-video', cb);
    expect(get(sleepTimer).mode).toBe('after-video');
    expect(cb).not.toHaveBeenCalled();
  });

  it('sets minute timer and triggers callback upon timeout', () => {
    const cb = vi.fn();
    setSleepTimer(15, cb);
    expect(get(sleepTimer).mode).toBe(15);
    expect(get(sleepTimer).targetTimestamp).toBeGreaterThan(Date.now());

    vi.advanceTimersByTime(15 * 60 * 1000);
    expect(cb).toHaveBeenCalledOnce();
    expect(get(sleepTimer).mode).toBe('off');
  });

  it('clears timer properly', () => {
    const cb = vi.fn();
    setSleepTimer(30, cb);
    clearSleepTimer();
    expect(get(sleepTimer).mode).toBe('off');
    vi.advanceTimersByTime(30 * 60 * 1000);
    expect(cb).not.toHaveBeenCalled();
  });
});
