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
    let script = document.querySelector<HTMLScriptElement>('script[src*="youtube.com/iframe_api"]');
    const previous = w.onYouTubeIframeAPIReady;
    let timeout = 0;
    const onError = () => finish(new Error('YouTube player could not be loaded.'));
    const ready = () => {
      finish();
      try {
        previous?.();
      } catch {
        // A consumer callback must not break the shared loader.
      }
    };
    const finish = (error?: Error) => {
      if (settled) return;
      settled = true;
      window.clearTimeout(timeout);
      script?.removeEventListener('error', onError);
      if (w.onYouTubeIframeAPIReady === ready) w.onYouTubeIframeAPIReady = previous;
      const failure =
        error ?? (w.YT?.Player ? null : new Error('YouTube player API loaded without a player constructor.'));
      if (failure) {
        if (!w.YT?.Player) script?.remove();
        reject(failure);
      } else resolve();
    };
    timeout = window.setTimeout(() => finish(new Error('YouTube player loading timed out.')), 12_000);
    w.onYouTubeIframeAPIReady = ready;
    const created = !script;
    if (!script) {
      script = document.createElement('script');
      script.src = 'https://www.youtube.com/iframe_api';
      script.async = true;
    }
    script.addEventListener('error', onError, { once: true });
    if (created) document.head.appendChild(script);
  });

  const pending = apiPromise;
  const tracked = pending.catch((error) => {
    if (apiPromise === tracked) apiPromise = null;
    throw error;
  });
  apiPromise = tracked;
  return tracked;
}
