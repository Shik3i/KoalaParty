<script lang="ts">
  import { onMount } from 'svelte';
  import { api, establish } from '$lib/api';

  type Room = {
    id: string;
    label: string;
    visibility: string;
    role: string;
    lastActiveAt: string;
    title: string;
    status: string;
    participants: number;
  };

  let rooms: Room[] = [];
  let loading = true;
  let error = '';
  let pending = '';
  let accountRequired = false;

  async function load() {
    loading = true;
    try {
      error = '';
      accountRequired = false;
      const me = await establish();
      if (!me.accountId) {
        accountRequired = true;
        return;
      }
      rooms = await api('/api/rooms');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not load your rooms.';
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function remove(room: Room) {
    const owner = room.role === 'owner';
    if (!confirm(owner ? `Delete ${room.label}? This cannot be undone.` : `Leave ${room.label}?`)) return;
    pending = room.id;
    try {
      await api(`/api/rooms/${room.id}${owner ? '' : '/membership'}`, { method: 'DELETE' });
      rooms = rooms.filter((candidate) => candidate.id !== room.id);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Room action failed.';
    } finally {
      pending = '';
    }
  }

  function date(value: string) {
    return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`).toLocaleString();
  }
</script>

<svelte:head><title>My rooms · KoalaParty</title></svelte:head>
<main class="page">
  <header class="title-row">
    <div>
      <p class="eyebrow">Your library</p>
      <h1>My rooms</h1>
    </div>
    <a class="button" href="/">Create a room</a>
  </header>
  {#if loading}<div class="panel empty" role="status">Loading your rooms…</div>{:else if accountRequired}<div
      class="panel empty"
    >
      <span>🔐</span>
      <h2>Account required</h2>
      <p>Create an account or log in to keep a room library across devices.</p>
      <a class="button" href="/register">Create account</a><a class="button secondary" href="/login">Log in</a>
    </div>{:else if error && !rooms.length}<div class="panel empty" role="alert">
      <span>🌧️</span>
      <h2>Could not load rooms</h2>
      <p>{error}</p>
      <button onclick={load}>Try again</button>
    </div>{:else if !rooms.length}<section class="panel empty">
      <span>🪵</span>
      <h2>No rooms yet</h2>
      <p>Rooms you own or joined with this account will appear here on every device.</p>
      <a class="button" href="/">Create a room</a>
    </section>{:else}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <section class="grid" aria-label="Your rooms">
      {#each rooms as room}
        <article class="panel room-card">
          <div class="room-icon" aria-hidden="true">{room.status === 'playing' ? '▶' : '🌿'}</div>
          <div class="room-copy">
            <div class="badges"><span>{room.role}</span><span>{room.visibility.replace('_', '-')}</span></div>
            <h2><a href={`/room/${room.id}`}>{room.label}</a></h2>
            <p>{room.title || 'Waiting for a video'}</p>
            <small>Active {date(room.lastActiveAt)} · {room.participants} online</small>
          </div>
          <div class="actions">
            <a class="button secondary" href={`/room/${room.id}`}>Open</a>
            <button class="danger" disabled={pending === room.id} onclick={() => remove(room)}>
              {pending === room.id ? 'Working…' : room.role === 'owner' ? 'Delete' : 'Leave'}
            </button>
          </div>
        </article>
      {/each}
    </section>
  {/if}
</main>

<style>
  .page {
    max-width: 1000px;
    margin: 4rem auto;
    padding: 0 1rem;
  }
  .title-row {
    display: flex;
    justify-content: space-between;
    align-items: end;
    gap: 1rem;
    margin-bottom: 1.5rem;
  }
  .title-row h1 {
    margin: 0;
  }
  .eyebrow {
    color: var(--accent-primary);
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    margin: 0 0 0.4rem;
  }
  .grid {
    display: grid;
    gap: 1rem;
  }
  .room-card {
    display: grid;
    grid-template-columns: auto 1fr auto;
    align-items: center;
    gap: 1rem;
    padding: 1.2rem;
  }
  .room-icon {
    width: 3.3rem;
    height: 3.3rem;
    display: grid;
    place-content: center;
    border-radius: 50%;
    background: var(--accent-muted);
  }
  .room-copy {
    min-width: 0;
  }
  .room-copy h2 {
    margin: 0.35rem 0;
    font-size: 1.15rem;
  }
  .room-copy h2 a {
    color: var(--text-primary);
    text-decoration: none;
  }
  .room-copy p,
  .room-copy small {
    color: var(--text-muted);
    margin: 0;
  }
  .badges {
    display: flex;
    gap: 0.4rem;
  }
  .badges span {
    background: var(--surface-hover);
    border-radius: 1rem;
    padding: 0.2rem 0.45rem;
    font-size: 0.68rem;
    text-transform: capitalize;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .empty {
    text-align: center;
    padding: 4rem 2rem;
  }
  .empty span {
    font-size: 3rem;
  }
  .empty .button {
    margin: 0.25rem;
  }
  @media (max-width: 700px) {
    .room-card {
      grid-template-columns: auto 1fr;
    }
    .actions {
      grid-column: 1 / -1;
    }
    .actions > * {
      flex: 1;
    }
  }
</style>
