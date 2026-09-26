<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { forgetRoom, recentRooms as loadRecentRooms, reconcileRecentRooms, type RecentRoom } from '$lib/recentRooms';
  import KoalaSyncPromo from '$lib/KoalaSyncPromo.svelte';
  import LiveDemo from '$lib/LiveDemo.svelte';
  import { errorText, t } from '$lib/i18n';
  import { parseYouTubeInput } from '$lib/room';
  import {
    Compass,
    Broadcast,
    Infinity as InfinityIcon,
    UserCircle,
    FilmSlate,
    ListPlus,
    ShieldCheck,
    GithubLogo,
    ArrowRight,
    Check,
    Minus,
    QrCode,
    ChatsCircle,
    ClockCounterClockwise,
    X,
  } from 'phosphor-svelte';
  let roomCode = '';
  let creating = false;
  let error = '';
  const fmtRecentTime = (seconds: number) =>
    `${Math.floor(Math.max(0, seconds) / 60)}:${String(Math.floor(Math.max(0, seconds) % 60)).padStart(2, '0')}`;
  let recentRooms: RecentRoom[] = [];
  let roomPreviews = new Map<string, { participants: number; status: string; position: number; thumbnail: string }>();
  onMount(async () => {
    // App shortcut ("Create a room") from the installed PWA.
    if (new URLSearchParams(location.search).get('create') === '1') {
      history.replaceState(history.state, '', '/');
      void createRoom();
      return;
    }
    recentRooms = loadRecentRooms();
    if (!recentRooms.length) return;
    try {
      const previews = await api<
        Array<RecentRoom & { participants: number; status: string; position: number; thumbnail: string }>
      >('/api/rooms/previews', { method: 'POST', body: JSON.stringify({ ids: recentRooms.map((room) => room.id) }) });
      recentRooms = reconcileRecentRooms(previews);
      roomPreviews = new Map(previews.map((preview) => [preview.id, preview]));
    } catch {
      // Local shortcuts remain useful while the server is temporarily unavailable.
    }
  });
  // `add` is a YouTube link to start the new room with, so pasting a video on the
  // start page is a one-step party.
  async function createRoom(add = '') {
    creating = true;
    error = '';
    try {
      const room = await api<{ id: string }>('/api/rooms', { method: 'POST' });
      let copied = false;
      try {
        await navigator.clipboard.writeText(`${location.origin}/room/${room.id}`);
        copied = true;
      } catch {
        /* clipboard blocked — the room page will prompt to copy manually */
      }
      try {
        sessionStorage.setItem('koalaparty.created', JSON.stringify({ id: room.id.toUpperCase(), copied }));
      } catch {
        /* sessionStorage unavailable */
      }
      await goto(`/room/${room.id}${add ? `?add=${encodeURIComponent(add)}` : ''}`);
    } catch (e) {
      error = errorText(e);
    } finally {
      creating = false;
    }
  }
  function joinOrCreate() {
    error = '';
    const value = roomCode.trim();
    if (!value) {
      createRoom();
      return;
    }
    const video = parseYouTubeInput(value);
    if (video.videos.length || video.playlistId) {
      createRoom(value);
      return;
    }
    const short = value.match(/(?:^|\/)r\/([a-z0-9-]{3,32})\/?$/i);
    if (short) {
      goto(`/r/${short[1].toLowerCase()}`);
      return;
    }
    const match = value.match(/(?:room\/)?([A-Z2-7]{16})$/i);
    if (!match) {
      error = $t('home.invalidInput');
      return;
    }
    goto(`/room/${match[1].toUpperCase()}`);
  }
</script>

<svelte:head><title>KoalaParty — {$t('home.title')}</title></svelte:head>
<main class="landing">
  <div class="hero-glow" aria-hidden="true"></div>
  <section class="hero">
    <div class="eyebrow">{$t('home.eyebrow')}</div>
    <h1>{$t('home.headline1')}<br /><span>{$t('home.headline2')}</span></h1>
    <p class="lede">{$t('home.lede')}</p>
    <div class="actions">
      <button type="button" onclick={() => createRoom()} disabled={creating}
        >{creating ? $t('home.creating') : $t('home.create')}<ArrowRight size={18} weight="bold" /></button
      >
      <a class="button secondary" href="/discover"><Compass size={18} weight="bold" />{$t('home.browse')}</a>
    </div>
    <p class="warning">{$t('home.anonWarning')}</p>
  </section>
  <aside class="join panel">
    <img class="room-mark" src="/icons/koalaparty-192.png" alt="" />
    <h2>{$t('home.joinTitle')}</h2>
    <p class="muted">{$t('home.joinBody')}</p>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        joinOrCreate();
      }}
    >
      <label
        >{$t('home.joinLabel')}<input
          bind:value={roomCode}
          placeholder={$t('home.joinPlaceholder')}
          autocomplete="off"
        /></label
      ><button type="submit" disabled={creating}
        >{creating
          ? $t('home.creating')
          : !roomCode.trim()
            ? $t('home.create')
            : parseYouTubeInput(roomCode).videos.length || parseYouTubeInput(roomCode).playlistId
              ? $t('home.startWithVideo')
              : $t('home.join')}<ArrowRight size={17} weight="bold" /></button
      >
    </form>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="signals">
      <span><Broadcast size={15} weight="bold" />{$t('home.signalSync')}</span><span
        ><InfinityIcon size={15} weight="bold" />{$t('home.signalPermanent')}</span
      ><span><UserCircle size={15} weight="bold" />{$t('home.signalAccount')}</span>
    </div>
  </aside>
