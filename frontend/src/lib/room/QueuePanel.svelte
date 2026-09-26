<script lang="ts">
  import { flip } from 'svelte/animate';
  import {
    ArrowBendDownRight,
    ArrowCounterClockwise,
    CaretDown,
    CaretUp,
    Copy,
    DotsSixVertical,
    DotsThreeVertical,
    FloppyDisk,
    FolderOpen,
    Play,
    Repeat,
    Shuffle,
    ThumbsUp,
    Trash,
    X,
  } from 'phosphor-svelte';
  import { api } from '$lib/api';
  import { errorText, t } from '$lib/i18n';
  import {
    filterQueue,
    formatDuration,
    participantNameParts,
    type QueueItem,
    type Snapshot,
    type VideoRequest,
  } from '$lib/room';
  import { anchoredMenu, closeMenu } from './actions';
  import Dialog from './Dialog.svelte';

  type Command = (type: string, payload?: Record<string, unknown>) => Promise<boolean>;
  interface SavedQueue {
    id: string;
    name: string;
    items: VideoRequest[];
    itemCount: number;
    createdAt: string;
  }
  let {
    room,
    caps,
    commandPending,
    accountLinked,
    command,
    onRemove,
    onPlayNow,
    onMoveTo,
    onReorder,
    onAdd,
    onNotice,
  }: {
    room: Snapshot;
    caps: Record<string, boolean>;
    commandPending: boolean;
    accountLinked: boolean;
    command: Command;
    onRemove: (item: QueueItem) => void;
    onPlayNow: (item: QueueItem) => void;
    onMoveTo: (itemId: string, index: number) => void;
    onReorder: (itemIds: string[]) => void;
    onAdd: (videos: VideoRequest[]) => Promise<boolean>;
    onNotice: (message: string, kind: 'success' | 'error' | 'info') => void;
  } = $props();

  let query = $state('');
  let dragging = $state<string | null>(null);
  let savedOpen = $state(false);
  let saveOpen = $state(false);
  let saved = $state<SavedQueue[]>([]);
  let savedLoading = $state(false);
  let saveName = $state('');
  const indexOf = (itemId: string) => room.queue.findIndex((item) => item.id === itemId);
  const visible = $derived(filterQueue(room.queue, query));
  const totalStart = $derived(room.queue.length);

  // Everything queued, plus what is playing, as links: paste them into any room.
  const shareableVideos = (): VideoRequest[] => {
    const videos = room.queue.map((item) => ({ videoId: item.media.providerId, start: item.start }));
    if (room.playback.media) videos.unshift({ videoId: room.playback.media.providerId, start: 0 });
    return videos;
  };
  const asLinks = (videos: VideoRequest[]) =>
    videos
      .map((video) => `https://youtu.be/${video.videoId}${video.start ? `?t=${Math.floor(video.start)}` : ''}`)
      .join('\n');

  async function copyLinks(event: Event) {
    closeMenu(event);
    try {
      await navigator.clipboard.writeText(asLinks(shareableVideos()));
      onNotice($t('queue.linksCopied'), 'success');
    } catch {
      onNotice($t('invite.copyFailed'), 'error');
    }
  }
  function drop(target: string) {
    if (!dragging || dragging === target) return;
    const ids = room.queue.map((item) => item.id);
    const from = ids.indexOf(dragging);
    const to = ids.indexOf(target);
    dragging = null;
    if (from < 0 || to < 0) return;
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    onReorder(ids);
  }
  async function openSaved(event: Event) {
    closeMenu(event);
    if (!accountLinked) {
      onNotice($t('queue.accountNeeded'), 'info');
      return;
    }
    savedOpen = true;
    savedLoading = true;
    try {
      saved = await api<SavedQueue[]>('/api/account/queues');
    } catch (e) {
      onNotice(errorText(e), 'error');
    } finally {
      savedLoading = false;
    }
  }
  function openSave(event: Event) {
    closeMenu(event);
    if (!accountLinked) {
      onNotice($t('queue.accountNeeded'), 'info');
      return;
    }
    saveName = room.label;
    saveOpen = true;
  }
  async function saveQueue(event: SubmitEvent) {
    event.preventDefault();
    try {
      await api('/api/account/queues', {
        method: 'POST',
        body: JSON.stringify({ name: saveName, items: shareableVideos() }),
      });
      saveOpen = false;
      onNotice($t('queue.saved'), 'success');
    } catch (e) {
      onNotice(errorText(e), 'error');
    }
  }
  async function loadSaved(queue: SavedQueue) {
    if (await onAdd(queue.items)) savedOpen = false;
  }
  async function deleteSaved(queue: SavedQueue) {
    try {
      await api(`/api/account/queues/${queue.id}`, { method: 'DELETE' });
      saved = saved.filter((item) => item.id !== queue.id);
    } catch (e) {
      onNotice(errorText(e), 'error');
    }
  }
</script>

