<script lang="ts">
  import { onMount } from 'svelte';
  import { Play, Warning, Hourglass, SkipForward, SpeakerSimpleSlash } from 'phosphor-svelte';
  import {
    isCurrentVideoError,
    isLocalTimelineJump,
    isRetryablePlayerError,
    isUnboundedTimeline,
    normalizedDuration,
    PLAYER_STATE,
    playerErrorMessage,
    shouldBaselineTimeline,
    stateChangeAction,
    timelineJump,
  } from '$lib/playerSync';
  import { createDiagnosticEvent, type DiagnosticEvent } from '$lib/diagnostics';
  import { loadYouTubeAPI } from '$lib/youtubeApi';
  import type { SponsorSegment } from '$lib/room';
  let {
    enabled = false,
    videoId = null,
    mediaId = null,
    playbackRevision = 0,
    status = 'paused',
    position = 0,
    positionAt = 0,
    rate = 1,
    segments = [],
    canControl = true,
    canSeek = true,
    hasQueue = false,
    onPlay = () => {},
    onPause = () => {},
    onSeek = () => {},
    onRate = () => {},
    onSponsorSkip = () => {},
    onEnded = () => {},
    onSkip = undefined,
    onDuration = () => {},
    onDiagnostics = () => {},
    onDiagnosticEvent = () => {},
  }: {
    enabled?: boolean;
    videoId?: string | null;
    mediaId?: string | null;
    playbackRevision?: number;
    status?: string;
    position?: number;
    positionAt?: number;
    rate?: number;
    segments?: SponsorSegment[];
    canControl?: boolean;
    canSeek?: boolean;
    hasQueue?: boolean;
    onPlay?: (position: number) => void;
    onPause?: (position: number) => void;
    onSeek?: (position: number) => void;
    onRate?: (rate: number, position: number) => void;
    onSponsorSkip?: (segment: SponsorSegment) => void;
    onEnded?: (mediaId: string) => void;
    onSkip?: ((mediaId: string) => void) | undefined;
    onDuration?: (duration: number) => void;
    onDiagnostics?: (diagnostics: { drift: number; state: string; correctedAt: number | null }) => void;
    onDiagnosticEvent?: (event: DiagnosticEvent) => void;
  } = $props();
  let host: HTMLDivElement;
  let player = $state<any>(null);
  let disposed = false;
  let loading = $state(false);
  let failed = $state(false);
  let ready = $state(false);
  let lastVideo: string | null = null;
  let lastMediaId = $state<string | null>(null);
  let confirmedVideo: string | null = null;
  let playerError = $state('');
  let playerErrorCode = $state<number | null>(null);
  let playerState = $state<number | null>(null);

  // The server is authoritative. `status`/`position`/`positionAt` describe the last
  // confirmed playback change: at `positionAt` (client clock) the media was at
  // `position`, advancing since then only while `status === 'playing'`. We never
  // re-baseline on unrelated snapshots, so the expected position stays correct.
  const { PLAYING, PAUSED, BUFFERING } = PLAYER_STATE;
  const POLL_MS = 500;
  const READY_TIMEOUT_MS = 15_000;
  const START_TIMEOUT_MS = 10_000;
  const TIMELINE_RECOVERY_MS = 1_200;
  const SEEK_JUMP = 1.5; // discontinuity in the player's own timeline => local scrub
  const DRIFT_MAX = 1.8; // divergence from the expected server position => realign
  let guardUntil = 0; // suppress the monitor right after we drive the player
  let localSeekUntil = 0; // suppress drift correction while our own seek round-trips
  let prevTime = 0; // last observed media time (for discontinuity detection)
  let prevWall = 0; // wall clock at prevTime
  let previousTimelineState: number | null = null;
  let timelineRecoveryUntil = 0;
  let monitor: ReturnType<typeof setInterval> | null = null;
  // Browsers block autoplay WITH SOUND until the tab has a user gesture, so a
  // passive viewer would otherwise sit on a paused video when someone else presses
  // play. We detect the blocked play, fall back to muted autoplay (always allowed),
  // and surface a one-tap unmute — so the video starts for everyone immediately.
  let autoplayTimer: ReturnType<typeof setTimeout> | null = null;
  let readyTimer: ReturnType<typeof setTimeout> | null = null;
  let watchdogTimer: ReturnType<typeof setTimeout> | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let retryCount = 0;
  let loadToken = 0;
  let mutedForAutoplay = $state(false);
  let correctedAt: number | null = null;

  function emitDiagnostic(event: string, details: Record<string, string | number | boolean | null> = {}) {
    onDiagnosticEvent(
      createDiagnosticEvent('player', event, {
        videoId: lastVideo,
        mediaId: lastMediaId,
        playbackRevision,
        status,
        ready,
        playerState,
        online: typeof navigator === 'undefined' ? null : navigator.onLine,
        visible: typeof document === 'undefined' ? null : document.visibilityState === 'visible',
        ...details,
      }),
    );
  }

  function clearRecoveryTimers() {
    if (autoplayTimer) clearTimeout(autoplayTimer);
    if (readyTimer) clearTimeout(readyTimer);
    if (watchdogTimer) clearTimeout(watchdogTimer);
    if (retryTimer) clearTimeout(retryTimer);
    autoplayTimer = null;
    readyTimer = null;
    watchdogTimer = null;
    retryTimer = null;
  }

  function currentTime(): number {
    return player?.getCurrentTime?.() ?? 0;
  }
  function guard(ms = 1200) {
    // Never shorten an existing, longer guard: a short guard (e.g. applying the rate
    // or the muted-autoplay fallback) must not cut the 3s video-load window short.
    guardUntil = Math.max(guardUntil, Date.now() + ms);
  }
  // Where the media should be right now according to the server. While playing the
  // media advances `rate` seconds per wall-clock second, so scale elapsed time by it.
  function expectedPosition(): number {
    if (status !== 'playing') return Math.max(0, position);
    return Math.max(0, position + ((Date.now() - positionAt) / 1000) * (rate || 1));
  }

  // Ask the player to start, then verify it actually did. If the browser blocked
  // autoplay-with-sound, retry muted so playback still begins in sync everywhere.
  function requestPlay() {
    // Starting playback is our own programmatic action. Guard the resulting state
    // changes so a blocked autoplay (reported as PAUSED) is not relayed to the room
    // as a real pause — otherwise pressing play on an already-loaded video would stop
    // it for everyone whose tab has no user gesture yet.
    guard();
    player.playVideo?.();
    scheduleAutoplayCheck();
    scheduleStartWatchdog('play_request');
  }

  function scheduleStartWatchdog(reason: string) {
    if (watchdogTimer) clearTimeout(watchdogTimer);
    if (status !== 'playing' || !lastVideo) return;
    const token = loadToken;
    watchdogTimer = setTimeout(() => {
      watchdogTimer = null;
      if (disposed || token !== loadToken || !player || status !== 'playing' || !lastVideo) return;
      const state = player.getPlayerState?.();
      if (state === PLAYING || state === PAUSED) return;
      emitDiagnostic('stalled', { reason, state: state ?? null });
      if (retryCount < 1) {
        retryCurrentVideo('start_watchdog');
      } else {
        playerError = 'YouTube did not start this video. Try again or skip it.';
        playerErrorCode = null;
      }
    }, START_TIMEOUT_MS);
  }

  function startMutedAutoplay(reason: string) {
    if (!player || status !== 'playing') return;
    guard();
    player.mute?.();
    mutedForAutoplay = true;
    emitDiagnostic('autoplay_fallback', { reason });
    player.playVideo?.();
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
      // muted and surface a one-tap unmute, then confirm the muted play took. Re-arm
      // the guard so the resulting state changes are recognised as our own and never
      // relayed to the room, even when a slow network resolves them late.
      startMutedAutoplay('autoplay_check');
      if (attempt < 3) autoplayTimer = setTimeout(() => check(attempt + 1), 600);
    };
    autoplayTimer = setTimeout(() => check(0), 450);
  }

  function retryCurrentVideo(reason = 'manual') {
    if (!player || !ready || !lastVideo) {
      retryInitialization();
      return;
    }
    retryCount += 1;
    loadToken += 1;
    clearRecoveryTimers();
    playerError = '';
    playerErrorCode = null;
    confirmedVideo = null;
    guard(3000);
    emitDiagnostic('retry', { reason, retryCount });
    const request = { videoId: lastVideo, startSeconds: Math.max(0, expectedPosition()) };
    if (status === 'playing') {
      player.loadVideoById(request);
      scheduleAutoplayCheck();
      scheduleStartWatchdog('retry');
    } else {
      player.cueVideoById(request);
    }
  }
  function unmute() {
    player?.unMute?.();
    player?.setVolume?.(100);
    mutedForAutoplay = false;
  }
  // Bring the player's playback speed in line with the authoritative rate. Guarded so
  // the resulting onPlaybackRateChange is recognised as our own and not relayed back.
  function applyRate() {
    const target = rate || 1;
    if (Math.abs((player.getPlaybackRate?.() ?? 1) - target) > 0.01) {
      guard();
      player.setPlaybackRate?.(target);
    }
  }

  type YTWindow = Window & { YT?: any };
  async function loadAPI() {
    await loadYouTubeAPI();
  }
  async function initialize() {
    loading = true;
    failed = false;
    emitDiagnostic('initialize_started');
    try {
      await loadAPI();
    } catch (error) {
      loading = false;
      failed = true;
      playerError = error instanceof Error ? error.message : 'YouTube player could not be loaded.';
      emitDiagnostic('initialize_failed', { message: playerError });
      return;
    }
    if (disposed) {
      loading = false;
      return;
    }
    const w = window as YTWindow;
    const token = ++loadToken;
    try {
      player = new w.YT.Player(host, {
        host: 'https://www.youtube-nocookie.com',
        // cc_load_policy: 0 stops us forcing captions on. Whether captions still appear
        // then depends on the viewer's own YouTube/browser caption preference, which we
        // cannot override; unloadModule below is a best-effort hide on top of that.
        playerVars: { origin: location.origin, rel: 0, cc_load_policy: 0 },
        events: {
          onReady: () => {
            if (token !== loadToken || disposed) return;
            if (readyTimer) clearTimeout(readyTimer);
            readyTimer = null;
            loading = false;
            failed = false;
            try {
              player.getIframe?.()?.setAttribute('allow', 'autoplay; encrypted-media; picture-in-picture');
            } catch {
              /* iframe permissions are best-effort across browser versions */
            }
            // Best-effort: hide auto-captions that would otherwise show by default. The
            // viewer can always re-enable them with the player's CC button.
            try {
              player.unloadModule?.('captions');
              player.unloadModule?.('cc');
            } catch {
              /* module may not be loaded yet; harmless */
            }
            ready = true;
            emitDiagnostic('ready');
            sync();
            startMonitor();
          },
          onAutoplayBlocked: () => {
            emitDiagnostic('autoplay_blocked');
            startMutedAutoplay('youtube_event');
          },
          onStateChange: (e: any) => handleStateChange(e.data),
          onPlaybackRateChange: (e: any) => handleRateChange(e.data),
          onError: (e: any) => {
            const iframeVideo = player?.getVideoData?.()?.video_id ?? '';
            // The IFrame API exposes no event-local video ID. Ignore an error when
            // the iframe is still reporting a different media item; that is a late
            // callback from the previous load and must not cover the replacement.
            // During the first error callback getVideoData() can still be empty, so
            // an empty ID is accepted for the current request and remains visible
            // until a successful PLAYING event clears it.
            if (!isCurrentVideoError(lastVideo, iframeVideo)) return;
            const code = Number(e?.data);
            playerErrorCode = Number.isFinite(code) ? code : null;
            emitDiagnostic('error', { code: playerErrorCode });
            if (isRetryablePlayerError(code) && status === 'playing' && retryCount < 1) {
              playerError = 'Playback interrupted. Retrying…';
              retryTimer = setTimeout(() => retryCurrentVideo('player_error'), 450);
            } else {
              playerError = playerErrorMessage(code);
            }
          },
        },
      });
      readyTimer = setTimeout(() => {
        readyTimer = null;
        if (token !== loadToken || ready || disposed) return;
        failed = true;
        loading = false;
        playerError = 'YouTube player did not initialize. Try again.';
        emitDiagnostic('ready_timeout');
        player?.destroy?.();
        player = null;
      }, READY_TIMEOUT_MS);
    } catch (error) {
      loading = false;
      failed = true;
      player = null;
      playerError = error instanceof Error ? error.message : 'YouTube player could not be initialized.';
      emitDiagnostic('initialize_failed', { message: playerError });
    }
  }
  // React to the local viewer operating the native player chrome and forward the
  // gesture to the server. If the viewer lacks the capability, snap the player
  // back to the authoritative state instead of emitting.
  function handleStateChange(state: number) {
    playerState = state;
    const iframeVideo = player?.getVideoData?.()?.video_id ?? '';
    if (iframeVideo === lastVideo && (state === PLAYING || state === PAUSED)) {
      confirmedVideo = lastVideo;
    }
    if (state === PLAYING) {
      playerError = '';
      playerErrorCode = null;
      retryCount = 0;
      if (watchdogTimer) clearTimeout(watchdogTimer);
      watchdogTimer = null;
      emitDiagnostic('playing');
    } else if (state === BUFFERING && status === 'playing') {
      timelineRecoveryUntil = Math.max(timelineRecoveryUntil, Date.now() + TIMELINE_RECOVERY_MS);
      emitDiagnostic('buffering');
      scheduleStartWatchdog('buffering');
    }
    const action = stateChangeAction({
      state,
      serverStatus: status,
      guarded: Date.now() < guardUntil,
      ready,
      hasVideo: !!lastVideo,
      videoMatches: iframeVideo === lastVideo && confirmedVideo === lastVideo,
      currentTime: currentTime(),
      duration: normalizedDuration(player?.getDuration?.() ?? 0),
      canControl,
    });
    switch (action) {
      case 'ended':
        emitDiagnostic('ended');
        if (lastMediaId) onEnded(lastMediaId);
        return;
      case 'emit-play':
        onPlay(currentTime());
        return;
      case 'emit-pause':
        onPause(currentTime());
        return;
      case 'snap-pause':
        guard();
        player.pauseVideo?.();
        return;
      case 'snap-play':
        guard();
        player.playVideo?.();
        return;
    }
  }
  // The local viewer changed playback speed through the native player menu. Forward it
  // if they may control playback, otherwise snap back to the authoritative rate. A
  // change inside the guard window is our own setPlaybackRate echo and is ignored.
  function handleRateChange(newRate: number) {
    if (!ready || !lastVideo || Date.now() < guardUntil) return;
    if (Math.abs(newRate - (rate || 1)) < 0.01) return;
    if (canControl) onRate(newRate, currentTime());
    else {
      guard();
      player.setPlaybackRate?.(rate || 1);
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
  function retryInitialization() {
    clearRecoveryTimers();
    loadToken += 1;
    player?.destroy?.();
    player = null;
    ready = false;
    loading = false;
    failed = false;
    playerError = '';
    playerErrorCode = null;
    emitDiagnostic('initialize_retry');
    void initialize();
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
    const previousState = previousTimelineState;
    previousTimelineState = state;
    if (state === BUFFERING) {
      timelineRecoveryUntil = Math.max(timelineRecoveryUntil, now + TIMELINE_RECOVERY_MS);
    }
    const rawDuration = player.getDuration?.() ?? 0;
    const duration = normalizedDuration(rawDuration);
    if (duration > 0 && Math.abs(duration - reportedDuration) > 0.5) {
      reportedDuration = duration;
      onDuration(duration);
    }
    // Live streams expose an absolute timeline (for example several million
    // seconds since the stream began), not the room-relative VOD position. A
    // drift correction against that value seeks the live stream back to 0 every
    // polling interval, which looks like a two-second loop to every viewer.
    if (isUnboundedTimeline(rawDuration, t)) {
      prevTime = t;
      prevWall = now;
      onDiagnostics({ drift: 0, state: 'live', correctedAt });
      return;
    }
    if (now < guardUntil) {
      prevTime = t;
      prevWall = now;
      return;
    }
    const playing = state === PLAYING;
    // SponsorBlock: while playing, jump over any segment the current time falls in.
    // Only viewers who may seek emit the skip; everyone else follows the broadcast
    // seek. The local jump keeps it snappy; the guard stops it being read as a scrub,
    // and also prevents re-firing on the segment we just left.
    if (playing && canSeek && segments.length) {
      const seg = segments.find((s) => t >= s.start && t < s.end - 0.4);
      if (seg) {
        guard();
        player.seekTo(seg.end, true);
        prevTime = seg.end;
        prevWall = now;
        onSponsorSkip(seg);
        return;
      }
    }
    // YouTube is allowed to reset its reported time while buffering or replacing
    // media. Baseline those transitions without treating them as a user scrub;
    // otherwise the transient 0-second reset is broadcast and every client loops.
    if (shouldBaselineTimeline(state, previousState, now, timelineRecoveryUntil)) {
      prevTime = t;
      prevWall = now;
      return;
    }
    const jump = timelineJump(t, prevTime, (now - prevWall) / 1000, playing, rate || 1);
    prevTime = t;
    prevWall = now;
    if (isLocalTimelineJump(state, jump, SEEK_JUMP)) {
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
    const drift = t - expected;
    onDiagnostics({
      drift,
      state:
        state === BUFFERING ? 'buffering' : state === PLAYING ? 'playing' : state === PAUSED ? 'paused' : 'loading',
      correctedAt,
    });
    if (Math.abs(drift) > DRIFT_MAX) {
      guard();
      player.seekTo(expected, true);
      prevTime = expected;
      correctedAt = now;
    }
  }
  onMount(() => {
    const onOnline = () => {
      emitDiagnostic('online');
      if (status === 'playing') requestPlay();
    };
    const onOffline = () => emitDiagnostic('offline');
    const onVisibility = () => {
      emitDiagnostic(document.visibilityState === 'visible' ? 'visible' : 'hidden');
      if (document.visibilityState === 'visible' && status === 'playing') requestPlay();
    };
    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOffline);
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      disposed = true;
      stopMonitor();
      clearRecoveryTimers();
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOffline);
      document.removeEventListener('visibilitychange', onVisibility);
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
        clearRecoveryTimers();
        mutedForAutoplay = false;
        retryCount = 0;
        playerError = '';
        playerErrorCode = null;
        reportedDuration = 0;
        onDuration(0);
        // Invalidate the old media before asking the iframe to stop. YouTube may
        // synchronously emit ENDED/PAUSED while clearing a video.
        lastVideo = null;
        lastMediaId = null;
        confirmedVideo = null;
        previousTimelineState = null;
        timelineRecoveryUntil = 0;
        player.stopVideo?.();
        player.clearVideo?.();
      }
      return;
    }
    if (lastMediaId !== mediaId) lastMediaId = mediaId;
    const target = Math.max(0, expectedPosition());
    if (lastVideo !== videoId) {
      clearRecoveryTimers();
      mutedForAutoplay = false;
      retryCount = 0;
      playerError = '';
      playerErrorCode = null;
      loadToken += 1;
      reportedDuration = 0;
      onDuration(0);
      // Publish the new identity before calling the iframe. A synchronous state
      // callback from load/cue must be associated with the new request, while a
      // delayed callback for the old request will fail the video-ID check.
      lastVideo = videoId;
      lastMediaId = mediaId;
      confirmedVideo = null;
      previousTimelineState = null;
      timelineRecoveryUntil = 0;
      guard(3000);
      const request = { videoId, startSeconds: target };
      if (status === 'playing') {
        player.loadVideoById(request);
        scheduleAutoplayCheck();
        scheduleStartWatchdog('media_load');
      } else player.cueVideoById(request);
      applyRate();
      prevTime = target;
      prevWall = Date.now();
      localSeekUntil = 0;
      return;
    }
    // A confirmed change arrived: stop suppressing correction and realign now.
    localSeekUntil = 0;
    applyRate();
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
    rate;
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
      <small
        >{playerErrorCode === 153
          ? 'The embedded player identity could not be verified.'
          : 'Playback can recover after a retry or a different video.'}</small
      >
      <div class="player-error-actions">
        {#if player && ready}<button class="secondary" onclick={() => retryCurrentVideo('manual')}>Try again</button
          >{:else}<button class="secondary" onclick={retryInitialization}>Reload player</button>{/if}
        {#if onSkip && lastMediaId}<button class="secondary skip-broken" onclick={() => onSkip(lastMediaId!)}
            ><SkipForward size={16} weight="fill" />Skip this video</button
          >{/if}
      </div>
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
    margin-top: 0;
  }
  .player-error-actions {
    display: flex;
    justify-content: center;
    gap: 0.6rem;
    margin-top: 0.9rem;
    flex-wrap: wrap;
  }
  .empty span {
    font-size: 2.4rem;
  }
  .empty p {
    margin: 0.5rem;
  }
</style>