</main>
{#if recentRooms.length > 0}
  <section class="recent" aria-labelledby="recent-rooms-title">
    <div class="section-heading">
      <div>
        <span class="eyebrow"><ClockCounterClockwise size={15} weight="bold" />{$t('home.recentEyebrow')}</span>
        <h2 id="recent-rooms-title">{$t('home.recentTitle')}</h2>
      </div>
    </div>
    <div class="recent-grid">
      {#each recentRooms as room (room.id)}
        <article class="recent-room panel">
          <a href={`/room/${room.id}`} aria-label={$t('home.openRoom', { room: room.label })}>
            <span class="room-code">{room.id}</span>
            <strong>{room.label}</strong>
            <span class="recent-title">{room.title || $t('home.readyToWatch')}</span>
            {#if roomPreviews.has(room.id)}{@const preview = roomPreviews.get(room.id)!}<span class="recent-meta"
                >{$t(preview.status === 'playing' ? 'home.previewPlaying' : 'home.previewPaused', {
                  count: preview.participants,
                  time: fmtRecentTime(preview.position),
                })}</span
              >{/if}
          </a>
          <button
            class="icon-button secondary"
            type="button"
            aria-label={$t('home.forgetRoom', { room: room.label })}
            title={$t('home.forget')}
            onclick={() => (recentRooms = forgetRoom(room.id))}><X size={16} weight="bold" /></button
          >
        </article>
      {/each}
    </div>
  </section>
{/if}
<section class="showcase" aria-labelledby="showcase-title">
  <div class="showcase-copy">
    <span class="eyebrow">{$t('home.showcaseEyebrow')}</span>
    <h2 id="showcase-title">{$t('home.showcaseTitle')}</h2>
    <ol class="steps">
      <li>
        <span class="step-icon"><ListPlus size={22} weight="duotone" /></span>
        <div><b>{$t('home.step1')}</b><span>{$t('home.step1Body')}</span></div>
      </li>
      <li>
        <span class="step-icon"><QrCode size={22} weight="duotone" /></span>
        <div><b>{$t('home.step2')}</b><span>{$t('home.step2Body')}</span></div>
      </li>
      <li>
        <span class="step-icon"><ChatsCircle size={22} weight="duotone" /></span>
        <div><b>{$t('home.step3')}</b><span>{$t('home.step3Body')}</span></div>
      </li>
    </ol>
  </div>
  <LiveDemo />
</section>
<section class="compare" aria-labelledby="compare-title">
  <h2 id="compare-title">{$t('home.compareTitle')}</h2>
  <p class="muted">{$t('home.compareBody')}</p>
  <div class="compare-table" role="table" aria-label={$t('home.compareTitle')}>
    <div class="row head" role="row">
      <span role="columnheader"></span><span role="columnheader">KoalaParty</span><span role="columnheader"
        >{$t('home.compareOthers')}</span
      >
    </div>
    {#each ['account', 'ads', 'tracking', 'chat', 'countdown', 'open'] as const as row (row)}<div
        class="row"
        role="row"
      >
        <span role="rowheader">{$t(`home.compare.${row}`)}</span>
        <span role="cell" class="yes"
          ><Check size={18} weight="bold" /><span class="sr-only">{$t('home.yes')}</span></span
        >
        <span role="cell" class="other"
          ><Minus size={18} weight="bold" /><small>{$t(`home.compare.${row}.others`)}</small></span
        >
      </div>{/each}
  </div>
</section>
<section class="features">
  <article>
    <FilmSlate size={24} weight="duotone" /><b>{$t('home.feature1')}</b><span>{$t('home.feature1Body')}</span>
  </article>
  <article>
    <ListPlus size={24} weight="duotone" /><b>{$t('home.feature2')}</b><span>{$t('home.feature2Body')}</span>
  </article>
  <article>
    <ShieldCheck size={24} weight="duotone" /><b>{$t('home.feature3')}</b><span>{$t('home.feature3Body')}</span>
  </article>
  <article>
    <GithubLogo size={24} weight="duotone" /><b>{$t('home.feature4')}</b><span>{$t('home.feature4Body')}</span>
  </article>
</section>
<KoalaSyncPromo />

<style>
  .landing {
    position: relative;
    overflow: clip;
    max-width: 1180px;
    margin: auto;
    padding: clamp(3rem, 9vw, 8rem) clamp(1rem, 4vw, 3rem);
    display: grid;
    grid-template-columns: 1.15fr 0.75fr;
    gap: clamp(2rem, 7vw, 7rem);
    align-items: center;
    min-height: min(760px, calc(100svh - 76px));
  }
  .hero,
  .join {
    animation: revealUp 0.65s cubic-bezier(0.2, 0.8, 0.2, 1) both;
  }
  .join {
    animation-delay: 0.08s;
  }
  @keyframes revealUp {
    from {
      opacity: 0;
      transform: translateY(18px);
    }
  }
  .hero-glow {
    position: absolute;
    inset: -20% -10% auto -10%;
    height: 60%;
    z-index: -1;
    pointer-events: none;
    background:
      radial-gradient(40% 55% at 20% 30%, color-mix(in srgb, var(--accent-primary) 24%, transparent), transparent 70%),
      radial-gradient(35% 50% at 80% 20%, color-mix(in srgb, var(--accent-hover) 20%, transparent), transparent 70%);
    filter: blur(28px);
    opacity: 0.9;
    animation: heroDrift 16s ease-in-out infinite alternate;
  }
  @keyframes heroDrift {
    from {
      transform: translate3d(-2%, -1%, 0) scale(1);
    }
    to {
      transform: translate3d(3%, 2%, 0) scale(1.08);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .hero-glow {
      animation: none;
    }
  }
  .eyebrow {
    text-transform: uppercase;
    letter-spacing: 0.12em;
    font-size: 0.75rem;
    font-weight: 800;
    color: var(--accent-primary);
    margin-bottom: 1rem;
  }
  .hero h1 {
    font-size: clamp(3rem, 7vw, 6.3rem);
    margin-bottom: 1.3rem;
  }
  .hero h1 span {
    color: var(--accent-primary);
  }
  .lede {
    max-width: 650px;
    font-size: clamp(1.05rem, 2vw, 1.35rem);
    color: var(--text-secondary);
    line-height: 1.6;
  }
  .actions {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
    margin: 2rem 0;
  }
  .warning {
    font-size: 0.85rem;
    color: var(--warning);
    max-width: 570px;
  }
  .join {
    padding: clamp(1.4rem, 4vw, 2.4rem);
    position: relative;
    overflow: hidden;
    min-width: 0;
    border-radius: var(--radius-lg);
    box-shadow: 0 28px 80px color-mix(in srgb, var(--accent-primary) 13%, transparent);
    transition:
      transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1),
      box-shadow 0.3s ease;
  }
  .join:hover {
    transform: translateY(-4px);
    box-shadow: 0 34px 90px color-mix(in srgb, var(--accent-primary) 19%, transparent);
  }
  .room-mark {
    display: block;
    width: 4rem;
    height: 4rem;
    object-fit: contain;
    margin-bottom: 1rem;
  }
  .join form {
    display: grid;
    gap: 1rem;
  }
  .signals {
    border-top: 1px solid var(--border-subtle);
    margin-top: 1.8rem;
    padding-top: 1rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem 1rem;
    color: var(--text-muted);
    font-size: 0.78rem;
  }
  .signals span {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  .signals :global(svg) {
    color: var(--accent-primary);
  }
  .showcase {
    max-width: 1180px;
    margin: 0 auto 4.5rem;
    padding: 0 clamp(1rem, 4vw, 3rem);
    display: grid;
    grid-template-columns: 0.8fr 1.2fr;
    gap: clamp(1.5rem, 5vw, 4rem);
    align-items: center;
  }
  .showcase h2,
  .compare h2 {
    font-size: clamp(1.6rem, 3.4vw, 2.4rem);
    margin: 0.3rem 0 1.2rem;
  }
  .steps {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 1.1rem;
  }
  .steps li {
    display: flex;
    gap: 0.9rem;
    align-items: flex-start;
  }
  .steps li div {
    display: grid;
    gap: 0.2rem;
  }
  .steps li span:not(.step-icon) {
    color: var(--text-muted);
    font-size: 0.92rem;
    line-height: 1.5;
  }
  .step-icon {
    flex: 0 0 auto;
    width: 2.6rem;
    height: 2.6rem;
    display: grid;
    place-content: center;
    border-radius: 12px;
    background: var(--accent-muted);
    color: var(--accent-primary);
  }
  .compare {
    max-width: 860px;
    margin: 0 auto 4.5rem;
    padding: 0 clamp(1rem, 4vw, 3rem);
    text-align: center;
  }
  .compare > p {
    margin: 0 auto 1.5rem;
    max-width: 38rem;
  }
  .compare-table {
    text-align: left;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    overflow: hidden;
    background: var(--surface-panel);
  }
  .compare-table .row {
    display: grid;
    grid-template-columns: minmax(0, 1.4fr) 0.6fr minmax(0, 1fr);
    align-items: center;
    gap: 0.8rem;
    padding: 0.75rem 1.1rem;
    border-top: 1px solid var(--border-subtle);
  }
  .compare-table .row.head {
    border-top: 0;
    font-weight: 800;
    background: var(--surface-elevated);
  }
  .compare-table .yes {
    color: var(--success);
  }
  .compare-table .other {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--text-muted);
  }
  .compare-table small {
    font-size: 0.8rem;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
  .features {
    max-width: 1180px;
    margin: 0 auto 4rem;
    padding: 0 clamp(1rem, 4vw, 3rem);
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1px;
    background: var(--border-subtle);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    overflow: hidden;
  }
  .recent {
    max-width: 1180px;
    margin: 0 auto 3rem;
    padding: 0 clamp(1rem, 4vw, 3rem);
  }
  .section-heading {
    display: flex;
    align-items: end;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  .section-heading .eyebrow {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-bottom: 0.35rem;
  }
  .section-heading h2 {
    margin: 0;
  }
  .recent-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 210px), 1fr));
    gap: 0.75rem;
  }
  .recent-room {
    position: relative;
    padding: 0;
    overflow: hidden;
    transition:
      transform 0.22s ease,
      border-color 0.22s ease,
      box-shadow 0.22s ease;
  }
  .recent-room:hover {
    transform: translateY(-4px);
    border-color: color-mix(in srgb, var(--accent-primary) 45%, var(--border-subtle));
    box-shadow: 0 20px 45px color-mix(in srgb, var(--accent-primary) 12%, transparent);
  }
  .recent-room > a {
    display: grid;
    gap: 0.35rem;
    min-height: 128px;
    padding: 1.2rem 3rem 1.2rem 1.2rem;
    color: inherit;
    text-decoration: none;
  }
  .recent-room > a:hover strong {
    color: var(--accent-primary);
  }
  .room-code {
    color: var(--text-muted);
    font-family: monospace;
    font-size: 0.72rem;
    letter-spacing: 0.06em;
  }
  .recent-title {
    color: var(--text-muted);
    font-size: 0.85rem;
    line-height: 1.4;
  }
  .recent-meta {
    color: var(--accent-primary);
    font-size: 0.75rem;
    font-weight: 700;
  }
  .recent-room .icon-button {
    position: absolute;
    top: 0.7rem;
    right: 0.7rem;
  }
  .features article {
    background: var(--surface-panel);
    padding: 1.5rem;
    display: grid;
    gap: 0.5rem;
    transition:
      background 0.2s ease,
      transform 0.2s ease;
  }
  .features article:hover {
    background: var(--surface-hover);
    transform: translateY(-3px);
  }
  .features article :global(svg) {
    color: var(--accent-primary);
    margin-bottom: 0.2rem;
  }
  .features span {
    color: var(--text-muted);
    font-size: 0.9rem;
    line-height: 1.5;
  }
  @media (max-width: 800px) {
    .landing {
      grid-template-columns: minmax(0, 1fr);
      padding-top: 3rem;
      min-height: auto;
    }
    .landing > * {
      min-width: 0;
    }
    .features {
      grid-template-columns: 1fr 1fr;
    }
    .showcase {
      grid-template-columns: minmax(0, 1fr);
    }
    .compare-table .row {
      grid-template-columns: minmax(0, 1fr) auto;
    }
    .compare-table .row > :nth-child(3) {
      grid-column: 1 / -1;
    }
    .compare-table .row.head > :nth-child(3) {
      display: none;
    }
    .section-heading {
      align-items: start;
    }
  }
  @media (max-width: 480px) {
    .landing {
      padding-top: 2rem;
      gap: 1.5rem;
    }
    .hero h1 {
      font-size: clamp(2.55rem, 15vw, 4rem);
    }
    .join {
      border-radius: var(--radius-md);
    }
    .features {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