<header>
  <h2>{$t('queue.title')}</h2>
  <div class="queue-tools">
    <button
      class="ghost"
      title={$t('queue.shuffle')}
      aria-label={$t('queue.shuffle')}
      onclick={() => command('queue.shuffle')}
      disabled={commandPending || room.queue.length < 2 || !caps['queue.reorder']}
      ><Shuffle size={15} weight="bold" /></button
    ><button
      class="ghost"
      class:active={room.queueLoop}
      title={$t('queue.loopHint')}
      aria-label={$t('queue.loop')}
      aria-pressed={room.queueLoop}
      onclick={() => command('queue.loop', { enabled: !room.queueLoop })}
      disabled={!caps['queue.reorder']}><Repeat size={15} weight="bold" /></button
    >
    <details class="queue-more" use:anchoredMenu>
      <summary aria-label={$t('queue.more')} title={$t('queue.more')}
        ><DotsThreeVertical size={16} weight="bold" /></summary
      >
      <div class="menu">
        <button class="ghost" disabled={!totalStart && !room.playback.media} onclick={copyLinks}
          ><Copy size={14} weight="bold" />{$t('queue.copyLinks')}</button
        >
        <button class="ghost" disabled={!totalStart && !room.playback.media} onclick={openSave}
          ><FloppyDisk size={14} weight="bold" />{$t('queue.save')}</button
        >
        <button class="ghost" disabled={!caps['queue.add']} onclick={openSaved}
          ><FolderOpen size={14} weight="bold" />{$t('queue.load')}</button
        >
      </div>
    </details>
  </div>
