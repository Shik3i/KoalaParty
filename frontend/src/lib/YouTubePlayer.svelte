<script lang="ts">
  import { onMount } from 'svelte';
  let {
    enabled = false,
    videoId = null,
    status = 'paused',
    position = 0,
    onEnded = () => {},
  }: {
    enabled?: boolean;
    videoId?: string | null;
    status?: string;
    position?: number;
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
        },
        onStateChange: (e: any) => {
          if (e.data === 0) onEnded();
        },
        onError: () => {
          playerError = 'This video is unavailable or cannot be embedded.';
        },
      },
    });
  }
  onMount(() => {
    return () => {
      disposed = true;
      player?.destroy();
    };
  });
  $effect(() => {
    if (enabled && !loading && !failed && !player) void initialize();
  });
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
    if (lastVideo !== videoId) {
      const request = { videoId, startSeconds: Math.max(0, position) };
      if (status === 'playing') player.loadVideoById(request);
      else player.cueVideoById(request);
      lastVideo = videoId;
    } else {
      const delta = Math.abs((player.getCurrentTime?.() || 0) - position);
      if (delta > 2.5) player.seekTo(position, true);
      if (status === 'playing') player.playVideo();
      else player.pauseVideo();
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
