type YouTubeWindow = Window & {
  YT?: { Player?: unknown };
  onYouTubeIframeAPIReady?: () => void;
};

let apiPromise: Promise<void> | null = null;

export function loadYouTubeAPI(): Promise<void> {
  if (typeof window === 'undefined') return Promise.reject(new Error('YouTube player requires a browser.'));
  const w = window as YouTubeWindow;
  if (w.YT?.Player) return Promise.resolve();
  if (apiPromise) return apiPromise;

  apiPromise = new Promise<void>((resolve, reject) => {
    let settled = false;
    const finish = (error?: Error) => {
      if (settled) return;
      settled = true;
      window.clearTimeout(timeout);
      if (error) reject(error);
      else if (w.YT?.Player) resolve();
      else reject(new Error('YouTube player API loaded without a player constructor.'));
    };
    const timeout = window.setTimeout(() => finish(new Error('YouTube player loading timed out.')), 12_000);
    const previous = w.onYouTubeIframeAPIReady;
    w.onYouTubeIframeAPIReady = () => {
      finish();
      try {
        previous?.();
      } catch {
        // A consumer callback must not break the shared loader.
      }
    };
    let script = document.querySelector<HTMLScriptElement>('script[src*="youtube.com/iframe_api"]');
    if (!script) {
      script = document.createElement('script');
      script.src = 'https://www.youtube.com/iframe_api';
      script.async = true;
      script.addEventListener('error', () => finish(new Error('YouTube player could not be loaded.')), { once: true });
      document.head.appendChild(script);
    } else {
      script.addEventListener('error', () => finish(new Error('YouTube player could not be loaded.')), { once: true });
    }
  });

  const pending = apiPromise;
  const tracked = pending.catch((error) => {
    if (apiPromise === tracked) apiPromise = null;
    throw error;
  });
  apiPromise = tracked;
  return tracked;
}
