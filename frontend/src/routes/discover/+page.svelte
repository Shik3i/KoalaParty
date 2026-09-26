<script lang="ts">
  import { errorText, t } from '$lib/i18n';
  import { onMount } from 'svelte';
  type Room = { id: string; label: string; title: string; thumbnail: string; status: string; participants: number };
  let rooms: Room[] = [];
  let error = '';
  let disabled = false;
  let loading = true;
  async function load() {
    loading = true;
    error = '';
    disabled = false;
    try {
      const r = await fetch('/api/discover');
      if (r.status === 404) {
        disabled = true;
        return;
      }
      if (!r.ok) throw new Error($t('discover.unavailable'));
      rooms = await r.json();
    } catch (e) {
      error = errorText(e);
    } finally {
      loading = false;
    }
  }
  onMount(load);
</script>

<svelte:head><title>{$t('discover.title')} · KoalaParty</title></svelte:head>
<main class="hub">
  <header>
    <p class="eyebrow">{$t('discover.eyebrow')}</p>
    <h1>{$t('discover.heading')}</h1>
    <p>{$t('discover.body')}</p>
  </header>
  {#if loading}<div class="empty panel" role="status">{$t('discover.loading')}</div>{:else if error}<div
      class="empty panel"
      role="alert"
    >
      <h2>{$t('discover.loadFailed')}</h2>
      <p class="error">{error}</p>
      <button onclick={load}>{$t('player.tryAgain')}</button>
    </div>{:else if disabled}<div class="empty panel">
      <span>🔗</span>
      <h2>{$t('discover.beta')}</h2>
      <p>{$t('discover.betaBody')}</p>
    </div>{:else if !rooms.length}<div class="empty panel">
      <span>🌱</span>
      <h2>{$t('discover.quiet')}</h2>
      <p>{$t('discover.quietBody')}</p>
    </div>{:else}<div class="grid">
      {#each rooms as room}<article class="panel">
          <div class="thumbnail-placeholder" aria-hidden="true"><span>▶</span><small>YouTube</small></div>
          <div>
            <div class="row">
              <b>{room.label}</b><span class="pill"
                >{room.status === 'playing' ? $t('discover.playing') : $t('discover.paused')}</span
              >
            </div>
            <p>{room.title || $t('rooms.waiting')}</p>
            <small>{$t('discover.participants', { count: room.participants })}</small><a
              class="button"
              href={`/room/${room.id}`}>{$t('discover.join')}</a
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
  .thumbnail-placeholder {
    width: 100%;
    aspect-ratio: 16/9;
    display: grid;
    place-content: center;
    gap: 0.35rem;
    text-align: center;
    color: var(--text-muted);
    background: var(--player-background);
  }
  .thumbnail-placeholder span {
    font-size: 1.8rem;
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
