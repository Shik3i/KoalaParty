<script lang="ts">
  import { onMount } from 'svelte';
  import { fly, fade, scale } from 'svelte/transition';
  import { Play, ShareNetwork } from 'phosphor-svelte';
  import { t } from '$lib/i18n';

  // A self-running, stylised room that shows what a party feels like: countdown,
  // people joining, chat, reactions and a combo. Pure HTML — no third parties.
  type Step =
    | { kind: 'count'; value: number }
    | { kind: 'play' }
    | { kind: 'toast'; badge: string; name: string; text: 'join' | 'add' }
    | { kind: 'chat'; badge: string; name: string; text: 'chat1' | 'chat2' }
    | { kind: 'react'; emoji: string }
    | { kind: 'combo' }
    | { kind: 'reset' };
  const script: Array<[number, Step]> = [
    [0, { kind: 'count', value: 3 }],
    [700, { kind: 'count', value: 2 }],
    [1400, { kind: 'count', value: 1 }],
    [2100, { kind: 'play' }],
    [2600, { kind: 'toast', badge: '🦘', name: 'Lisa', text: 'join' }],
    [3900, { kind: 'chat', badge: '🦘', name: 'Lisa', text: 'chat1' }],
    [5000, { kind: 'react', emoji: '😂' }],
    [5400, { kind: 'react', emoji: '🔥' }],
    [5800, { kind: 'react', emoji: '🔥' }],
    [6150, { kind: 'react', emoji: '🔥' }],
    [6200, { kind: 'combo' }],
    [7600, { kind: 'toast', badge: '🦊', name: 'Max', text: 'add' }],
    [8800, { kind: 'chat', badge: '🦊', name: 'Max', text: 'chat2' }],
    [10200, { kind: 'react', emoji: '❤️' }],
    [12500, { kind: 'reset' }],
  ];

  let count = $state<number | null>(3);
  let playing = $state(false);
  let progress = $state(0);
  let toasts = $state<Array<{ id: number; badge: string; name: string; text: 'join' | 'add' }>>([]);
  let chat = $state<Array<{ id: number; badge: string; name: string; text: 'chat1' | 'chat2' }>>([]);
  let flying = $state<Array<{ id: number; emoji: string; x: number }>>([]);
  let combo = $state(false);
  let reduced = $state(false);
  let seq = 0;

  function apply(step: Step) {
    switch (step.kind) {
      case 'count':
        count = step.value;
        break;
      case 'play':
        count = null;
        playing = true;
        break;
      case 'toast': {
        const toast = { id: ++seq, badge: step.badge, name: step.name, text: step.text };
        toasts = [...toasts, toast].slice(-2);
        setTimeout(() => (toasts = toasts.filter((item) => item.id !== toast.id)), 2600);
        break;
      }
      case 'chat':
        chat = [...chat, { id: ++seq, badge: step.badge, name: step.name, text: step.text }].slice(-2);
        break;
      case 'react': {
        const item = { id: ++seq, emoji: step.emoji, x: 8 + Math.round(Math.random() * 30) };
        flying = [...flying, item];
        setTimeout(() => (flying = flying.filter((entry) => entry.id !== item.id)), 2200);
        break;
      }
      case 'combo':
        combo = true;
        setTimeout(() => (combo = false), 1400);
        break;
      case 'reset':
        count = 3;
        playing = false;
        progress = 0;
        toasts = [];
        chat = [];
        break;
    }
  }

  onMount(() => {
    reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduced) {
      count = null;
      playing = true;
      progress = 38;
      toasts = [{ id: 1, badge: '🦘', name: 'Lisa', text: 'join' }];
      chat = [
        { id: 2, badge: '🦘', name: 'Lisa', text: 'chat1' },
        { id: 3, badge: '🦊', name: 'Max', text: 'chat2' },
      ];
      return;
    }
    let timers: ReturnType<typeof setTimeout>[] = [];
    const run = () => {
      timers = script.map(([at, step]) => setTimeout(() => apply(step), at));
      timers.push(setTimeout(run, 13000));
    };
    run();
    const ticker = setInterval(() => {
      if (playing) progress = Math.min(100, progress + 0.8);
    }, 100);
    return () => {
      timers.forEach(clearTimeout);
      clearInterval(ticker);
    };
  });
</script>