</header>
{#if room.queue.length > 4}<label class="queue-search">
    <span>{$t('queue.search')}</span>
    <input bind:value={query} type="search" placeholder={$t('queue.searchPlaceholder')} />
  </label>{/if}
{#if !room.queue.length}<div class="panel-empty">
    <span>🎋</span>
    <p>{$t('queue.empty')}<br />{$t('queue.emptyHint')}</p>
  </div>{:else if !visible.length}<div class="panel-empty">
    <span>🔎</span>
    <p>{$t('queue.noMatch', { query })}</p>
  </div>{:else}<ol class="queue">
    {#each visible as item (item.id)}{@const i = indexOf(item.id)}
      <li
        animate:flip={{ duration: 260 }}
        draggable={!query && !commandPending && caps['queue.reorder']}
        ondragstart={() => (dragging = item.id)}
        ondragend={() => (dragging = null)}
        ondragover={(e) => e.preventDefault()}
        ondrop={() => drop(item.id)}
      >
        {#if caps['queue.reorder']}<span class="handle" aria-hidden="true"
            ><DotsSixVertical size={16} weight="bold" /></span
          >{/if}<img src={item.media.thumbnail} alt="" loading="lazy" />
        <div>
          <b title={item.media.title}>{item.media.title}</b><small
            >{i + 1}{item.addedBy ? ` · ${participantNameParts(item.addedBy).label}` : ''}{item.start
              ? ` · ${$t('queue.from', { time: formatDuration(item.start) })}`
              : ''}</small
          >
        </div>
        <button
          class="ghost vote"
          class:active={item.voted}
          aria-label={$t('queue.voteFor', { title: item.media.title })}
          title={$t('queue.voteHint')}
          onclick={() => command('queue.vote', { itemId: item.id })}
          disabled={!caps['queue.vote']}
          ><ThumbsUp size={14} weight={item.voted ? 'fill' : 'bold'} />{item.votes}</button
        >
        <details class="item-menu" use:anchoredMenu>
          <summary aria-label={$t('queue.itemMore', { title: item.media.title })}
            ><DotsThreeVertical size={16} weight="bold" /></summary
          >
          <div class="menu">
            <button
              class="ghost"
              disabled={!caps['media.play_now'] || !caps['queue.remove']}
              onclick={(event) => {
                closeMenu(event);
                onPlayNow(item);
              }}><Play size={14} weight="fill" />{$t('add.playNow')}</button
            ><button
              class="ghost"
              disabled={!caps['queue.reorder'] || i === 0}
              onclick={(event) => {
                closeMenu(event);
                onMoveTo(item.id, 0);
              }}><ArrowBendDownRight size={14} weight="bold" />{$t('add.playNext')}</button
            ><button
              class="ghost"
              aria-label={$t('queue.moveUp', { title: item.media.title })}
              disabled={!caps['queue.reorder'] || i === 0}
              onclick={(event) => {
                closeMenu(event);
                onMoveTo(item.id, i - 1);
              }}><CaretUp size={14} weight="bold" />{$t('queue.up')}</button
            ><button
              class="ghost"
              aria-label={$t('queue.moveDown', { title: item.media.title })}
              disabled={!caps['queue.reorder'] || i === room.queue.length - 1}
              onclick={(event) => {
                closeMenu(event);
                onMoveTo(item.id, i + 1);
              }}><CaretDown size={14} weight="bold" />{$t('queue.down')}</button
            >
          </div>
        </details>
        <button
          class="ghost icon"
          aria-label={$t('queue.remove', { title: item.media.title })}
          onclick={() => onRemove(item)}
          disabled={!caps['queue.remove']}><X size={16} weight="bold" /></button
        >
      </li>{/each}
  </ol>{/if}
{#if room.history.length}<details class="history">
    <summary>{$t('queue.history', { count: room.history.length })}</summary>
    <ul>
      {#each room.history as item, index (`${item.id}-${index}`)}<li>
          <span title={item.title}>{item.title}</span>{#if caps['queue.add']}<button
              class="ghost"
              aria-label={$t('queue.addAgainTitle', { title: item.title })}
              title={$t('queue.addAgain')}
              onclick={() => onAdd([{ videoId: item.providerId, start: 0 }])}
              ><ArrowCounterClockwise size={14} weight="bold" /></button
            >{/if}
        </li>{/each}
    </ul>
  </details>{/if}

{#if saveOpen}<Dialog label={$t('queue.save')} onClose={() => (saveOpen = false)}>
    <h2>{$t('queue.save')}</h2>
    <p class="muted">{$t('queue.saveHint', { count: shareableVideos().length })}</p>
    <form class="save-form" onsubmit={saveQueue}>
      <label>{$t('queue.saveName')}<input bind:value={saveName} maxlength="60" required /></label>
      <div class="modal-actions">
        <button type="button" class="secondary" onclick={() => (saveOpen = false)}>{$t('common.cancel')}</button><button
          >{$t('common.save')}</button
        >
      </div>
    </form>
  </Dialog>{/if}
{#if savedOpen}<Dialog label={$t('queue.load')} wide onClose={() => (savedOpen = false)}>
    <h2>{$t('queue.load')}</h2>
    {#if savedLoading}<p class="muted">{$t('common.loading')}</p>{:else if !saved.length}<p class="muted">
        {$t('queue.noSaved')}
      </p>{:else}<ul class="saved">
        {#each saved as queue (queue.id)}<li>
            <div><b>{queue.name}</b><small>{$t('queue.videoCount', { count: queue.itemCount })}</small></div>
            <button class="secondary small-button" onclick={() => loadSaved(queue)}>{$t('queue.addAll')}</button><button
              class="ghost small-button"
              aria-label={$t('queue.deleteSaved', { name: queue.name })}
              onclick={() => deleteSaved(queue)}><Trash size={14} weight="bold" /></button
            >
          </li>{/each}
      </ul>{/if}
    <div class="modal-actions"><button onclick={() => (savedOpen = false)}>{$t('common.done')}</button></div>
  </Dialog>{/if}

<style>
  .queue-tools {
    display: flex;
    align-items: center;
    gap: 0.2rem;
  }
  .queue-tools .active,
  .vote.active {
    color: var(--accent-primary);
    background: var(--surface-hover);
  }
  .queue-more summary {
    display: inline-flex;
    border-radius: var(--radius-sm);
  }
  .queue-search {
    display: grid;
    gap: 0.35rem;
    padding: 0 1rem 0.8rem;
    color: var(--text-muted);
    font-size: 0.75rem;
  }
  .queue {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .queue li {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.6rem 0.8rem;
    border-top: 1px solid var(--border-subtle);
    transition: background 0.18s ease;
  }
  .queue li:hover {
    background: var(--surface-hover);
  }
  .queue img {
    width: 72px;
    aspect-ratio: 16/9;
    object-fit: cover;
    border-radius: 6px;
    flex: 0 0 auto;
  }
  .queue li > div {
    min-width: 0;
    flex: 1;
  }
  .queue b {
    display: -webkit-box;
    font-size: 0.85rem;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.3;
  }
  .queue small {
    display: block;
    font-size: 0.7rem;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .vote {
    display: inline-flex;
    gap: 0.25rem;
    padding: 0.35rem 0.5rem;
  }
  .handle {
    color: var(--text-muted);
    cursor: grab;
  }
  .icon {
    font-size: 1.3rem;
    padding: 0.3rem;
  }
  .history {
    margin: 0.6rem 0.8rem 1rem;
    color: var(--text-muted);
    font-size: 0.82rem;
  }
  .history ul {
    list-style: none;
    padding: 0;
    margin: 0.4rem 0 0;
  }
  .history li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
    padding: 0.15rem 0;
  }
  .history li span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .history li button {
    padding: 0.3rem;
  }
  .save-form {
    display: grid;
    gap: 1rem;
  }
  .saved {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 0.4rem;
  }
  .saved li {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.55rem 0.7rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
  }
  .saved li div {
    flex: 1;
    display: grid;
    min-width: 0;
  }
  .saved small {
    color: var(--text-muted);
  }
  @media (pointer: coarse) {
    .handle {
      display: none;
    }
  }
  @media (max-width: 580px) {
    .queue li {
      padding: 0.55rem 0.6rem;
      gap: 0.45rem;
    }
    .queue img {
      width: 58px;
    }
    .vote {
      padding: 0.3rem 0.4rem;
    }
  }
</style>
