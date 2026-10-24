<script lang="ts">
  import {
    ArrowsIn,
    ArrowsOut,
    CornersIn,
    CornersOut,
    Pause,
    PictureInPicture,
    Play,
    SkipForward,
    Sparkle,
    SpeakerHigh,
    SpeakerLow,
    SpeakerSlash,
  } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import { formatDuration, type Snapshot } from '$lib/room';
  import AmbilightPopover from './AmbilightPopover.svelte';
  import { volume, muted, setVolume, toggleMute } from '$lib/volume';

  let {
    room,
    caps,
    commandPending,
    position,
    duration,
    rate,
    playing,
    heat,
    miniPlayer,
    fullscreen,
    theater,
    onTogglePlay,
    onSeek,
    onRate,
    onSkip,
    onVoteSkip,
    onFullscreen,
    onTheater,
    onMiniPlayer,
  }: {
    room: Snapshot;
    caps: Record<string, boolean>;
    commandPending: boolean;
    position: number;
    duration: number;
    rate: number;
    playing: boolean;
    heat: Record<number, number>;
    miniPlayer: boolean;
    fullscreen: boolean;
    theater: boolean;
    onTogglePlay: () => void;
    onSeek: (position: number) => void;
    onRate: (rate: number) => void;
    onSkip: () => void;
    onVoteSkip: () => void;
    onFullscreen: () => void;
    onTheater: () => void;
    onMiniPlayer: () => void;
  } = $props();

  const HEAT_BUCKET = 5;
  let ambilightOpen = $state(false);
  const pct = $derived(duration > 0 ? Math.min(100, (position / duration) * 100) : 0);
  // Reaction density along the timeline: where the room laughed, cheered or gasped.
  const heatBars = $derived.by(() => {
    if (duration <= 0) return [];
    const entries = Object.entries(heat).map(([bucket, count]) => [Number(bucket), count] as const);
    const peak = Math.max(1, ...entries.map(([, count]) => count));
    return entries
      .filter(([bucket]) => bucket * HEAT_BUCKET < duration)
      .map(([bucket, count]) => ({
        left: ((bucket * HEAT_BUCKET) / duration) * 100,
        width: Math.max(0.6, (HEAT_BUCKET / duration) * 100),
        height: 30 + (count / peak) * 70,
        count,
        time: formatDuration(bucket * HEAT_BUCKET),
      }));
  });
</script>

