<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { t } from '$lib/i18n';

  // Short room links (/r/movie-night) resolve to the room and replace the URL.
  let missing = $state(false);

  onMount(async () => {
    const slug = page.params.slug ?? '';
    try {
      const response = await fetch(`/api/room-links/${encodeURIComponent(slug)}`);
      if (!response.ok) throw new Error('not found');
      const { id } = (await response.json()) as { id: string };
      await goto(`/room/${id}${page.url.search}`, { replaceState: true });
    } catch {
      missing = true;
    }
  });
</script>

<svelte:head><title>KoalaParty</title></svelte:head>
<main class="short-link panel" aria-busy={!missing}>
  {#if missing}<img src="/icons/koalaparty-192.png" alt="" />
    <h1>{$t('shortLink.missing')}</h1>
    <p class="muted">{$t('shortLink.missingHint')}</p>
    <a class="button" href="/">{$t('common.backHome')}</a>
  {:else}<div class="spinner" aria-hidden="true"></div>
    <p>{$t('room.joining')}</p>{/if}
</main>

<style>
  .short-link {
    max-width: 30rem;
    margin: 5rem auto;
    padding: 2.5rem 1.5rem;
    text-align: center;
    display: grid;
    gap: 0.8rem;
    justify-items: center;
  }
  img {
    width: 4rem;
    height: 4rem;
  }
  h1 {
    margin: 0;
    font-size: 1.4rem;
  }
  p {
    margin: 0;
  }
  .spinner {
    width: 2.2rem;
    height: 2.2rem;
    border: 3px solid var(--border-subtle);
    border-top-color: var(--accent-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
