<script lang="ts">
  import { ambilight, extractPalette } from '$lib/ambilight';

  let {
    thumbnail = null,
    videoId = null,
    playing = false,
  }: {
    thumbnail?: string | null;
    videoId?: string | null;
    playing?: boolean;
  } = $props();

  let palette = $state<[string, string, string, string]>(['#00e5ff', '#a855f7', '#ff7a00', '#10b981']);

  $effect(() => {
    let active = true;
    if ($ambilight.mode === 'dynamic-thumbnail') {
      void extractPalette(thumbnail || '', videoId || 'koala').then((colors) => {
        if (active) palette = colors;
      });
    }
    return () => {
      active = false;
    };
  });
</script>

{#if $ambilight.mode !== 'off'}
  <div
    class="ambient-container"
    aria-hidden="true"
    style="--blur: {$ambilight.blur}px; --opacity: {$ambilight.intensity};"
  >
    {#if $ambilight.mode === 'thumbnail' && thumbnail}
      <div class="ambient ambient-thumb" style="background-image: url('{thumbnail}');"></div>
    {:else if $ambilight.mode === 'static'}
      <div class="ambient ambient-static" style="--color: {$ambilight.color};"></div>
    {:else if $ambilight.mode === 'dynamic-thumbnail'}
      <div
        class="ambient ambient-adaptive"
        class:playing
        style="--c1: {palette[0]}; --c2: {palette[1]}; --c3: {palette[2]}; --c4: {palette[3]};"
      ></div>
    {:else}
      <!-- Dynamic Aurora -->
      <div class="ambient ambient-aurora" class:playing>
        <div class="aurora-blob aurora-1"></div>
        <div class="aurora-blob aurora-2"></div>
        <div class="aurora-blob aurora-3"></div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .ambient-container {
    position: absolute;
    inset: -3% -1.5%;
    z-index: -1;
    pointer-events: none;
    border-radius: 2rem;
    overflow: visible;
    transform: translateZ(0);
    contain: layout paint;
  }

  .ambient {
    position: absolute;
    inset: 0;
    filter: blur(var(--blur, 52px)) saturate(1.8) brightness(1.1);
    opacity: var(--opacity, 0.75);
    transform: scale(1.08) translateZ(0);
    will-change: transform, opacity;
    backface-visibility: hidden;
    transition:
      opacity 0.5s ease,
      filter 0.5s ease;
    border-radius: 2rem;
  }

  /* 1. Thumbnail mode */
  .ambient-thumb {
    background-size: cover;
    background-position: center;
  }

  /* 2. Static Custom Color */
  .ambient-static {
    background: radial-gradient(
      ellipse at center,
      var(--color) 0%,
      color-mix(in srgb, var(--color) 70%, transparent) 45%,
      transparent 80%
    );
  }

  /* 3. Adaptive Video-Thumbnail Mode */
  .ambient-adaptive {
    background:
      radial-gradient(circle at 10% 15%, var(--c1) 0%, transparent 60%),
      radial-gradient(circle at 90% 15%, var(--c2) 0%, transparent 60%),
      radial-gradient(circle at 15% 85%, var(--c3) 0%, transparent 60%),
      radial-gradient(circle at 85% 85%, var(--c4) 0%, transparent 60%);
    transition:
      background 1.2s ease,
      opacity 0.6s ease;
  }

  .ambient-adaptive.playing {
    animation: pulseAdaptive 9s ease-in-out infinite alternate;
  }

  @keyframes pulseAdaptive {
    0% {
      transform: scale(1.06) rotate(0deg);
      filter: blur(var(--blur, 52px)) saturate(1.8) brightness(1.05);
    }
    50% {
      transform: scale(1.12) rotate(1.5deg);
      filter: blur(calc(var(--blur, 52px) * 1.15)) saturate(2.1) brightness(1.2);
    }
    100% {
      transform: scale(1.06) rotate(-1.5deg);
      filter: blur(var(--blur, 52px)) saturate(1.8) brightness(1.05);
    }
  }

  /* 4. Dynamic Aurora Mode */
  .ambient-aurora {
    position: absolute;
    inset: 0;
    filter: blur(var(--blur, 52px));
    opacity: var(--opacity, 0.75);
    transform: scale(1.08) translateZ(0);
    will-change: transform, opacity;
    backface-visibility: hidden;
  }

  .aurora-blob {
    position: absolute;
    inset: -10%;
    border-radius: 50%;
    mix-blend-mode: screen;
  }

  .aurora-1 {
    background: radial-gradient(circle at 25% 30%, #00e5ff 0%, transparent 65%);
    animation: floatBlob1 14s ease-in-out infinite alternate;
    animation-play-state: paused;
  }

  .aurora-2 {
    background: radial-gradient(circle at 75% 70%, #a855f7 0%, transparent 65%);
    animation: floatBlob2 18s ease-in-out infinite alternate;
    animation-play-state: paused;
  }

  .aurora-3 {
    background: radial-gradient(circle at 60% 25%, #ff5722 0%, transparent 65%);
    animation: floatBlob3 12s ease-in-out infinite alternate;
    animation-play-state: paused;
  }

  .ambient-aurora.playing .aurora-blob {
    animation-play-state: running;
  }

  @keyframes floatBlob1 {
    0% {
      transform: translate(0, 0) scale(1);
    }
    50% {
      transform: translate(6%, 5%) scale(1.12);
    }
    100% {
      transform: translate(-4%, 8%) scale(0.95);
    }
  }

  @keyframes floatBlob2 {
    0% {
      transform: translate(0, 0) scale(1);
    }
    50% {
      transform: translate(-8%, -6%) scale(1.18);
    }
    100% {
      transform: translate(5%, -4%) scale(1.05);
    }
  }

  @keyframes floatBlob3 {
    0% {
      transform: translate(0, 0) scale(1);
    }
    50% {
      transform: translate(5%, -7%) scale(1.08);
    }
    100% {
      transform: translate(-6%, 4%) scale(1.15);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .ambient-adaptive.playing,
    .aurora-blob {
      animation: none !important;
    }
  }
</style>
