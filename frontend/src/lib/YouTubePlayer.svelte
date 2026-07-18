<script lang="ts">
  import { onMount } from 'svelte';
  let {
    enabled = false,
    videoId = null,
    status = 'paused',
    position = 0,
    canControl = true,
    canSeek = true,
    onPlay = () => {},
    onPause = () => {},
    onSeek = () => {},
    onEnded = () => {},
  }: {
    enabled?: boolean;
    videoId?: string | null;
    status?: string;
    position?: number;
    canControl?: boolean;
    canSeek?: boolean;
    onPlay?: (position: number) => void;
    onPause?: (position: number) => void;
    onSeek?: (position: number) => void;
    onEnded?: () => void;
  } = $props();
  let host: HTMLDivElement;
  let player: any = null;
  let disposed = false;
  let loading = false;
  let failed = false;
  let ready = false;
  let lastVideo: string | null = null;
  let playerError = $state('');

  // Sync bookkeeping. The server is authoritative: `expectedPlaying`/`expectedPosition`
  // track the state we last drove the player into so we can tell a genuine user
  // gesture (native play/pause/scrubber) apart from our own programmatic updates.
  const ENDED = 0,
    PLAYING = 1,
    PAUSED = 2;
  const SEEK_THRESHOLD = 2; // unexpected jump (seconds) that counts as a user seek
  const RESYNC_THRESHOLD = 2.5; // drift (seconds) before we re-align a follower
  let expectedPlaying = false;
  let expectedPosition = 0;
  let guardUntil = 0; // suppress seek detection right after we drive the player
  let monitor: ReturnType<typeof setInterval> | null = null;

  function currentTime(): number {
    return player?.getCurrentTime?.() ?? 0;
  }
  function guard(ms = 1400) {
    guardUntil = Date.now() + ms;
  }

  type YTWindow = Window & { YT?: any; onYouTubeIframeAPIReady?: () => void };
  async function loadAPI() {
    const w = window as YTWindow;
    if (w.YT?.Player) return;
    await new Promise<void>((resolve, reject) => {
      const timeout = window.setTimeout(() => reject(new Error('YouTube player loading timed out.')), 12_000);
      const previous = w.onYouTubeIframeAPIReady;
      w.onYouTubeIframeAPIReady = () => {
        previous?.();
        clearTimeout(timeout);
        resolve();
      };
      let script = document.querySelector<HTMLScriptElement>('script[src*="youtube.com/iframe_api"]');
      if (!script) {
        script = document.createElement('script');
        script.src = 'https://www.youtube.com/iframe_api';
        document.head.appendChild(script);
      }
      script.addEventListener(
        'error',
        () => {
          clearTimeout(timeout);
          reject(new Error('YouTube player could not be loaded.'));
        },
        { once: true },
      );
    });
  }
  async function initialize() {
    loading = true;
    try {
      await loadAPI();
    } catch (error) {
      loading = false;
      failed = true;
      playerError = error instanceof Error ? error.message : 'YouTube player could not be loaded.';
      return;
    }
    if (disposed) return;
    const w = window as YTWindow;
    player = new w.YT.Player(host, {
      host: 'https://www.youtube-nocookie.com',
      playerVars: { origin: location.origin, rel: 0 },
      events: {
        onReady: () => {
          ready = true;
          sync();
          startMonitor();
        },
        onStateChange: (e: any) => handleStateChange(e.data),
        onError: () => {
          playerError = 'This video is unavailable or cannot be embedded.';
        },
      },
    });
  }
  // React to the local viewer operating the native player chrome and forward the
  // gesture to the server. If the viewer lacks the capability, snap the player
  // back to the authoritative state instead of emitting.
  function handleStateChange(state: number) {
    if (state === ENDED) {
      onEnded();
      return;
    }
    if (!ready || !lastVideo) return;
    if (state === PLAYING && !expectedPlaying) {
      if (canControl) onPlay(currentTime());
      else {
        guard();
        player.pauseVideo?.();
      }
    } else if (state === PAUSED && expectedPlaying) {
      if (canControl) onPause(currentTime());
      else {
        guard();
        player.playVideo?.();
      }
    }
  }
  function startMonitor() {
    stopMonitor();
    monitor = setInterval(tick, 1000);
  }
  function stopMonitor() {
    if (monitor) clearInterval(monitor);
    monitor = null;
  }
  // YouTube exposes no "seeked" event, so we watch the playhead: a jump larger
  // than one poll interval can explain is a scrubber drag.
  function tick() {
    if (!player || !ready || !lastVideo) return;
    const t = currentTime();
    if (Date.now() < guardUntil) {
      expectedPosition = t;
      return;
    }
    const state = player.getPlayerState?.();
    const tolerance = state === PLAYING ? SEEK_THRESHOLD : 1;
    if ((state === PLAYING || state === PAUSED) && Math.abs(t - expectedPosition) > tolerance) {
      if (canSeek) onSeek(t);
      else {
        guard();
        player.seekTo(expectedPosition, true);
      }
    }
    expectedPosition = t;
  }
  onMount(() => {
    return () => {
      disposed = true;
      stopMonitor();
      player?.destroy();
    };
  });
  $effect(() => {
    if (enabled && !loading && !failed && !player) void initialize();
  });
  function sync() {
    if (!ready) return;
    expectedPlaying = status === 'playing';
    if (!videoId) {
      if (lastVideo) {
        player.stopVideo?.();
        player.clearVideo?.();
        lastVideo = null;
      }
      return;
    }
    if (lastVideo !== videoId) {
      const request = { videoId, startSeconds: Math.max(0, position) };
      guard(3000);
      if (status === 'playing') player.loadVideoById(request);
      else player.cueVideoById(request);
      lastVideo = videoId;
      expectedPosition = Math.max(0, position);
    } else {
      if (Math.abs(currentTime() - position) > RESYNC_THRESHOLD) {
        guard();
        player.seekTo(position, true);
        expectedPosition = position;
      }
      if (status === 'playing') player.playVideo?.();
      else player.pauseVideo?.();
    }
  }
  $effect(() => {
    videoId;
    playerError = '';
  });
  $effect(() => {
    videoId;
    status;
    position;
    sync();
  });
</script>

<div class="player">
  <div bind:this={host}></div>
  {#if playerError}<div class="player-error" role="alert">
      <span>⚠</span>
      <p>{playerError}</p>
      <small>Try another video or reload the room.</small>
    </div>{/if}
  {#if !videoId}<div class="empty">
      <span>▶</span>
      <p>Add a YouTube video to start watching.</p>
    </div>{/if}
</div>

<style>
  .player {
    aspect-ratio: 16/9;
    background: var(--player-background);
    position: relative;
    overflow: hidden;
    border-radius: var(--radius-md);
  }
  .player :global(iframe) {
    width: 100%;
    height: 100%;
    border: 0;
  }
  .empty {
    position: absolute;
    inset: 0;
    display: grid;
    place-content: center;
    text-align: center;
    color: #b9c8bf;
  }
  .player-error {
    position: absolute;
    inset: 0;
    display: grid;
    place-content: center;
    text-align: center;
    color: #f3d7a1;
    background: rgba(5, 8, 6, 0.92);
    padding: 1rem;
    z-index: 2;
  }
  .player-error span {
    font-size: 2rem;
  }
  .player-error p {
    margin: 0.5rem 0;
  }
  .empty span {
    font-size: 2.4rem;
  }
  .empty p {
    margin: 0.5rem;
  }
</style>
