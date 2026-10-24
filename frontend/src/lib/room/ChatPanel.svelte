<script lang="ts">
  import { tick } from 'svelte';
  import { PaperPlaneRight, Plus, Bell, BellSlash } from 'phosphor-svelte';
  import { locale, t } from '$lib/i18n';
  import { chatSound, toggleChatSound, playNotificationChime } from '$lib/audio';
  import {
    participantNameParts,
    parseYouTubeInput,
    REACTION_EMOJIS,
    splitTimestamps,
    type ChatMessage,
    type Member,
  } from '$lib/room';

  let {
    messages,
    me,
    members = [],
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
    members?: Member[];
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
  let mentionQuery = $state<string | null>(null);
  let mentionIndex = $state(0);

  const myMember = $derived(members.find((m) => m.identityId === me));
  const myName = $derived(myMember?.displayName ? participantNameParts(myMember.displayName).label.toLowerCase() : '');

  function mentionsMe(msgText: string): boolean {
    if (!myName) return false;
    const lower = msgText.toLowerCase();
    return lower.includes('@' + myName) || lower.includes('@all') || lower.includes('@everyone');
  }

  // Play a gentle notification sound when mentioned or on messages while unfocused
  let prevCount = 0;
  $effect(() => {
    const len = messages.length;
    if (len > prevCount && prevCount > 0) {
      const lastMsg = messages[len - 1];
      if (lastMsg && lastMsg.identityId !== me) {
        if ($chatSound && (mentionsMe(lastMsg.text) || !active)) {
          playNotificationChime();
        }
      }
    }
    prevCount = len;
  });

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
      mentionQuery = null;
      stickToBottom = true;
    }
  }

  function handleInput(e: Event) {
    const target = e.currentTarget as HTMLTextAreaElement;
    const val = target.value;
    const cursor = target.selectionStart ?? val.length;
    const beforeCursor = val.slice(0, cursor);
    const atMatch = beforeCursor.match(/(?:^|\s)@([a-zA-Z0-9_\u00C0-\u017F]*)$/);
    if (atMatch) {
      mentionQuery = atMatch[1].toLowerCase();
      mentionIndex = 0;
    } else {
      mentionQuery = null;
    }
  }

  const matchingMembers = $derived(
    mentionQuery !== null
      ? members
          .map((m) => participantNameParts(m.displayName))
          .filter((p) => !mentionQuery || p.label.toLowerCase().includes(mentionQuery))
          .slice(0, 5)
      : [],
  );

  function insertMention(name: string) {
    if (!inputEl) return;
    const cursor = inputEl.selectionStart ?? text.length;
    const before = text.slice(0, cursor);
    const after = text.slice(cursor);
    const atIndex = before.lastIndexOf('@');
    if (atIndex !== -1) {
      text = before.slice(0, atIndex) + `@${name} ` + after;
      mentionQuery = null;
      requestAnimationFrame(() => {
        if (!inputEl) return;
        const newPos = atIndex + name.length + 2;
        inputEl.selectionStart = newPos;
        inputEl.selectionEnd = newPos;
        inputEl.focus();
      });
    }
  }

  function onKeydown(event: KeyboardEvent) {
    if (mentionQuery !== null && matchingMembers.length > 0) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        mentionIndex = (mentionIndex + 1) % matchingMembers.length;
        return;
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault();
        mentionIndex = (mentionIndex - 1 + matchingMembers.length) % matchingMembers.length;
        return;
      }
      if (event.key === 'Tab' || (event.key === 'Enter' && !event.shiftKey)) {
        event.preventDefault();
        const selected = matchingMembers[mentionIndex] ?? matchingMembers[0];
        insertMention(selected.label);
        return;
      }
      if (event.key === 'Escape') {
        mentionQuery = null;
        return;
      }
    }
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      send();
    }
  }

  function splitMentions(raw: string): Array<{ text: string; isMention: boolean }> {
    const parts: Array<{ text: string; isMention: boolean }> = [];
    const regex = /(@[\w\u00C0-\u017F]+)/g;
    let last = 0;
    for (const match of raw.matchAll(regex)) {
      const idx = match.index ?? 0;
      if (idx > last) parts.push({ text: raw.slice(last, idx), isMention: false });
      parts.push({ text: match[0], isMention: true });
      last = idx + match[0].length;
    }
    if (last < raw.length) parts.push({ text: raw.slice(last), isMention: false });
    return parts;
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
      <article
        class:mine={message.identityId === me}
        class:grouped={grouped(index)}
        class:mentioned={message.identityId !== me && mentionsMe(message.text)}
      >
        {#if !grouped(index)}<header>
            <span class="badge" aria-hidden="true">{parts.badge}</span><b>{parts.label}</b><time datetime={message.at}
              >{time(message.at)}</time
            >
          </header>{/if}
        <p>
          {#each splitTimestamps(message.text) as part, tsIndex (tsIndex)}{#if part.seconds !== undefined && canSeek}<button
                type="button"
                class="timestamp"
                title={$t('chat.jumpTo', { time: part.text })}
                onclick={() => onSeek(part.seconds!)}>{part.text}</button
              >{:else}{#each splitMentions(part.text) as sub, subIndex (subIndex)}{#if sub.isMention}<span
                    class="mention"
                    class:highlight={myName && sub.text.toLowerCase() === '@' + myName}>{sub.text}</span
                  >{:else}{sub.text}{/if}{/each}{/if}{/each}
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
    <button
      type="button"
      class="ghost sound-toggle"
      class:muted={!$chatSound}
      title={$chatSound ? $t('chat.soundMute') : $t('chat.soundUnmute')}
      aria-label={$chatSound ? $t('chat.soundMute') : $t('chat.soundUnmute')}
      onclick={toggleChatSound}
    >
      {#if $chatSound}
        <Bell size={15} weight="bold" />
      {:else}
        <BellSlash size={15} weight="bold" />
      {/if}
    </button>
  </div>

  <form
    class="composer"
    onsubmit={(event) => {
      event.preventDefault();
      send();
    }}
  >
    {#if matchingMembers.length > 0}
      <div class="mention-autocomplete" role="listbox">
        {#each matchingMembers as member, idx (member.label)}
          <button
            type="button"
            class="mention-item"
            class:selected={idx === mentionIndex}
            onmousedown={(e) => {
              e.preventDefault();
              insertMention(member.label);
            }}
          >
            <span class="badge">{member.badge}</span>
            <span class="name">{member.label}</span>
          </button>
        {/each}
      </div>
    {/if}
    <label class="sr-only" for="chat-input">{$t('chat.message')}</label>
    <textarea
      id="chat-input"
      bind:this={inputEl}
      bind:value={text}
      rows="1"
      maxlength="500"
      placeholder={!canChat ? $t('chat.muted') : connected ? $t('chat.placeholder') : $t('room.reconnecting')}
      disabled={!canChat || !connected}
      oninput={handleInput}
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
    padding: 0.15rem 0.35rem;
    border-radius: var(--radius-sm);
    transition: background 0.2s ease;
  }
  article:not(.grouped) {
    margin-top: 0.55rem;
  }
  article.mentioned {
    background: color-mix(in srgb, var(--accent-primary) 12%, var(--surface-panel));
    border-left: 3px solid var(--accent-primary);
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
    display: inline-grid;
    place-content: center;
    width: 1.25rem;
    height: 1.25rem;
    font-size: 0.72rem;
    border-radius: 50%;
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
  }
  time {
    color: var(--text-muted);
    font-size: 0.68rem;
    margin-left: auto;
  }
  p {
    margin: 0.15rem 0 0.1rem 1.35rem;
    font-size: 0.88rem;
    line-height: 1.35;
    word-break: break-word;
    white-space: pre-wrap;
  }
  .mention {
    display: inline-block;
    padding: 0 0.25rem;
    border-radius: 4px;
    font-weight: 700;
    color: var(--accent-primary);
    background: var(--accent-muted);
  }
  .mention.highlight {
    background: var(--accent-primary);
    color: var(--surface-panel);
  }
  .quick-reactions {
    display: flex;
    justify-content: space-around;
    align-items: center;
    padding: 0.25rem 0.5rem;
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
  .sound-toggle {
    margin-left: auto;
    padding: 0.3rem 0.45rem !important;
    color: var(--text-muted);
    border-radius: var(--radius-sm) !important;
  }
  .sound-toggle:hover {
    color: var(--text-primary);
    transform: none !important;
  }
  .sound-toggle.muted {
    opacity: 0.5;
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
    position: relative;
    display: flex;
    gap: 0.4rem;
    padding: 0.5rem 0.6rem 0.7rem;
  }
  .mention-autocomplete {
    position: absolute;
    bottom: calc(100% - 0.2rem);
    left: 0.6rem;
    right: 0.6rem;
    background: var(--surface-panel);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    box-shadow: 0 8px 24px rgba(3, 12, 8, 0.25);
    padding: 0.3rem;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    z-index: 30;
  }
  .mention-item {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.35rem 0.55rem;
    border-radius: var(--radius-sm);
    background: transparent;
    border: 0;
    color: var(--text-primary);
    font-size: 0.82rem;
    font-weight: 650;
    cursor: pointer;
    text-align: left;
    width: 100%;
  }
  .mention-item:hover,
  .mention-item.selected {
    background: var(--accent-muted);
    color: var(--accent-primary);
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
