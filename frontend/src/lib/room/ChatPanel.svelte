<script lang="ts">
  import { tick } from 'svelte';
  import { PaperPlaneRight, Plus } from 'phosphor-svelte';
  import { locale, t } from '$lib/i18n';
  import {
    participantNameParts,
    parseYouTubeInput,
    REACTION_EMOJIS,
    splitTimestamps,
    type ChatMessage,
  } from '$lib/room';

  let {
    messages,
    me,
    canChat = true,
    connected = true,
    active = true,
    onSend,
    onReact,
    canAdd = false,
    canSeek = false,
    onAddLink = () => {},
    onSeek = () => {},
    inputEl = $bindable(null),
  }: {
    messages: ChatMessage[];
    me: string;
    canChat?: boolean;
    connected?: boolean;
    active?: boolean;
    onSend: (text: string) => boolean;
    onReact: (emoji: string) => void;
    canAdd?: boolean;
    canSeek?: boolean;
    onAddLink?: (text: string) => void;
    onSeek?: (seconds: number) => void;
    inputEl?: HTMLTextAreaElement | null;
  } = $props();

  let text = $state('');
  let list: HTMLDivElement | undefined = $state();
  let stickToBottom = true;

  // Follow new messages unless the reader scrolled up to read history.
  $effect(() => {
    void messages.length;
    void active;
    if (!list || !stickToBottom) return;
    void tick().then(() => list && (list.scrollTop = list.scrollHeight));
  });

  function onScroll() {
    if (!list) return;
    stickToBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
  }

  function send() {
    const value = text.trim();
    if (!value) return;
    if (onSend(value)) {
      text = '';
      stickToBottom = true;
    }
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      send();
    }
  }

  const time = (at: string) => new Date(at).toLocaleTimeString($locale, { hour: '2-digit', minute: '2-digit' });
  const grouped = (index: number) =>
    index > 0 &&
    messages[index - 1].identityId === messages[index].identityId &&
    Date.parse(messages[index].at) - Date.parse(messages[index - 1].at) < 120_000;
</script>

<div class="chat">
  <div class="messages" bind:this={list} onscroll={onScroll} aria-live="polite" aria-label={$t('chat.history')}>
    {#if !messages.length}<div class="empty">
        <span>💬</span>
        <p>{$t('chat.empty')}</p>
      </div>{/if}
    {#each messages as message, index (message.id)}{@const parts = participantNameParts(message.name)}
      <article class:mine={message.identityId === me} class:grouped={grouped(index)}>
        {#if !grouped(index)}<header>
            <span class="badge" aria-hidden="true">{parts.badge}</span><b>{parts.label}</b><time datetime={message.at}
              >{time(message.at)}</time
            >
          </header>{/if}
        <p>
          {#each splitTimestamps(message.text) as part, index (index)}{#if part.seconds !== undefined && canSeek}<button
                type="button"
                class="timestamp"
                title={$t('chat.jumpTo', { time: part.text })}
                onclick={() => onSeek(part.seconds!)}>{part.text}</button
              >{:else}{part.text}{/if}{/each}
        </p>
        {#if canAdd && parseYouTubeInput(message.text).videos.length}<button
            type="button"
            class="secondary add-link"
            onclick={() => onAddLink(message.text)}><Plus size={13} weight="bold" />{$t('add.addToQueue')}</button
          >{/if}
      </article>{/each}
  </div>
  <div class="quick-reactions" aria-label={$t('player.react')}>
    {#each REACTION_EMOJIS as emoji}<button
        type="button"
        class="ghost"
        aria-label={$t('player.reactWith', { emoji })}
        onclick={() => onReact(emoji)}>{emoji}</button
      >{/each}
  </div>
  <form
    class="composer"
    onsubmit={(event) => {
      event.preventDefault();
      send();
    }}
  >
    <label class="sr-only" for="chat-input">{$t('chat.message')}</label>
    <textarea
      id="chat-input"
      bind:this={inputEl}
      bind:value={text}
      rows="1"
      maxlength="500"
      placeholder={!canChat ? $t('chat.muted') : connected ? $t('chat.placeholder') : $t('room.reconnecting')}
      disabled={!canChat || !connected}
      onkeydown={onKeydown}></textarea>
    <button aria-label={$t('chat.send')} disabled={!canChat || !connected || !text.trim()}
      ><PaperPlaneRight size={18} weight="fill" /></button
    >
  </form>
</div>

<style>
  .chat {
    display: grid;
    grid-template-rows: minmax(0, 1fr) auto auto;
    height: 100%;
    min-height: 0;
  }
  .messages {
    overflow-y: auto;
    padding: 0.6rem 0.9rem;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    overscroll-behavior: contain;
  }
  .empty {
    margin: auto;
    text-align: center;
    color: var(--text-muted);
    font-size: 0.85rem;
    padding: 1.5rem 0.5rem;
  }
  .empty span {
    font-size: 1.8rem;
  }
  article {
    padding: 0.15rem 0;
  }
  article:not(.grouped) {
    margin-top: 0.55rem;
  }
  article header {
    display: flex;
    align-items: baseline;
    gap: 0.4rem;
    font-size: 0.78rem;
  }
  article.mine header b {
    color: var(--accent-primary);
  }
  .badge {
    font-size: 0.9rem;
  }
  time {
    color: var(--text-muted);
    font-size: 0.68rem;
  }
  article p {
    margin: 0.1rem 0 0 1.35rem;
    font-size: 0.88rem;
    line-height: 1.4;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .quick-reactions {
    display: flex;
    justify-content: space-between;
    padding: 0.25rem 0.6rem;
    border-top: 1px solid var(--border-subtle);
  }
  .quick-reactions button {
    font-size: 1rem;
    padding: 0.25rem 0.2rem;
    border-radius: 999px;
    transition: transform 0.15s ease;
  }
  .quick-reactions button:hover {
    transform: scale(1.2);
    background: var(--surface-hover);
  }
  .timestamp {
    display: inline;
    padding: 0 0.2rem;
    border: 0;
    border-radius: 4px;
    background: var(--accent-muted);
    color: var(--accent-primary);
    font: inherit;
    font-weight: 750;
    box-shadow: none;
    cursor: pointer;
  }
  .timestamp:hover {
    transform: none;
    background: var(--surface-hover);
  }
  .add-link {
    margin: 0.3rem 0 0.1rem 1.35rem;
    padding: 0.25rem 0.6rem;
    font-size: 0.72rem;
    border-radius: 999px;
  }
  .composer {
    display: flex;
    gap: 0.4rem;
    padding: 0.5rem 0.6rem 0.7rem;
  }
  textarea {
    flex: 1;
    resize: none;
    font: inherit;
    font-size: 0.9rem;
    color: var(--text-primary);
    background: var(--surface-elevated);
    border: 1px solid var(--border-strong);
    border-radius: 1.2rem;
    padding: 0.6rem 0.9rem;
    max-height: 6rem;
    field-sizing: content;
  }
  textarea:focus {
    outline: none;
    border-color: var(--accent-primary);
  }
  .composer button {
    border-radius: 999px;
    padding: 0.55rem 0.7rem;
    align-self: end;
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
