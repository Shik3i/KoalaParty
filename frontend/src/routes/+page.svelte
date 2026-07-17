<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import KoalaSyncPromo from '$lib/KoalaSyncPromo.svelte';
  let roomCode = '';
  let creating = false;
  let error = '';
  async function createRoom() {
    creating = true;
    error = '';
    try {
      const room = await api<{ id: string }>('/api/rooms', { method: 'POST' });
      await goto(`/room/${room.id}`);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not create room';
    } finally {
      creating = false;
    }
  }
  function join() {
    const value = roomCode.trim();
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
  <section class="hero">
    <div class="eyebrow">Your permanent digital living room</div>
    <h1>Watch YouTube together.<br /><span>Keep it private.</span></h1>
    <p class="lede">Shared playback and a collaborative queue without accounts, advertising, analytics, or tracking.</p>
    <div class="actions">
      <button onclick={createRoom} disabled={creating}>{creating ? 'Creating…' : 'Create a room'}</button><a
        class="button secondary"
        href="/discover">Browse public rooms</a
      >
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <p class="warning">Anonymous rooms belong to this browser. Link an account before clearing browser storage.</p>
  </section>
  <aside class="join panel">
    <div class="room-mark" aria-hidden="true">🌿</div>
    <h2>Join the living room</h2>
    <p class="muted">Paste an invite link or room code.</p>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        join();
      }}
    >
      <label>Room link or code<input bind:value={roomCode} placeholder="7FD3KQ9X…" autocomplete="off" /></label><button
        type="submit">Join room</button
      >
    </form>
    <div class="signals"><span>● Live sync</span><span>∞ Permanent</span><span>◌ Account optional</span></div>
  </aside>
</main>
<section class="features">
  <article><b>Shared player</b><span>Play, pause, seek, and stay together.</span></article>
  <article><b>Open queue</b><span>Everyone can add and arrange videos by default.</span></article>
  <article><b>Real privacy</b><span>No analytics scripts, ads, fingerprinting, or third-party fonts.</span></article>
  <article><b>Public source</b><span>Self-host with Go, SQLite, Docker, and Caddy.</span></article>
</section>
<KoalaSyncPromo />

<style>
  .landing {
    max-width: 1180px;
    margin: auto;
    padding: clamp(3rem, 9vw, 8rem) clamp(1rem, 4vw, 3rem);
    display: grid;
    grid-template-columns: 1.15fr 0.75fr;
    gap: clamp(2rem, 7vw, 7rem);
    align-items: center;
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
