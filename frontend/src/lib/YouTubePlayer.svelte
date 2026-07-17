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
  let ready = false;
  let lastVideo: string | null = null;
  type YTWindow = Window & { YT?: any; onYouTubeIframeAPIReady?: () => void };
  async function loadAPI() {
    const w = window as YTWindow;
    if (w.YT?.Player) return;
    await new Promise<void>((resolve) => {
      const previous = w.onYouTubeIframeAPIReady;
      w.onYouTubeIframeAPIReady = () => {
        previous?.();
        resolve();
      };
      if (!document.querySelector('script[src*="youtube.com/iframe_api"]')) {
        const s = document.createElement('script');
        s.src = 'https://www.youtube.com/iframe_api';
        document.head.appendChild(s);
      }
    });
  }
  async function initialize() {
    loading = true;
    await loadAPI();
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
    if (enabled && !loading && !player) void initialize();
  });
  function sync() {
    if (!ready || !videoId) return;
    if (lastVideo !== videoId) {
      player.loadVideoById({ videoId, startSeconds: Math.max(0, position) });
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
    status;
    position;
    sync();
  });
</script>

<div class="player">
  <div bind:this={host}></div>
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
  .empty span {
    font-size: 2.4rem;
  }
  .empty p {
    margin: 0.5rem;
  }
</style>
