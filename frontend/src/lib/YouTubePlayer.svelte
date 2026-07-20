<script lang="ts">
  import { onMount } from 'svelte';
  import { Play, Warning, Hourglass, SkipForward, SpeakerSimpleSlash } from 'phosphor-svelte';
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
    onDuration = () => {},
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
    onDuration?: (duration: number) => void;
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
    PAUSED = 2,
    BUFFERING = 3;
  const POLL_MS = 500;
  const SEEK_JUMP = 1.5; // discontinuity in the player's own timeline => local scrub
  const DRIFT_MAX = 1.8; // divergence from the expected server position => realign
  let guardUntil = 0; // suppress the monitor right after we drive the player
  let localSeekUntil = 0; // suppress drift correction while our own seek round-trips
  let prevTime = 0; // last observed media time (for discontinuity detection)
  let prevWall = 0; // wall clock at prevTime
  let monitor: ReturnType<typeof setInterval> | null = null;
  // Browsers block autoplay WITH SOUND until the tab has a user gesture, so a
  // passive viewer would otherwise sit on a paused video when someone else presses
  // play. We detect the blocked play, fall back to muted autoplay (always allowed),
  // and surface a one-tap unmute — so the video starts for everyone immediately.
  let autoplayTimer: ReturnType<typeof setTimeout> | null = null;
  let mutedForAutoplay = $state(false);

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

  // Ask the player to start, then verify it actually did. If the browser blocked
  // autoplay-with-sound, retry muted so playback still begins in sync everywhere.
  function requestPlay() {
    player.playVideo?.();
    scheduleAutoplayCheck();
  }
  function scheduleAutoplayCheck() {
    if (autoplayTimer) clearTimeout(autoplayTimer);
    // A single snapshot is fragile: a slow network shows BUFFERING before PLAYING,
    // while a blocked autoplay stays UNSTARTED/CUED/PAUSED. Poll a few times so we
    // only fall back to muted playback once it is clear the sound play never took.
    const check = (attempt: number) => {
      autoplayTimer = null;
      if (disposed || !player || status !== 'playing') return;
      const state = player.getPlayerState?.();
      if (state === PLAYING) return; // playing (with or without sound) — nothing to do
      if (state === BUFFERING && attempt < 3) {
        autoplayTimer = setTimeout(() => check(attempt + 1), 500);
        return;
      }
      // Blocked (or stuck buffering): muted autoplay is always allowed, so start it
      // muted and surface a one-tap unmute, then confirm the muted play took.
      player.mute?.();
      mutedForAutoplay = true;
      player.playVideo?.();
      if (attempt < 3) autoplayTimer = setTimeout(() => check(attempt + 1), 600);
    };
    autoplayTimer = setTimeout(() => check(0), 450);
  }
  function unmute() {
    player?.unMute?.();
    player?.setVolume?.(100);
    mutedForAutoplay = false;
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
  let reportedDuration = 0;
  function tick() {
    if (!player || !ready || !lastVideo) return;
    const now = Date.now();
    const t = currentTime();
    const state = player.getPlayerState?.();
    const duration = player.getDuration?.() ?? 0;
    if (duration > 0 && Math.abs(duration - reportedDuration) > 0.5) {
      reportedDuration = duration;
      onDuration(duration);
    }
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
      if (autoplayTimer) clearTimeout(autoplayTimer);
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
      if (status === 'playing') {
        player.loadVideoById(request);
        scheduleAutoplayCheck();
      } else player.cueVideoById(request);
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
    if (status === 'playing') requestPlay();
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
  {#if mutedForAutoplay && !playerError}<button class="unmute" onclick={unmute}
      ><SpeakerSimpleSlash size={18} weight="fill" /><span>Muted — tap for sound</span></button
    >{/if}
  {#if playerError}<div class="player-error" role="alert">
      <span><Warning size={38} weight="fill" /></span>
      <p>{playerError}</p>
      <small>Try another video or reload the room.</small>
      {#if onSkip}<button class="secondary skip-broken" onclick={onSkip}
          ><SkipForward size={16} weight="fill" />Skip this video</button
        >{/if}
    </div>{/if}
  {#if !videoId}<div class="empty">
      <span
        >{#if hasQueue}<Hourglass size={40} weight="regular" />{:else}<Play size={40} weight="fill" />{/if}</span
      >
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
  .unmute {
    position: absolute;
    left: 50%;
    bottom: 0.9rem;
    transform: translateX(-50%);
    z-index: 3;
    font-size: 0.85rem;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  }
  .unmute:hover {
    transform: translateX(-50%) translateY(-1px);
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
