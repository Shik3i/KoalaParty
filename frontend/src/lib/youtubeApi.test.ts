import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type YouTubeTestWindow = Window & {
  YT?: { Player?: unknown };
  onYouTubeIframeAPIReady?: () => void;
};

describe('YouTube iframe API loader', () => {
  beforeEach(() => {
    vi.resetModules();
    document.querySelectorAll('script[src*="youtube.com/iframe_api"]').forEach((script) => script.remove());
    delete (window as YouTubeTestWindow).YT;
    delete (window as YouTubeTestWindow).onYouTubeIframeAPIReady;
  });

  afterEach(() => {
    document.querySelectorAll('script[src*="youtube.com/iframe_api"]').forEach((script) => script.remove());
    delete (window as YouTubeTestWindow).YT;
    delete (window as YouTubeTestWindow).onYouTubeIframeAPIReady;
  });

  it('removes a failed script and creates a fresh script for retry', async () => {
    const { loadYouTubeAPI } = await import('./youtubeApi');
    const first = loadYouTubeAPI();
    const firstFailure = expect(first).rejects.toThrow('YouTube player could not be loaded.');
    const failedScript = document.querySelector<HTMLScriptElement>('script[src*="youtube.com/iframe_api"]');
    expect(failedScript).not.toBeNull();
    failedScript!.dispatchEvent(new Event('error'));
    await firstFailure;
    expect(failedScript!.isConnected).toBe(false);

    const second = loadYouTubeAPI();
    const replacement = document.querySelector<HTMLScriptElement>('script[src*="youtube.com/iframe_api"]');
    expect(replacement).not.toBeNull();
    expect(replacement).not.toBe(failedScript);
    (window as YouTubeTestWindow).YT = { Player: class {} };
    (window as YouTubeTestWindow).onYouTubeIframeAPIReady?.();
    await expect(second).resolves.toBeUndefined();
  });
});
