<script lang="ts">
  import { onMount } from 'svelte';
  let {
    enabled = false,
    videoId = null,
    status = 'paused',
    position = 0,
    positionAt = 0,
    canControl = true,
    canSeek = true,
    hasQueue = false,
    onPlay = () => {},
    onPause = () => {},
    onSeek = () => {},
    onEnded = () => {},
    onSkip = undefined,
  }: {
    enabled?: boolean;
    videoId?: string | null;
    status?: string;
    position?: number;
    positionAt?: number;
    canControl?: boolean;
    canSeek?: boolean;
    hasQueue?: boolean;
    onPlay?: (position: number) => void;
    onPause?: (position: number) => void;
    onSeek?: (position: number) => void;
    onEnded?: () => void;
    onSkip?: (() => void) | undefined;
  } = $props();
  let host: HTMLDivElement;
  let player: any = null;
  let disposed = false;
  let loading = false;
  let failed = false;
  let ready = false;
  let lastVideo: string | null = null;
  let playerError = $state('');

  // The server is authoritative. `status`/`position`/`positionAt` describe the last
  // confirmed playback change: at `positionAt` (client clock) the media was at
  // `position`, advancing since then only while `status === 'playing'`. We never
  // re-baseline on unrelated snapshots, so the expected position stays correct.
  const ENDED = 0,
    PLAYING = 1,
    PAUSED = 2;
  const POLL_MS = 500;
  const SEEK_JUMP = 1.5; // discontinuity in the player's own timeline => local scrub
  const DRIFT_MAX = 1.8; // divergence from the expected server position => realign
  let guardUntil = 0; // suppress the monitor right after we drive the player
  let localSeekUntil = 0; // suppress drift correction while our own seek round-trips
  let prevTime = 0; // last observed media time (for discontinuity detection)
  let prevWall = 0; // wall clock at prevTime
  let monitor: ReturnType<typeof setInterval> | null = null;

  function currentTime(): number {
    return player?.getCurrentTime?.() ?? 0;
  }
  function guard(ms = 1200) {
    guardUntil = Date.now() + ms;
  }
  // Where the media should be right now according to the server.
  function expectedPosition(): number {
    if (status !== 'playing') return Math.max(0, position);
    return Math.max(0, position + (Date.now() - positionAt) / 1000);
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
    if (state === PLAYING && status !== 'playing') {
      if (canControl) onPlay(currentTime());
      else {
        guard();
        player.pauseVideo?.();
      }
    } else if (state === PAUSED && status === 'playing') {
      if (canControl) onPause(currentTime());
      else {
        guard();
        player.playVideo?.();
      }
    }
  }
  function startMonitor() {
    stopMonitor();
    prevTime = currentTime();
    prevWall = Date.now();
    monitor = setInterval(tick, POLL_MS);
  }
  function stopMonitor() {
    if (monitor) clearInterval(monitor);
    monitor = null;
  }
  // YouTube exposes no "seeked" event. We distinguish a local scrub (a discontinuity
  // in the player's OWN timeline) from ordinary drift (divergence from the server's
  // expected position). The first is broadcast; the second is silently corrected.
  function tick() {
    if (!player || !ready || !lastVideo) return;
    const now = Date.now();
    const t = currentTime();
    const state = player.getPlayerState?.();
    if (now < guardUntil) {
      prevTime = t;
      prevWall = now;
      return;
    }
    const playing = state === PLAYING;
    const natural = playing ? (now - prevWall) / 1000 : 0;
    const jump = t - prevTime - natural;
    prevTime = t;
    prevWall = now;
    if (Math.abs(jump) > SEEK_JUMP) {
      if (canSeek) {
        onSeek(t);
        localSeekUntil = now + 4000;
      } else {
        guard();
        player.seekTo(expectedPosition(), true);
      }
      return;
    }
    if (now < localSeekUntil || (state !== PLAYING && state !== PAUSED)) return;
    const expected = expectedPosition();
    if (Math.abs(t - expected) > DRIFT_MAX) {
      guard();
      player.seekTo(expected, true);
      prevTime = expected;
    }
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
  // Runs only when the server reports a real playback change (media, status, or a
  // new position anchor) — never on unrelated snapshots — so it will not fight the
  // monitor's continuous correction.
  function sync() {
    if (!ready) return;
    if (!videoId) {
      if (lastVideo) {
        player.stopVideo?.();
        player.clearVideo?.();
        lastVideo = null;
      }
      return;
    }
    const target = Math.max(0, expectedPosition());
    if (lastVideo !== videoId) {
      guard(3000);
      const request = { videoId, startSeconds: target };
      if (status === 'playing') player.loadVideoById(request);
      else player.cueVideoById(request);
      lastVideo = videoId;
      prevTime = target;
      prevWall = Date.now();
      localSeekUntil = 0;
      return;
    }
    // A confirmed change arrived: stop suppressing correction and realign now.
    localSeekUntil = 0;
    if (Math.abs(currentTime() - target) > DRIFT_MAX) {
      guard();
      player.seekTo(target, true);
      prevTime = target;
      prevWall = Date.now();
    }
    if (status === 'playing') player.playVideo?.();
    else player.pauseVideo?.();
  }
  $effect(() => {
    videoId;
    playerError = '';
  });
  $effect(() => {
    videoId;
    status;
    position;
    positionAt;
    sync();
  });
</script>

<div class="player">
  <div bind:this={host}></div>
  {#if playerError}<div class="player-error" role="alert">
      <span>⚠</span>
      <p>{playerError}</p>
      <small>Try another video or reload the room.</small>
      {#if onSkip}<button class="secondary skip-broken" onclick={onSkip}>Skip this video</button>{/if}
    </div>{/if}
  {#if !videoId}<div class="empty">
      <span>{hasQueue ? '⏳' : '▶'}</span>
      <p>{hasQueue ? 'Nothing playing right now.' : 'Add a YouTube video to start watching.'}</p>
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
  .skip-broken {
    margin-top: 0.9rem;
  }
  .empty span {
    font-size: 2.4rem;
  }
  .empty p {
    margin: 0.5rem;
  }
</style>
