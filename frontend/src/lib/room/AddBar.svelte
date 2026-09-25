<script lang="ts">
  import { fly } from 'svelte/transition';
  import { ClipboardText, MagnifyingGlass, Play, Plus, X, ArrowBendDownRight, Spinner } from 'phosphor-svelte';
  import { api } from '$lib/api';
  import { formatDuration, looksLikeLink, parseYouTubeInput, type AddMode, type VideoRequest } from '$lib/room';

  interface SearchResult {
    videoId: string;
    title: string;
    channel: string;
    thumbnail: string;
    duration: number;
  }

  let {
    canAdd = true,
    canPlayNow = true,
    searchEnabled = false,
    variant = 'panel',
    onAdd,
    onPlaylist,
    onPlaylistOffer = () => {},
    onError,
    inputEl = $bindable(null),
  }: {
    canAdd?: boolean;
    canPlayNow?: boolean;
    searchEnabled?: boolean;
    variant?: 'panel' | 'hero';
    onAdd: (videos: VideoRequest[], mode: AddMode) => Promise<boolean>;
    onPlaylist: (playlistId: string) => Promise<void>;
    onPlaylistOffer?: (playlistId: string) => void;
    onError: (message: string) => void;
    inputEl?: HTMLInputElement | null;
  } = $props();

  let value = $state('');
  let busy = $state(false);
  let results = $state<SearchResult[] | null>(null);
  let searching = $state(false);
  let searchedFor = $state('');
  const inputId = `add-video-${Math.random().toString(36).slice(2, 8)}`;
  const parsed = $derived(parseYouTubeInput(value));
  const isLink = $derived(looksLikeLink(value));
  const hint = $derived(
    !value.trim()
      ? ''
      : parsed.videos.length > 1
        ? `${parsed.videos.length} videos found`
        : parsed.playlistId && !parsed.videos.length
          ? 'Playlist link'
          : parsed.videos.length === 1
            ? parsed.videos[0].start
              ? `Starts at ${formatDuration(parsed.videos[0].start)}`
              : ''
            : isLink
              ? 'That link does not point to a YouTube video'
              : searchEnabled
                ? 'Press Enter to search YouTube'
                : 'Paste a YouTube link',
  );

  async function submit(mode: AddMode) {
    if (busy || !value.trim()) return;
    if (parsed.videos.length) {
      busy = true;
      try {
        if (await onAdd(parsed.videos, mode)) {
          const playlist = parsed.playlistId;
          value = '';
          results = null;
          if (playlist && searchEnabled) onPlaylistOffer(playlist);
        }
      } finally {
        busy = false;
      }
      return;
    }
    if (parsed.playlistId) {
      busy = true;
      try {
        await onPlaylist(parsed.playlistId);
        value = '';
      } finally {
        busy = false;
      }
      return;
    }
    if (isLink) {
      onError('That link does not point to a YouTube video.');
      return;
    }
    if (!searchEnabled) {
      onError('Paste a YouTube link — search is not enabled on this server.');
      return;
    }
    await search();
  }

  async function search() {
    const query = value.trim();
    if (!query || searching) return;
    searching = true;
    searchedFor = query;
    try {
      results = await api<SearchResult[]>(`/api/youtube/search?q=${encodeURIComponent(query)}`);
    } catch (e) {
      results = null;
      onError(e instanceof Error ? e.message : 'Search failed.');
    } finally {
      searching = false;
    }
  }

  async function addResult(result: SearchResult, mode: AddMode) {
    if (await onAdd([{ videoId: result.videoId, start: 0 }], mode)) {
      results = results?.filter((item) => item.videoId !== result.videoId) ?? null;
    }
  }

  async function paste() {
    try {
      const text = (await navigator.clipboard.readText()).trim();
      if (!text) return;
      value = text;
      if (parseYouTubeInput(text).videos.length || parseYouTubeInput(text).playlistId) await submit('queue');
      else inputEl?.focus();
    } catch {
      inputEl?.focus();
      onError('Clipboard access was blocked — press Ctrl+V (⌘V) in the box instead.');
    }
  }

  // Pasting a link into the box adds it right away; no extra click needed.
  function onPaste(event: ClipboardEvent) {
    const text = event.clipboardData?.getData('text') ?? '';
    const pasted = parseYouTubeInput(text);
    if (!pasted.videos.length && !pasted.playlistId) return;
    event.preventDefault();
    value = text.trim();
    void submit('queue');
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      void submit(event.shiftKey ? 'now' : event.altKey ? 'next' : 'queue');
    } else if (event.key === 'Escape') {
      if (results) results = null;
      else if (value) value = '';
      else inputEl?.blur();
    }
  }
</script>