<div class="player-bar">
  <button
    class="play-toggle"
    aria-label={playing ? $t('player.pause') : $t('player.play')}
    onclick={onTogglePlay}
    disabled={commandPending || !caps['playback.play_pause'] || !room.playback.media}
    >{#if playing}<Pause size={18} weight="fill" />{:else}<Play size={18} weight="fill" />{/if}</button
  >
  <div class="volume-control" class:muted={$muted || $volume === 0}>
    <button
      type="button"
      class="secondary icon-button volume-btn"
      aria-label={$muted ? $t('player.unmute') : $t('player.mute')}
      title={$muted ? $t('player.unmute') : $t('player.mute')}
      onclick={toggleMute}
    >
      {#if $muted || $volume === 0}
        <SpeakerSlash size={17} weight="bold" />
      {:else if $volume < 50}
        <SpeakerLow size={17} weight="bold" />
      {:else}
        <SpeakerHigh size={17} weight="bold" />
      {/if}
    </button>
    <div class="volume-slider-wrap">
      <input
        type="range"
        class="volume-slider"
        min="0"
        max="100"
        step="1"
        value={$muted ? 0 : $volume}
        aria-label={$t('player.volume')}
        oninput={(e) => setVolume(Number(e.currentTarget.value))}
      />
    </div>
  </div>
  <div class="scrubber-area">
    {#if room.playback.media}<div
        class="scrubber"
        role="progressbar"
        aria-label={$t('player.progress')}
        aria-valuemin="0"
        aria-valuemax={Math.max(1, Math.round(duration || position || 1))}
        aria-valuenow={Math.min(Math.max(0, Math.round(position)), Math.max(1, Math.round(duration || position || 1)))}
        aria-valuetext={duration > 0
          ? $t('player.progressOf', { position: formatDuration(position), duration: formatDuration(duration) })
          : formatDuration(position)}
      >
        {#if heatBars.length}<div class="heat" aria-hidden="true">
            {#each heatBars as bar (bar.left)}<span
                style={`left:${bar.left}%;width:${bar.width}%;height:${bar.height}%`}
                title={$t('player.heat', { count: bar.count, time: bar.time })}
              ></span>{/each}
          </div>{/if}
        <div class="scrubber-track">
          <div class="scrubber-fill" style="width:{pct}%"></div>
          {#if caps['playback.seek'] && duration > 0}<input
              class="scrubber-input"
              type="range"
              min="0"
              max={Math.floor(duration)}
              step="1"
              value={Math.floor(Math.min(position, duration))}
              aria-label={$t('player.seekEveryone')}
              onchange={(event) => onSeek(Number(event.currentTarget.value))}
            />{/if}
        </div>
        <div class="scrubber-time">
          <span>{formatDuration(position)}</span><span>{duration > 0 ? formatDuration(duration) : '–:--'}</span>
        </div>
      </div>{:else}<p class="idle-note">{$t('player.idle')}</p>{/if}
  </div>
  {#if room.playback.media && !miniPlayer}<select
      class="speed-control"
      aria-label={$t('player.speed')}
      title={$t('player.speedHint')}
      value={rate}
      disabled={commandPending || !caps['playback.play_pause']}
      onchange={(event) => onRate(Number(event.currentTarget.value))}
      >{#each [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2] as option}<option value={option}>{option}×</option
        >{/each}</select
    >{/if}
  {#if room.playback.media}{#if caps['queue.skip']}<button
        class="secondary bar-button"
        aria-label={$t('player.skipNext')}
        title={$t('player.skipNext')}
        onclick={onSkip}
        disabled={commandPending}
        ><SkipForward size={17} weight="fill" /><span class="bar-label">{$t('player.skip')}</span></button
      >{:else if caps['queue.vote']}<button
        class="secondary bar-button"
        class:active={room.playback.skipVoted}
        aria-pressed={room.playback.skipVoted}
        title={$t('player.voteSkipHint')}
        onclick={onVoteSkip}
        ><SkipForward size={17} weight="fill" /><span
          >{$t('player.voteSkip', { votes: room.playback.skipVotes, needed: room.playback.skipNeeded })}</span
        ></button
      >{/if}{/if}
  <button
    class="secondary bar-button"
    aria-label={fullscreen ? $t('player.exitFullscreen') : $t('player.fullscreen')}
    title={$t('player.fullscreenHint')}
    onclick={onFullscreen}
    >{#if fullscreen}<CornersIn size={17} weight="bold" />{:else}<CornersOut size={17} weight="bold" />{/if}</button
  ><button
    class="secondary bar-button theater-toggle"
    aria-pressed={theater}
    aria-label={theater ? $t('player.exitTheater') : $t('player.theater')}
    title={theater ? $t('player.exitTheater') : $t('player.theater')}
    onclick={onTheater}
    >{#if theater}<ArrowsIn size={17} weight="bold" />{:else}<ArrowsOut size={17} weight="bold" />{/if}</button
  >
  <div class="ambilight-anchor">
    <button
      class="secondary bar-button"
      class:active={ambilightOpen}
      aria-pressed={ambilightOpen}
      aria-label={$t('ambilight.title')}
      title={$t('ambilight.title')}
      onclick={() => (ambilightOpen = !ambilightOpen)}><Sparkle size={17} weight="bold" /></button
    >
    {#if ambilightOpen}
      <AmbilightPopover onClose={() => (ambilightOpen = false)} />
    {/if}
  </div>
  <button
    class="secondary bar-button"
    aria-pressed={miniPlayer}
    aria-label={miniPlayer ? $t('player.dock') : $t('player.mini')}
    title={miniPlayer ? $t('player.dock') : $t('player.mini')}
    onclick={onMiniPlayer}><PictureInPicture size={17} weight="bold" /></button
  >
</div>

<style>
  .ambilight-anchor {
    position: relative;
    display: flex;
    align-items: center;
  }
  .player-bar {
    display: flex;
    align-items: center;
    gap: 0.55rem;
  }
  .play-toggle {
    flex: 0 0 auto;
    width: 2.8rem;
    height: 2.8rem;
    padding: 0;
    border-radius: 50%;
  }
  .volume-control {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    position: relative;
  }
  .volume-btn {
    padding: 0.55rem;
    border-radius: var(--radius-sm);
  }
  .volume-slider-wrap {
    width: 0;
    opacity: 0;
    overflow: hidden;
    transition:
      width 0.22s ease,
      opacity 0.22s ease;
    display: flex;
    align-items: center;
  }
  .volume-control:hover .volume-slider-wrap,
  .volume-control:focus-within .volume-slider-wrap {
    width: 74px;
    opacity: 1;
    margin-right: 0.35rem;
  }
  .volume-slider {
    width: 70px;
    height: 4px;
    accent-color: var(--accent-primary);
    cursor: pointer;
    margin: 0;
  }
  .scrubber-area {
    flex: 1;
    min-width: 0;
    padding: 0 0.3rem;
  }
  .scrubber {
    position: relative;
  }
  .heat {
    position: absolute;
    left: 0;
    right: 0;
    bottom: calc(100% - 0.9rem);
    height: 1.1rem;
    pointer-events: none;
  }
  .heat span {
    position: absolute;
    bottom: 0;
    border-radius: 3px 3px 0 0;
    background: linear-gradient(to top, color-mix(in srgb, var(--accent-primary) 80%, transparent), transparent);
    opacity: 0.85;
  }
  .scrubber-track {
    position: relative;
    height: 6px;
    border-radius: 999px;
    background: var(--surface-hover);
  }
  .scrubber-fill {
    height: 100%;
    border-radius: 999px;
    background: linear-gradient(90deg, var(--accent-primary), var(--accent-hover));
    transition: width 0.5s linear;
  }
  .scrubber-input {
    position: absolute;
    inset: -8px 0;
    width: 100%;
    height: calc(100% + 16px);
    opacity: 0;
    cursor: pointer;
    margin: 0;
    padding: 0;
  }
  .scrubber-area:hover .scrubber-track {
    height: 8px;
  }
  .scrubber-time {
    display: flex;
    justify-content: space-between;
    margin-top: 0.35rem;
    font-size: 0.72rem;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .idle-note {
    margin: 0;
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .bar-button {
    flex: 0 0 auto;
    padding: 0.55rem 0.7rem;
  }
  .bar-button.active {
    color: var(--accent-primary);
    border-color: var(--accent-primary);
  }
  .speed-control {
    flex: 0 0 5.2rem;
    width: 5.2rem;
    padding: 0.5rem;
    border-radius: var(--radius-md);
    border: 1px solid var(--border-subtle);
    background: var(--surface-elevated);
    color: inherit;
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .speed-control:disabled {
    cursor: default;
    opacity: 0.6;
  }
  @media (max-width: 1100px) {
    .bar-label {
      display: none;
    }
  }
  @media (max-width: 580px) {
    .volume-slider-wrap {
      display: none !important;
    }
    .player-bar {
      gap: 0.35rem;
    }
    .play-toggle {
      width: 2.5rem;
      height: 2.5rem;
    }
    .bar-button {
      padding: 0.5rem 0.55rem;
    }
    .theater-toggle {
      display: none;
    }
    .speed-control {
      flex-basis: 4.2rem;
      width: 4.2rem;
    }
  }
</style>
