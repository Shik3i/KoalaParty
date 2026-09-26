<script lang="ts">
  import { errorText, t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { parseYouTubeInput } from '$lib/room';
  import { recentRooms as loadRecentRooms, type RecentRoom } from '$lib/recentRooms';
  import { ArrowRight, Plus, ShareNetwork } from 'phosphor-svelte';

  // Target of the installed app's share sheet ("Share → KoalaParty") and of the
  // bookmarklet. Android puts the link in `text` or `url` depending on the app.
  const params = page.url.searchParams;
  const shared = [params.get('url'), params.get('text'), params.get('title')]
    .filter((value): value is string => !!value)
    .join(' ');
  const parsed = parseYouTubeInput(shared);
  const link = parsed.videos.length
    ? parsed.videos
        .map((video) => `https://youtu.be/${video.videoId}${video.start ? `?t=${video.start}` : ''}`)
        .join('\n')
    : parsed.playlistId
      ? `https://www.youtube.com/playlist?list=${parsed.playlistId}`
      : '';
  let rooms: RecentRoom[] = [];
  let creating = false;
  let error = '';

  onMount(() => {
    rooms = loadRecentRooms();
  });

  function open(roomId: string) {
    void goto(`/room/${roomId}?add=${encodeURIComponent(link)}`);
  }

  async function createRoom() {
    creating = true;
    error = '';
    try {
      const room = await api<{ id: string }>('/api/rooms', { method: 'POST' });
      open(room.id);
    } catch (e) {
      error = errorText(e);
      creating = false;
    }
  }
</script>

<svelte:head><title>{$t('share.title')}</title></svelte:head>
<main class="share panel">
  <span class="icon" aria-hidden="true"><ShareNetwork size={28} weight="bold" /></span>
  {#if !link}<h1>{$t('share.nothing')}</h1>
    <p class="muted">{$t('share.nothingBody')}</p>
    <a class="button" href="/">{$t('share.goHome')}</a>
  {:else}
    <h1>{$t('share.heading')}</h1>
    <p class="muted">
      {parsed.videos.length > 1
        ? $t('share.many', { count: parsed.videos.length })
        : parsed.videos.length
          ? $t('share.one')
          : $t('share.playlist')}
    </p>
    {#if rooms.length}<ul class="rooms">
        {#each rooms as room, index (room.id)}<li>
            <button class:secondary={index > 0} onclick={() => open(room.id)}>
              <span><b>{room.label}</b><small>{room.title || $t('home.readyToWatch')}</small></span><ArrowRight
                size={18}
                weight="bold"
              />
            </button>
          </li>{/each}
      </ul>{/if}
    <button class:secondary={rooms.length > 0} class="new-room" onclick={createRoom} disabled={creating}
      ><Plus size={18} weight="bold" />{creating ? $t('home.creating') : $t('share.newRoom')}</button
    >
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  {/if}
</main>

<style>
  .share {
    max-width: 30rem;
    margin: 3rem auto;
    padding: 2rem 1.5rem;
    display: grid;
    gap: 0.9rem;
    text-align: center;
  }
  .icon {
    justify-self: center;
    display: grid;
    place-content: center;
    width: 3.5rem;
    height: 3.5rem;
    border-radius: 50%;
    background: var(--accent-muted);
    color: var(--accent-primary);
  }
  h1 {
    margin: 0;
    font-size: 1.4rem;
  }
  .muted {
    margin: 0;
    color: var(--text-muted);
  }
  .rooms {
    list-style: none;
    padding: 0;
    margin: 0.4rem 0 0;
    display: grid;
    gap: 0.5rem;
  }
  .rooms button,
  .new-room {
    width: 100%;
    justify-content: space-between;
    text-align: left;
  }
  .new-room {
    justify-content: center;
  }
  .rooms span {
    display: grid;
    min-width: 0;
  }
  .rooms b,
  .rooms small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rooms small {
    font-weight: 500;
    opacity: 0.75;
  }
</style>