<div class="demo" aria-hidden="true">
  <div class="chrome">
    <span class="dot"></span><span class="dot"></span><span class="dot"></span>
    <span class="address">party.koalastuff.net/r/{$t('demo.slug')}</span>
  </div>
  <div class="room">
    <div class="stage">
      <div class="screen" class:playing>
        <img src="/icons/koalaparty-512.png" alt="" />
        {#if count !== null}{#key count}<div class="count" in:scale={{ start: 1.6, duration: 260 }}>
              {count}
            </div>{/key}{/if}
        {#if playing && !reduced}<span class="play-hint" in:fade><Play size={16} weight="fill" /></span>{/if}
        <div class="flying">
          {#each flying as item (item.id)}<span style={`right:${item.x}%`}>{item.emoji}</span>{/each}
        </div>
        {#if combo}<div class="combo" transition:scale={{ start: 0.4, duration: 220 }}>🔥 <b>×3</b></div>{/if}
        <div class="toasts">
          {#each toasts as toast (toast.id)}<p in:fly={{ x: -16, duration: 220 }} out:fade={{ duration: 200 }}>
              <span>{toast.badge}</span><b>{toast.name}</b>
              {toast.text === 'join' ? $t('demo.joined') : $t('demo.added')}
            </p>{/each}
        </div>
      </div>
      <div class="bar"><div class="fill" style={`width:${progress}%`}></div></div>
    </div>
    <aside class="side">
      <div class="side-head"><b>{$t('tab.chat')}</b><span><ShareNetwork size={12} weight="bold" /></span></div>
      <div class="messages">
        {#each chat as message (message.id)}<p in:fly={{ y: 8, duration: 220 }}>
            <span>{message.badge}</span><b>{message.name}</b>
            <em>{message.text === 'chat1' ? $t('demo.chat1') : $t('demo.chat2')}</em>
          </p>{/each}
      </div>
      <div class="input">{$t('chat.placeholder')}</div>
    </aside>
  </div>
</div>

<style>
  .demo {
    border-radius: 18px;
    overflow: hidden;
    border: 1px solid var(--border-subtle);
    background: var(--surface-panel);
    box-shadow: 0 30px 80px rgba(3, 12, 8, 0.35);
    user-select: none;
  }
  .chrome {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.55rem 0.8rem;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-elevated);
  }
  .dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    background: var(--border-strong);
  }
  .address {
    margin-left: 0.6rem;
    font-size: 0.72rem;
    color: var(--text-muted);
    padding: 0.2rem 0.6rem;
    border-radius: 999px;
    background: var(--surface-hover);
  }
  .room {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 11rem;
    gap: 0.7rem;
    padding: 0.8rem;
  }
  .screen {
    position: relative;
    aspect-ratio: 16 / 9;
    border-radius: 12px;
    overflow: hidden;
    display: grid;
    place-items: center;
    background:
      radial-gradient(circle at 30% 20%, color-mix(in srgb, var(--accent-primary) 45%, transparent), transparent 55%),
      radial-gradient(circle at 80% 90%, rgba(255, 180, 90, 0.35), transparent 50%), #0e1a14;
  }
  .screen img {
    width: 34%;
    opacity: 0.35;
    filter: saturate(0.6);
    transition:
      opacity 0.4s,
      transform 6s linear;
  }
  .screen.playing img {
    opacity: 0.9;
    transform: scale(1.12) rotate(-2deg);
  }
  .count {
    position: absolute;
    font-size: 4.5rem;
    font-weight: 900;
    color: white;
    text-shadow: 0 8px 30px color-mix(in srgb, var(--accent-primary) 80%, transparent);
  }
  .play-hint {
    position: absolute;
    right: 0.6rem;
    top: 0.6rem;
    color: white;
    opacity: 0.8;
  }
  .flying {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }
  .flying span {
    position: absolute;
    bottom: 12%;
    font-size: 1.6rem;
    animation: rise 2.2s ease-out forwards;
  }
  @keyframes rise {
    from {
      transform: translateY(0) scale(0.6);
      opacity: 0;
    }
    15% {
      opacity: 1;
      transform: translateY(-10%) scale(1.1);
    }
    to {
      transform: translateY(-230%);
      opacity: 0;
    }
  }
  .combo {
    position: absolute;
    font-size: 3rem;
    color: white;
    text-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
  .combo b {
    font-size: 1.8rem;
  }
  .toasts {
    position: absolute;
    left: 0.6rem;
    top: 0.6rem;
    display: grid;
    gap: 0.3rem;
  }
  .toasts p,
  .messages p {
    margin: 0;
    font-size: 0.72rem;
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }
  .toasts p {
    padding: 0.3rem 0.6rem 0.3rem 0.35rem;
    border-radius: 999px;
    background: rgba(8, 14, 11, 0.72);
    color: white;
  }
  .bar {
    margin-top: 0.55rem;
    height: 5px;
    border-radius: 999px;
    background: var(--surface-hover);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background: linear-gradient(90deg, var(--accent-primary), var(--accent-hover));
    transition: width 0.1s linear;
  }
  .side {
    display: grid;
    grid-template-rows: auto 1fr auto;
    gap: 0.5rem;
    border-left: 1px solid var(--border-subtle);
    padding-left: 0.7rem;
    min-width: 0;
  }
  .side-head {
    display: flex;
    justify-content: space-between;
    font-size: 0.78rem;
    color: var(--text-secondary);
  }
  .messages {
    display: grid;
    align-content: end;
    gap: 0.45rem;
    min-height: 6rem;
  }
  .messages p {
    flex-wrap: wrap;
  }
  .messages em {
    flex-basis: 100%;
    font-style: normal;
    color: var(--text-secondary);
    padding-left: 1.2rem;
  }
  .input {
    font-size: 0.68rem;
    color: var(--text-muted);
    padding: 0.35rem 0.55rem;
    border-radius: 999px;
    border: 1px solid var(--border-subtle);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  @media (max-width: 560px) {
    .room {
      grid-template-columns: 1fr;
    }
    .side {
      border-left: 0;
      padding-left: 0;
    }
    .messages {
      min-height: 0;
    }
  }
</style>