<div class="add-bar {variant}">
  <div class="field">
    <label class="sr-only" for={inputId}>YouTube URL or search</label>
    <span class="lead" aria-hidden="true"
      >{#if searching || busy}<Spinner
          size={18}
          class="spin"
        />{:else if value && !isLink && searchEnabled}<MagnifyingGlass size={18} weight="bold" />{:else}<Plus
          size={18}
          weight="bold"
        />{/if}</span
    >
    <input
      id={inputId}
      bind:this={inputEl}
      bind:value
      maxlength="4096"
      autocomplete="off"
      enterkeyhint="go"
      disabled={!canAdd}
      placeholder={canAdd
        ? searchEnabled
          ? 'Paste a YouTube link or search…'
          : 'Paste a YouTube link…'
        : 'You cannot add videos in this room'}
      onpaste={onPaste}
      onkeydown={onKeydown}
    />
    {#if value}<button
        type="button"
        class="ghost clear"
        aria-label="Clear"
        onclick={() => ((value = ''), (results = null))}><X size={16} weight="bold" /></button
      >{:else}<button
        type="button"
        class="ghost clear"
        aria-label="Paste from clipboard"
        title="Paste from clipboard"
        disabled={!canAdd}
        onclick={paste}><ClipboardText size={18} weight="bold" /></button
      >{/if}
  </div>
  {#if value.trim() && (parsed.videos.length || !searchEnabled || isLink)}<div
      class="actions"
      transition:fly={{ y: -4, duration: 140 }}
    >
      <button
        type="button"
        onclick={() => submit('queue')}
        disabled={busy || !canAdd || (!parsed.videos.length && !parsed.playlistId)}
        ><Plus size={16} weight="bold" />Add to queue</button
      >
      {#if parsed.videos.length}<button
          type="button"
          class="secondary"
          title="Insert at the top of the queue (Alt+Enter)"
          onclick={() => submit('next')}
          disabled={busy || !canAdd}><ArrowBendDownRight size={16} weight="bold" />Play next</button
        ><button
          type="button"
          class="secondary"
          title="Start for everyone now (Shift+Enter)"
          onclick={() => submit('now')}
          disabled={busy || !canPlayNow}><Play size={16} weight="fill" />Play now</button
        >{/if}
    </div>{/if}
  {#if hint}<p class="hint" aria-live="polite">{hint}</p>{/if}
  {#if results}<div class="results" transition:fly={{ y: -6, duration: 160 }}>
      <header>
        <span>Results for “{searchedFor}”</span><button
          type="button"
          class="ghost"
          aria-label="Close search results"
          onclick={() => (results = null)}><X size={14} weight="bold" /></button
        >
      </header>
      {#if !results.length}<p class="muted">No embeddable videos found.</p>{/if}
      <ul>
        {#each results as result (result.videoId)}<li>
            <img src={result.thumbnail} alt="" loading="lazy" />
            <div>
              <b title={result.title}>{result.title}</b>
              <small>{result.channel}{result.duration ? ` · ${formatDuration(result.duration)}` : ''}</small>
            </div>
            <span class="result-actions">
              <button
                type="button"
                class="ghost"
                aria-label={`Add ${result.title} to the queue`}
                title="Add to queue"
                onclick={() => addResult(result, 'queue')}><Plus size={16} weight="bold" /></button
              ><button
                type="button"
                class="ghost"
                aria-label={`Play ${result.title} now`}
                title="Play now"
                disabled={!canPlayNow}
                onclick={() => addResult(result, 'now')}><Play size={16} weight="fill" /></button
              >
            </span>
          </li>{/each}
      </ul>
    </div>{/if}
</div>

<style>
  .add-bar {
    display: grid;
    gap: 0.5rem;
  }
  .field {
    position: relative;
    display: flex;
    align-items: center;
  }
  .lead {
    position: absolute;
    left: 0.85rem;
    display: grid;
    color: var(--accent-primary);
    pointer-events: none;
  }
  .lead :global(.spin) {
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  input {
    padding: 0.8rem 2.8rem 0.8rem 2.6rem;
    border-radius: 999px;
    font-size: 0.95rem;
    border-color: color-mix(in srgb, var(--accent-primary) 45%, var(--border-strong));
    background: var(--surface-elevated);
  }
  input:focus {
    border-color: var(--accent-primary);
    box-shadow: 0 0 0 4px color-mix(in srgb, var(--accent-primary) 18%, transparent);
  }
  .hero input {
    font-size: 1.05rem;
    padding-block: 1rem;
  }
  .clear {
    position: absolute;
    right: 0.3rem;
    padding: 0.45rem;
    border-radius: 999px;
  }
  .actions {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .actions button {
    flex: 1 1 auto;
    padding: 0.55rem 0.75rem;
    font-size: 0.85rem;
  }
  .hint {
    margin: 0 0.9rem;
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  .hero .hint {
    color: #cfdcd4;
  }
  .results {
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--surface-elevated);
    overflow: hidden;
    box-shadow: var(--shadow-panel);
  }
  .results header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.35rem 0.4rem 0.35rem 0.8rem;
    font-size: 0.75rem;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border-subtle);
  }
  .results header span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .results ul {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 340px;
    overflow: auto;
  }
  .results li {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0.6rem;
    border-top: 1px solid var(--border-subtle);
  }
  .results li:first-child {
    border-top: 0;
  }
  .results img {
    width: 76px;
    aspect-ratio: 16/9;
    object-fit: cover;
    border-radius: 6px;
    flex: 0 0 auto;
  }
  .results li > div {
    flex: 1;
    min-width: 0;
    display: grid;
  }
  .results b,
  .results small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.82rem;
  }
  .results small {
    color: var(--text-muted);
    font-size: 0.72rem;
  }
  .result-actions {
    display: flex;
  }
  .result-actions button {
    padding: 0.4rem;
  }
  .muted {
    color: var(--text-muted);
    padding: 0.8rem;
    margin: 0;
    font-size: 0.85rem;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
</style>
