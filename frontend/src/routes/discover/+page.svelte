<script lang="ts">
  import { onMount } from 'svelte';
  type Room = { id: string; label: string; title: string; thumbnail: string; status: string; participants: number };
  let rooms: Room[] = [];
  let error = '';
  onMount(async () => {
    try {
      const r = await fetch('/api/discover');
      if (!r.ok) throw new Error('Discovery is unavailable.');
      rooms = await r.json();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Discovery is unavailable.';
    }
  });
</script>

<svelte:head><title>Discover public rooms · KoalaParty</title></svelte:head>
<main class="hub">
  <header>
    <p class="eyebrow">Public living rooms</p>
    <h1>See what’s playing</h1>
    <p>Controlled metadata only. Rooms have no editable titles, descriptions, or promotional text.</p>
  </header>
  {#if error}<p class="error" role="alert">{error}</p>{:else if !rooms.length}<div class="empty panel">
      <span>🌱</span>
      <h2>It’s quiet here</h2>
      <p>No public rooms are active. Unlisted rooms never appear here.</p>
    </div>{:else}<div class="grid">
      {#each rooms as room}<article class="panel">
          {#if room.thumbnail}<img src={room.thumbnail} alt="" />{/if}
          <div>
            <div class="row"><b>{room.label}</b><span class="pill">{room.status}</span></div>
            <p>{room.title || 'Waiting for a video'}</p>
            <small>{room.participants} participant{room.participants === 1 ? '' : 's'}</small><a
              class="button"
              href={`/room/${room.id}`}>Join</a
            >
          </div>
        </article>{/each}
    </div>{/if}
</main>

<style>
  .hub {
    max-width: 1100px;
    margin: auto;
    padding: 4rem clamp(1rem, 4vw, 3rem);
  }
  .eyebrow {
    text-transform: uppercase;
    color: var(--accent-primary);
    font-weight: 800;
    font-size: 0.75rem;
    letter-spacing: 0.13em;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 1rem;
  }
  .grid article {
    overflow: hidden;
  }
  .grid img {
    width: 100%;
    aspect-ratio: 16/9;
    object-fit: cover;
  }
  .grid article > div {
    padding: 1rem;
    display: grid;
    gap: 0.7rem;
  }
  .grid p {
    color: var(--text-secondary);
    margin: 0;
  }
  .pill {
    font-size: 0.7rem;
    padding: 0.2rem 0.45rem;
    border-radius: 1rem;
    background: var(--accent-muted);
    margin-left: auto;
  }
  .empty {
    text-align: center;
    padding: 4rem;
  }
  .empty span {
    font-size: 3rem;
  }
</style>
