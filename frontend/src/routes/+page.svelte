<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import KoalaSyncPromo from '$lib/KoalaSyncPromo.svelte';
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
  } from 'phosphor-svelte';
  let roomCode = '';
  let creating = false;
  let error = '';
  async function createRoom() {
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
      await goto(`/room/${room.id}`);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not create room';
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
    const match = value.match(/(?:room\/)?([A-Z2-7]{16})$/i);
    if (!match) {
      error = 'Enter a valid 16-character room code or link.';
      return;
    }
    goto(`/room/${match[1].toUpperCase()}`);
  }
</script>

<svelte:head><title>KoalaParty — Watch YouTube together privately</title></svelte:head>
<main class="landing">
  <div class="hero-glow" aria-hidden="true"></div>
  <section class="hero">
    <div class="eyebrow">Your permanent digital living room</div>
    <h1>Watch YouTube together.<br /><span>Keep it private.</span></h1>
    <p class="lede">Shared playback and a collaborative queue without accounts, advertising, analytics, or tracking.</p>
    <div class="actions">
      <a class="button secondary" href="/discover"><Compass size={18} weight="bold" />Browse public rooms</a>
    </div>
    <p class="warning">Anonymous rooms belong to this browser. Link an account before clearing browser storage.</p>
  </section>
  <aside class="join panel">
    <div class="room-mark" aria-hidden="true">🐨</div>
    <h2>Start a living room</h2>
    <p class="muted">Paste an invite link to join friends — or leave it empty to open a fresh room.</p>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        joinOrCreate();
      }}
    >
      <label>Room link or code<input bind:value={roomCode} placeholder="7FD3KQ9X…" autocomplete="off" /></label><button
        type="submit"
        disabled={creating}
        >{creating ? 'Creating…' : roomCode.trim() ? 'Join room' : 'Create a room'}<ArrowRight
          size={17}
          weight="bold"
        /></button
      >
    </form>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="signals">
      <span><Broadcast size={15} weight="bold" />Live sync</span><span
        ><InfinityIcon size={15} weight="bold" />Permanent</span
      ><span><UserCircle size={15} weight="bold" />Account optional</span>
    </div>
  </aside>
</main>
<section class="features">
  <article>
    <FilmSlate size={24} weight="duotone" /><b>Shared player</b><span>Play, pause, seek, and stay together.</span>
  </article>
  <article>
    <ListPlus size={24} weight="duotone" /><b>Open queue</b><span>Everyone can add and arrange videos by default.</span>
  </article>
  <article>
    <ShieldCheck size={24} weight="duotone" /><b>Real privacy</b><span
      >No analytics scripts, ads, fingerprinting, or third-party fonts.</span
    >
  </article>
  <article>
    <GithubLogo size={24} weight="duotone" /><b>Public source</b><span
      >Self-host with Go, SQLite, Docker, and Caddy.</span
    >
  </article>
</section>
<KoalaSyncPromo />

<style>
  .landing {
    position: relative;
    max-width: 1180px;
    margin: auto;
    padding: clamp(3rem, 9vw, 8rem) clamp(1rem, 4vw, 3rem);
    display: grid;
    grid-template-columns: 1.15fr 0.75fr;
    gap: clamp(2rem, 7vw, 7rem);
    align-items: center;
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
  }
  .room-mark {
    font-size: 4rem;
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
  .features article {
    background: var(--surface-panel);
    padding: 1.5rem;
    display: grid;
    gap: 0.5rem;
    transition: background 0.15s ease;
  }
  .features article:hover {
    background: var(--surface-hover);
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
    }
    .landing > * {
      min-width: 0;
    }
    .features {
      grid-template-columns: 1fr 1fr;
    }
  }
  @media (max-width: 480px) {
    .features {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
